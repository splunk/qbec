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
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
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
)

const (
	defaultConcurrency = 5
	maxDisplayErrors   = 3
	postprocessTLAVAr  = "object"
)

// VMConfigFunc is a function that returns a VM configuration containing only the
// specified top-level variables of interest.
type VMConfigFunc func(tlaVars []string) vm.Config

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
	cfg := p.ctx.baseVMConfig(procName, nil).WithTopLevelCodeVars(map[string]string{
		postprocessTLAVAr: string(b),
	})

	jvm := vm.New(cfg)
	evalCode, err := p.ctx.evalFile(jvm, p.file)
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
	VMConfig         VMConfigFunc // the base VM config to use for eval
	Verbose          bool         // show generated code
	Concurrency      int          // concurrent components to evaluate, default 5
	PreProcessFiles  []string     // preprocessor files that are evaluated if present
	PostProcessFiles []string     // files that contains post-processing code for all objects
}

func (c Context) baseVMConfig(componentName string, tlas []string) vm.Config {
	base := c.VMConfig(tlas)
	if componentName == "" {
		return base
	}
	return base.WithVars(map[string]string{
		model.QbecNames.ComponentName: componentName,
	})
}

func (c Context) vm(componentName string, tlas []string) *vm.VM {
	return vm.New(c.baseVMConfig(componentName, tlas))
}

func (c *Context) initDefaults() {
	if c.VMConfig == nil {
		c.VMConfig = defaultFunc
	}
}

func (c *Context) evalFile(jvm *vm.VM, file string) (json string, err error) {
	s, err := os.Stat(file)
	if err != nil {
		return "", err
	}
	if s.IsDir() {
		return "", fmt.Errorf("file '%s' was a directory", file)
	}
	file = filepath.ToSlash(file)
	return jvm.EvaluateFile(file)
}

func (c *Context) runPreprocessors() error {
	for _, file := range c.PreProcessFiles {
		procName := baseName(file)
		evalCode, err := c.evalFile(c.vm(model.QBECPreprocessorNamespace+procName, nil), file)
		if err != nil {
			return errors.Wrapf(err, "preprocessor eval %s", file)
		}
		fn := c.VMConfig
		name := model.QBECComputedNamespace + procName
		sio.Debugln("setting external variable", name)
		c.VMConfig = func(tlas []string) vm.Config {
			ret := fn(tlas)
			return ret.WithCodeVars(map[string]string{
				name: evalCode,
			})
		}
	}
	return nil
}

func (c Context) postProcessors() ([]postProc, error) {
	var ret []postProc
	for _, file := range c.PostProcessFiles {
		ret = append(ret, postProc{
			ctx:  c,
			file: file,
		})
	}
	return ret, nil
}

var defaultFunc = func(_ []string) vm.Config { return vm.Config{} }

// Components evaluates the specified components using the specific runtime
// parameters file and returns the result.
func Components(components []model.Component, ctx Context, lop LocalObjectProducer) (_ []model.K8sLocalObject, finalErr error) {
	start := time.Now()
	defer func() {
		if finalErr == nil {
			sio.Debugf("%d components evaluated in %v\n", len(components), time.Since(start).Round(time.Millisecond))
		}
	}()
	ctx.initDefaults()
	err := ctx.runPreprocessors()
	if err != nil {
		return nil, err
	}
	pe, err := ctx.postProcessors()
	if err != nil {
		return nil, err
	}
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
	ctx.initDefaults()
	output, err := ctx.evalFile(ctx.vm("", nil), file)
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

func evaluationCode(c Context, file string) evalFn {
	switch {
	case strings.HasSuffix(file, ".yaml"):
		return func(file string, component string, tlas []string) (interface{}, error) {
			b, err := ioutil.ReadFile(file)
			if err != nil {
				return nil, err
			}
			return vm.ParseYAMLDocuments(bytes.NewReader(b))
		}
	case strings.HasSuffix(file, ".json"):
		return func(file string, component string, tlas []string) (interface{}, error) {
			b, err := ioutil.ReadFile(file)
			if err != nil {
				return nil, err
			}
			var data interface{}
			if err := json.Unmarshal(b, &data); err != nil {
				return nil, err
			}
			return data, nil
		}
	default:
		return func(file string, component string, tlas []string) (interface{}, error) {
			evalCode, err := c.evalFile(c.vm(component, tlas), file)
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
