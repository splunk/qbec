// Copyright 2025 Splunk Inc.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package commands

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"sort"

	"sigs.k8s.io/yaml"
	"github.com/spf13/cobra"
	"github.com/splunk/qbec/internal/cmd"
	"github.com/splunk/qbec/internal/diff"
	"github.com/splunk/qbec/internal/eval"
	"github.com/splunk/qbec/internal/model"
	"github.com/splunk/qbec/internal/sio"
)

var maxDisplayValueLength = 1024

func newParamCommand(cp ctxProvider) *cobra.Command {
	cmd := &cobra.Command{
		Use:     "param <subcommand>",
		Short:   "parameter lists and diffs",
		Aliases: []string{"params"},
	}
	cmd.AddCommand(newParamListCommand(cp), newParamDiffCommand(cp))
	return cmd
}

func listParams(components map[string]interface{}, formatSpecified bool, format string, w io.Writer) error {
	var p []param
	for c, v := range components {
		val, ok := v.(map[string]interface{})
		if !ok {
			sio.Warnln("invalid parameter format for", c, ",expected object")
			continue
		}
		for n, v := range val {
			p = append(p, param{Component: c, Name: n, Value: v})
		}
	}
	sort.Slice(p, func(i, j int) bool {
		if p[i].Component != p[j].Component {
			return p[i].Component < p[j].Component
		}
		return p[i].Name < p[j].Name
	})
	if !formatSpecified {
		fmt.Fprintf(w, "%-30s %-30s %s\n", "COMPONENT", "NAME", "VALUE")
		for _, param := range p {
			valBytes, _ := json.Marshal(param.Value)
			valStr := string(valBytes)
			if len(valStr) > maxDisplayValueLength {
				valStr = valStr[:maxDisplayValueLength-3] + "..."
			}
			fmt.Fprintf(w, "%-30s %-30s %s\n", param.Component, param.Name, valStr)
		}
		return nil
	}
	switch format {
	case "yaml":
		b, err := yaml.Marshal(p)
		if err != nil {
			return err
		}
		fmt.Fprintln(w, "---")
		fmt.Fprintf(w, "%s\n", b)
		return nil
	case "json":
		encoder := json.NewEncoder(w)
		encoder.SetIndent("", "  ")
		return encoder.Encode(p)
	default:
		return cmd.NewUsageError(fmt.Sprintf("listParams: unsupported format %q", format))
	}
}

type param struct {
	Component string      `json:"component"`
	Name      string      `json:"name"`
	Value     interface{} `json:"value"`
}

func extractComponentParams(paramsObject map[string]interface{}, fp model.Filters) (map[string]interface{}, error) {
	cf, err := model.NewComponentFilter(fp.ComponentIncludes(), fp.ComponentExcludes())
	if err != nil {
		return nil, err
	}
	baseComponents, ok := paramsObject["components"].(map[string]interface{})
	if !ok {
		return nil, fmt.Errorf("unable to find 'components' key in the parameter object")
	}
	var components map[string]interface{}
	if !cf.HasFilters() {
		components = baseComponents
	} else {
		components = map[string]interface{}{}
		for k, v := range baseComponents {
			if cf.ShouldInclude(k) {
				components[k] = v
			}
		}
	}
	return components, nil
}

type paramListCommandConfig struct {
	cmd.AppContext
	format     string
	filterFunc func() (model.Filters, error)
}

func doParamList(args []string, config paramListCommandConfig) error {
	if len(args) != 1 {
		return cmd.NewUsageError(fmt.Sprintf("exactly one environment required, but provided: %q", args))
	}
	env := args[0]
	if env != model.Baseline {
		_, err := config.App().ServerURL(env)
		if err != nil {
			return err
		}
	}
	paramsFile := config.App().ParamsFile()
	envCtx, err := config.EnvContext(env)
	if err != nil {
		return err
	}
	paramsObject, err := eval.Params(paramsFile, envCtx.EvalContext(cleanEvalMode))
	if err != nil {
		return err
	}
	fp, err := config.filterFunc()
	if err != nil {
		return err
	}
	components, err := extractComponentParams(paramsObject, fp)
	if err != nil {
		return err
	}
	return listParams(components, config.format != "", config.format, config.Stdout())
}

func newParamListCommand(cp ctxProvider) *cobra.Command {
	c := &cobra.Command{
		Use:     "list [-c component]...  <environment>|_",
		Short:   "list all parameters for an environment, optionally for a subset of components",
		Example: paramListExamples(),
	}
	config := paramListCommandConfig{
		filterFunc: addFilterParams(c, false),
	}
	c.Flags().StringVarP(&config.format, "format", "o", "", "use json|yaml to display machine readable input")
	c.RunE = func(c *cobra.Command, args []string) error {
		config.AppContext = cp()
		return cmd.WrapError(doParamList(args, config))
	}
	return c
}

type paramDiffCommandConfig struct {
	cmd.AppContext
	filterFunc func() (model.Filters, error)
}

func doParamDiff(args []string, config paramDiffCommandConfig) error {
	var leftEnv, rightEnv string
	switch len(args) {
	case 1:
		leftEnv = model.Baseline
		rightEnv = args[0]
	case 2:
		leftEnv = args[0]
		rightEnv = args[1]
	default:
		return cmd.NewUsageError("one or two environments required")
	}

	fp, err := config.filterFunc()
	if err != nil {
		return err
	}
	getParams := func(env string) (str string, name string, err error) {
		if env != model.Baseline {
			_, err := config.App().ServerURL(env)
			if err != nil {
				return "", "", err
			}
		}
		paramsFile := config.App().ParamsFile()
		envCtx, err := config.AppContext.EnvContext(env)
		if err != nil {
			return "", "", err
		}
		paramsObject, err := eval.Params(paramsFile, envCtx.EvalContext(cleanEvalMode))
		if err != nil {
			return "", "", err
		}
		components, err := extractComponentParams(paramsObject, fp)
		if err != nil {
			return "", "", err
		}
		var buf bytes.Buffer
		if err := listParams(components, false, "", &buf); err != nil {
			return "", "", err
		}
		name = "environment: " + env
		if env == model.Baseline {
			name = "baseline"
		}
		return buf.String(), name, nil
	}

	var left, right, leftName, rightName string

	left, leftName, err = getParams(leftEnv)
	if err != nil {
		return err
	}
	right, rightName, err = getParams(rightEnv)
	if err != nil {
		return err
	}

	opts := diff.Options{Context: -1, LeftName: leftName, RightName: rightName, Colorize: config.Colorize()}
	d, err := diff.Strings(left, right, opts)
	if err != nil {
		return err
	}
	fmt.Fprintln(config.Stdout(), string(d))
	return nil

}

func newParamDiffCommand(cp ctxProvider) *cobra.Command {
	c := &cobra.Command{
		Use:     "diff [-c component]... <environment>|_ [<environment>|_]",
		Short:   "diff parameter lists across two environments or between the baseline (use _ for baseline) and an environment",
		Example: paramDiffExamples(),
	}

	config := paramDiffCommandConfig{
		filterFunc: addFilterParams(c, false),
	}

	c.RunE = func(c *cobra.Command, args []string) error {
		config.AppContext = cp()
		return cmd.WrapError(doParamDiff(args, config))
	}
	return c
}
