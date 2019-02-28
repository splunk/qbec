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
	"strings"

	"github.com/pkg/errors"
	"github.com/splunk/qbec/internal/model"
	"github.com/splunk/qbec/internal/sio"
	"github.com/splunk/qbec/internal/vm"
)

// Context is the evaluation context
type Context struct {
	App     string // the application for which the evaluation is done
	Env     string // the environment for which the evaluation is done
	VM      *vm.VM // the base VM to use for eval
	Verbose bool   // show generated code
}

// Components evaluates the specified components using the specific runtime
// parameters file and returns the result.
func Components(components []model.Component, ctx Context) ([]model.K8sLocalObject, error) {
	if ctx.VM == nil {
		ctx.VM = vm.New(vm.Config{})
	}
	cCode, err := evalComponents(components, ctx)
	if err != nil {
		return nil, errors.Wrap(err, "evaluate components")
	}
	objs, err := k8sObjectsFromJSONString(cCode, ctx.App, ctx.Env)
	if err != nil {
		return nil, errors.Wrap(err, "extract objects")
	}
	return objs, nil
}

// Params evaluates the supplied parameters file in the supplied VM and
// returns it as a JSON object.
func Params(file string, ctx Context) (map[string]interface{}, error) {
	baseVM := ctx.VM
	if baseVM == nil {
		baseVM = vm.New(vm.Config{})
	}
	cfg := baseVM.Config().WithVars(map[string]string{model.QbecNames.EnvVarName: ctx.Env})
	jvm := vm.New(cfg)
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

func evalComponents(list []model.Component, ctx Context) (string, error) {
	cfg := ctx.VM.Config().WithVars(map[string]string{model.QbecNames.EnvVarName: ctx.Env})
	jvm := vm.New(cfg)
	var lines []string
	for _, c := range list {
		switch {
		case strings.HasSuffix(c.File, ".yaml"):
			lines = append(lines, fmt.Sprintf("'%s': parseYaml(importstr '%s')", c.Name, c.File))
		case strings.HasSuffix(c.File, ".json"):
			lines = append(lines, fmt.Sprintf("'%s': parseJson(importstr '%s')", c.Name, c.File))
		default:
			lines = append(lines, fmt.Sprintf("'%s': import '%s'", c.Name, c.File))
		}
	}
	preamble := []string{
		"local parseYaml = std.native('parseYaml');",
		"local parseJson = std.native('parseJson');",
	}
	code := strings.Join(preamble, "\n") + "\n{\n  " + strings.Join(lines, ",\n  ") + "\n}"
	if ctx.Verbose {
		sio.Debugln("Eval components:\n" + code)
	}
	ret, err := jvm.EvaluateSnippet("component-loader.jsonnet", code)
	if err != nil {
		return "", err
	}
	if ctx.Verbose {
		sio.Debugln("Eval components output:\n" + prettyJSON(ret))
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
