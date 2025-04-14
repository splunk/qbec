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
	"encoding/json"
	"fmt"

	"github.com/spf13/cobra"
	"github.com/splunk/qbec/internal/cmd"
	"github.com/splunk/qbec/internal/eval"
	"github.com/splunk/qbec/vm/vmutil"
)

type evalCommandConfig struct {
	cmd.AppContext
	format string
	env    string
}

func doEval(args []string, config evalCommandConfig) error {
	if len(args) != 1 {
		return cmd.NewUsageError("exactly one file required")
	}
	var output string
	var err error
	var envCtx cmd.EnvContext
	var basicCtx eval.BaseContext
	if config.env == "" {
		basicCtx, err = config.BasicEvalContext()
		if err != nil {
			return err
		}
		output, err = eval.File(args[0], basicCtx)
	} else {
		envCtx, err = config.EnvContext(config.env)
		if err != nil {
			return err
		}
		ctx := envCtx.EvalContext(cleanEvalMode)
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
		err = vmutil.RenderYAMLDocuments([]interface{}{data}, config.Stdout())
		if err != nil {
			return err
		}
	default:
		b, err = json.MarshalIndent(data, "", "  ")
		if err != nil {
			return err
		}
		_, err = fmt.Fprintf(config.Stdout(), "%s\n", b)
		if err != nil {
			return err
		}
	}
	return nil
}

func newEvalCommand(cp ctxProvider) *cobra.Command {
	c := &cobra.Command{
		Use:     "eval [--env <env>] path/to/file.jsonnet",
		Short:   "evaluate the supplied file optionally under a qbec environment",
		Example: evalExamples(),
	}
	cfg := evalCommandConfig{}
	c.Flags().StringVarP(&cfg.format, "format", "o", "json", "Output format. Supported values are: json, yaml")
	c.Flags().StringVar(&cfg.env, "env", "", "qbec environment context, optional")
	c.RunE = func(c *cobra.Command, args []string) error {
		cfg.AppContext = cp()
		return cmd.WrapError(doEval(args, cfg))
	}
	return c
}
