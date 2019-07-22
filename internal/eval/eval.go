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
	"io/ioutil"
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

type postProc struct {
	ctx  Context
	code string
	file string
}

func (p postProc) run(obj map[string]interface{}) (map[string]interface{}, error) {
	if p.code == "" {
		return obj, nil
	}
	b, err := json.Marshal(obj)
	if err != nil {
		return nil, errors.Wrap(err, "json marshal")
	}
	cfg := p.ctx.baseVMConfig(nil).WithTopLevelCodeVars(map[string]string{
		postprocessTLAVAr: string(b),
	})

	jvm := vm.New(cfg)
	evalCode, err := jvm.EvaluateSnippet(p.file, p.code)
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
	App             string       // the application for which the evaluation is done
	Tag             string       // the gc tag if present
	Env             string       // the environment for which the evaluation is done
	DefaultNs       string       // the default namespace to expose as an external variable
	VMConfig        VMConfigFunc // the base VM config to use for eval
	Verbose         bool         // show generated code
	Concurrency     int          // concurrent components to evaluate, default 5
	PostProcessFile string       // the file that contains post-processing code for all objects
}

func (c Context) baseVMConfig(tlas []string) vm.Config {
	fn := c.VMConfig
	if fn == nil {
		fn = defaultFunc
	}
	cfg := fn(tlas).WithVars(map[string]string{
		model.QbecNames.EnvVarName:       c.Env,
		model.QbecNames.TagVarName:       c.Tag,
		model.QbecNames.DefaultNsVarName: c.DefaultNs,
	})
	return cfg
}

func (c Context) vm(tlas []string) *vm.VM {
	return vm.New(c.baseVMConfig(tlas))
}

func (c Context) postProcessor() (postProc, error) {
	if c.PostProcessFile == "" {
		return postProc{}, nil
	}
	b, err := ioutil.ReadFile(c.PostProcessFile)
	if err != nil {
		return postProc{}, errors.Wrap(err, "read post-eval file")
	}
	return postProc{
		ctx:  c,
		code: string(b),
		file: c.PostProcessFile,
	}, nil
}

var defaultFunc = func(_ []string) vm.Config { return vm.Config{} }

// Components evaluates the specified components using the specific runtime
// parameters file and returns the result.
func Components(components []model.Component, ctx Context) (_ []model.K8sLocalObject, finalErr error) {
	start := time.Now()
	defer func() {
		if finalErr == nil {
			sio.Debugf("%d components evaluated in %v\n", len(components), time.Since(start).Round(time.Millisecond))
		}
	}()
	pe, err := ctx.postProcessor()
	if err != nil {
		return nil, err
	}
	ret, err := evalComponents(components, ctx, pe)
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
	jvm := ctx.vm(nil)
	code := fmt.Sprintf("import '%s'", file)
	if ctx.Verbose {
		sio.Debugln("Eval params:\n" + code)
	}
	output, err := jvm.EvaluateSnippet("param-loader.jsonnet", code)
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

func evalComponent(ctx Context, c model.Component, pe postProc) ([]model.K8sLocalObject, error) {
	jvm := ctx.vm(c.TopLevelVars)
	var inputCode string
	contextFile := c.File
	switch {
	case strings.HasSuffix(c.File, ".yaml"):
		inputCode = fmt.Sprintf("std.native('parseYaml')(importstr '%s')", c.File)
		contextFile = "yaml-loader.jsonnet"
	case strings.HasSuffix(c.File, ".json"):
		inputCode = fmt.Sprintf("std.native('parseJson')(importstr '%s')", c.File)
		contextFile = "json-loader.jsonnet"
	default:
		b, err := ioutil.ReadFile(c.File)
		if err != nil {
			return nil, errors.Wrap(err, "read inputCode for "+c.File)
		}
		inputCode = string(b)
	}
	evalCode, err := jvm.EvaluateSnippet(contextFile, inputCode)
	if err != nil {
		return nil, errors.Wrap(err, fmt.Sprintf("evaluate '%s'", c.Name))
	}
	var data interface{}
	if err := json.Unmarshal([]byte(evalCode), &data); err != nil {
		return nil, errors.Wrap(err, fmt.Sprintf("unexpected unmarshal '%s'", c.File))
	}

	objs, err := walk(data)
	if err != nil {
		return nil, errors.Wrap(err, "extract objects")
	}

	var processed []model.K8sLocalObject
	for _, o := range objs {
		proc, err := pe.run(o)
		if err != nil {
			return nil, err
		}
		processed = append(processed, model.NewK8sLocalObject(proc, ctx.App, ctx.Tag, c.Name, ctx.Env))
	}
	return processed, nil
}

func evalComponents(list []model.Component, ctx Context, pe postProc) ([]model.K8sLocalObject, error) {
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
				objs, err := evalComponent(ctx, c, pe)
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
