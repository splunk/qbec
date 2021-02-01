/*
   Copyright 2021 Splunk Inc.

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

package commands

import (
	"encoding/json"
	"fmt"

	"github.com/ghodss/yaml"
	"github.com/spf13/cobra"
	"github.com/splunk/qbec/internal/eval"
	"github.com/splunk/qbec/internal/vm"
)

type evalCommandConfig struct {
	*config
	format string
	env    string
}

func doEval(args []string, config evalCommandConfig) error {
	if len(args) != 1 {
		return newUsageError("exactly one file required")
	}
	var output string
	var err error
	if config.env == "" {
		output, err = eval.File(args[0], eval.BaseContext{
			LibPaths: config.ext.LibPaths,
			Vars:     vm.VariablesFromConfig(config.ext),
			Verbose:  config.verbose > 1,
		})
	} else {
		props, err := config.App().Properties(config.env)
		if err != nil {
			return err
		}
		ctx, err := config.EvalContext(config.env, props)
		if err != nil {
			return err
		}
		output, err = eval.File(args[0], ctx.BaseContext)
	}
	if err != nil {
		return err
	}
	var data interface{}
	err = json.Unmarshal([]byte(output), &data)
	if err != nil {
		return err
	}
	var b []byte
	switch config.format {
	case "yaml":
		b, err = yaml.Marshal(data)
	default:
		b, err = json.MarshalIndent(data, "", "  ")
	}
	if err != nil {
		return err
	}
	_, _ = fmt.Fprintf(config.Stdout(), "%s\n", b)
	return nil
}

func newEvalCommand(cp configProvider) *cobra.Command {
	cmd := &cobra.Command{
		Use:     "eval [--env <env>] path/to/file.jsonnet",
		Short:   "evaluate the supplied file optionally under a qbec environment",
		Example: evalExamples(),
	}
	cfg := evalCommandConfig{}
	cmd.Flags().StringVarP(&cfg.format, "format", "o", "json", "Output format. Supported values are: json, yaml")
	cmd.Flags().StringVar(&cfg.env, "env", "", "qbec environment context, optional")
	cmd.RunE = func(c *cobra.Command, args []string) error {
		cfg.config = cp()
		return wrapError(doEval(args, cfg))
	}
	return cmd
}
