/*
   Copyright 2019 Splunk Inc.

   Licensed under the Apache License, Version 2.0 (the "License");
   you may not use this file except in compliance with the License.
   You may obtain a copy of the License at

       http://www.apache.org/licenses/LICENSE-2.0

   Unless required by applicable law or agreed to in writing, software
   distributed under the License is distributed on an "AS IS" BASIS,
   WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
   See the License for the specific language governing permissions and
   limitations under the License.
*/

// Package eval encapsulates the manner in which components and parameters are evaluated for qbec.
package eval

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/pkg/errors"
	"github.com/splunk/qbec/internal/model"
	"github.com/splunk/qbec/internal/sio"
	"github.com/splunk/qbec/internal/vm"
	"github.com/splunk/qbec/internal/vm/natives"
)

const (
	defaultConcurrency = 5
	maxDisplayErrors   = 3
	postprocessTLAVar  = "object"
)

// LocalObjectProducer converts a data object that has basic Kubernetes attributes
// to a local model object.
type LocalObjectProducer func(component string, data map[string]interface{}) model.K8sLocalObject

func baseName(file string) string {
	base := filepath.Base(file)
	pos := strings.LastIndex(base, ".")
	if pos > 0 {
		base = base[:pos]
	}
	return base
}

type postProc struct {
	ctx  Context
	file string
}

func (p postProc) run(obj map[string]interface{}) (map[string]interface{}, error) {
	b, err := json.Marshal(obj)
	if err != nil {
		return nil, errors.Wrap(err, "json marshal")
	}
	procName := model.QBECPostprocessorNamespace + baseName(p.file)
	baseVars := p.ctx.Vars.WithTopLevelVars(vm.NewCodeVar(postprocessTLAVar, string(b)))
	evalCode, err := p.ctx.evalFile(p.file, p.ctx.componentVars(baseVars, procName, nil))
	if err != nil {
		return nil, errors.Wrap(err, "post-eval object")
	}

	var data interface{}
	if err := json.Unmarshal([]byte(evalCode), &data); err != nil {
		return nil, errors.Wrap(err, fmt.Sprintf("unexpected unmarshal '%s'", p.file))
	}
	t, ok := data.(map[string]interface{})
	if !ok {
		return nil, fmt.Errorf("post-eval did not return an object, %s", evalCode)
	}
	if getRawObjectType(t) != leafType {
		return nil, fmt.Errorf("post-eval did not return a K8s object, %s", evalCode)
	}
	return t, nil
}

// Context is the evaluation context
type Context struct {
	LibPaths         []string
	Vars             vm.VariableSet
	Verbose          bool              // show generated code
	Concurrency      int               // concurrent components to evaluate, default 5
	PreProcessFiles  []string          // preprocessor files that are evaluated if present
	PostProcessFiles []string          // files that contains post-processing code for all objects
	tlaVars          map[string]vm.Var // all top level string vars specified for the command
}

func (c *Context) init() {
	tlas := c.Vars.TopLevelVars()
	c.Vars = c.Vars.WithoutTopLevel()
	c.tlaVars = map[string]vm.Var{}
	for _, v := range tlas {
		c.tlaVars[v.Name] = v
	}
}

func (c Context) componentVars(base vm.VariableSet, componentName string, tlas []string) vm.VariableSet {
	vs := base
	if componentName != "" {
		vs = base.WithVars(vm.NewVar(model.QbecNames.ComponentName, componentName))
	}
	if len(tlas) == 0 {
		return vs
	}
	check := map[string]bool{}
	for _, v := range tlas {
		check[v] = true
	}
	var add []vm.Var
	for k, v := range c.tlaVars {
		if check[k] {
			add = append(add, v)
		}
	}
	return vs.WithTopLevelVars(add...)
}

func (c *Context) evalFile(file string, vars vm.VariableSet) (jsonData string, err error) {
	jvm := vm.New(vm.Config{
		LibPaths:  c.LibPaths,
		Variables: c.Vars,
	})
	return jvm.EvalFile(file, vars)
}

func (c *Context) runPreprocessors() error {
	for _, file := range c.PreProcessFiles {
		procName := baseName(file)
		evalCode, err := c.evalFile(file, c.componentVars(c.Vars, model.QBECPreprocessorNamespace+procName, nil))
		if err != nil {
			return errors.Wrapf(err, "preprocessor eval %s", file)
		}
		name := model.QBECComputedNamespace + procName
		sio.Debugln("setting external variable", name)
		c.Vars = c.Vars.WithVars(vm.NewCodeVar(name, evalCode))
	}
	return nil
}

func (c Context) postProcessors() []postProc {
	var ret []postProc
	for _, file := range c.PostProcessFiles {
		ret = append(ret, postProc{
			ctx:  c,
			file: file,
		})
	}
	return ret
}

// Components evaluates the specified components using the specific runtime
// parameters file and returns the result.
func Components(components []model.Component, ctx Context, lop LocalObjectProducer) (_ []model.K8sLocalObject, finalErr error) {
	start := time.Now()
	defer func() {
		if finalErr == nil {
			sio.Debugf("%d components evaluated in %v\n", len(components), time.Since(start).Round(time.Millisecond))
		}
	}()
	ctx.init()
	err := ctx.runPreprocessors()
	if err != nil {
		return nil, err
	}
	pe := ctx.postProcessors()
	ret, err := evalComponents(components, ctx, pe, lop)
	if err != nil {
		return nil, err
	}

	sort.Slice(ret, func(i, j int) bool {
		left := ret[i]
		right := ret[j]
		leftKey := fmt.Sprintf("%s:%s:%s:%s", left.Component(), left.GetNamespace(), left.GroupVersionKind().Kind, left.GetName())
		rightKey := fmt.Sprintf("%s:%s:%s:%s", right.Component(), right.GetNamespace(), right.GroupVersionKind().Kind, right.GetName())
		return leftKey < rightKey
	})
	return ret, nil
}

// Params evaluates the supplied parameters file in the supplied VM and
// returns it as a JSON object.
func Params(file string, ctx Context) (map[string]interface{}, error) {
	ctx.init()
	output, err := ctx.evalFile(file, ctx.componentVars(ctx.Vars, "", nil))
	if err != nil {
		return nil, err
	}
	if ctx.Verbose {
		sio.Debugln("Eval params output:\n" + prettyJSON(output))
	}
	var ret map[string]interface{}
	if err := json.Unmarshal([]byte(output), &ret); err != nil {
		return nil, err
	}
	return ret, nil
}

type evalFn func(file string, component string, tlas []string) (interface{}, error)

func openFile(file string) (*os.File, error) {
	f, err := os.Open(file)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, fmt.Errorf("%s: file not found", file)
		}
		return nil, err
	}
	return f, nil
}

func evaluationCode(c Context, file string) evalFn {
	switch {
	case strings.HasSuffix(file, ".yaml"):
		return func(file string, component string, tlas []string) (interface{}, error) {
			f, err := openFile(file)
			if err != nil {
				return nil, err
			}
			defer f.Close()
			return natives.ParseYAMLDocuments(f)
		}
	case strings.HasSuffix(file, ".json"):
		return func(file string, component string, tlas []string) (interface{}, error) {
			f, err := openFile(file)
			if err != nil {
				return nil, err
			}
			defer f.Close()
			return natives.ParseJSON(f)
		}
	default:
		return func(file string, component string, tlas []string) (interface{}, error) {
			evalCode, err := c.evalFile(file, c.componentVars(c.Vars, component, tlas))
			if err != nil {
				return nil, err
			}
			var data interface{}
			if err := json.Unmarshal([]byte(evalCode), &data); err != nil {
				return nil, errors.Wrap(err, fmt.Sprintf("unexpected unmarshal '%s'", file))
			}
			return data, nil
		}
	}
}

func evalComponent(ctx Context, c model.Component, pe []postProc, lop LocalObjectProducer) ([]model.K8sLocalObject, error) {
	var data []interface{}
	for _, file := range c.Files {
		fn := evaluationCode(ctx, file)
		ret, err := fn(file, c.Name, c.TopLevelVars)
		if err != nil {
			return nil, errors.Wrapf(err, "evaluate '%s'", c.Name)
		}
		data = append(data, ret)
	}
	var evalData interface{}
	if len(data) == 1 {
		evalData = data[0]
	} else {
		evalData = data
	}
	objs, err := walk(evalData)
	if err != nil {
		return nil, errors.Wrap(err, "extract objects")
	}

	runPostProcessors := func(obj map[string]interface{}) (map[string]interface{}, error) {
		var err error
		for _, pp := range pe {
			obj, err = pp.run(obj)
			if err != nil {
				return nil, errors.Wrapf(err, "run post-processor %s", pp.file)
			}
		}
		return obj, nil
	}

	var processed []model.K8sLocalObject
	for _, o := range objs {
		proc, err := runPostProcessors(o)
		if err != nil {
			return nil, err
		}
		if err := model.AssertMetadataValid(proc); err != nil {
			return nil, err
		}
		processed = append(processed, lop(c.Name, proc))
	}
	return processed, nil
}

func evalComponents(list []model.Component, ctx Context, pe []postProc, lop LocalObjectProducer) ([]model.K8sLocalObject, error) {
	var ret []model.K8sLocalObject
	if len(list) == 0 {
		return ret, nil
	}

	ch := make(chan model.Component, len(list))
	for _, c := range list {
		ch <- c
	}
	close(ch)

	var errs []error
	var l sync.Mutex

	concurrency := ctx.Concurrency
	if concurrency <= 0 {
		concurrency = defaultConcurrency
	}
	if concurrency > len(list) {
		concurrency = len(list)
	}
	var wg sync.WaitGroup
	wg.Add(concurrency)

	for i := 0; i < concurrency; i++ {
		go func() {
			defer wg.Done()
			for c := range ch {
				objs, err := evalComponent(ctx, c, pe, lop)
				l.Lock()
				if err != nil {
					errs = append(errs, err)
				} else {
					ret = append(ret, objs...)
				}
				l.Unlock()
			}
		}()
	}
	wg.Wait()
	if len(errs) > 0 {
		var msgs []string
		for i, e := range errs {
			if i == maxDisplayErrors {
				msgs = append(msgs, fmt.Sprintf("... and %d more errors", len(errs)-maxDisplayErrors))
				break
			}
			msgs = append(msgs, e.Error())
		}
		return nil, errors.New(strings.Join(msgs, "\n"))
	}
	return ret, nil
}

func prettyJSON(s string) string {
	var data interface{}
	if err := json.Unmarshal([]byte(s), &data); err == nil {
		b, err := json.MarshalIndent(data, "", "  ")
		if err == nil {
			return string(b)
		}
	}
	return s
}
