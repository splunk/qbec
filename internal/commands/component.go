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

package commands

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"strings"

	"github.com/ghodss/yaml"
	"github.com/spf13/cobra"
	"github.com/splunk/qbec/internal/cmd"
	"github.com/splunk/qbec/internal/diff"
	"github.com/splunk/qbec/internal/model"
)

func newComponentCommand(cp ctxProvider) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "component <subcommand>",
		Short: "component lists and diffs",
	}
	cmd.AddCommand(newComponentListCommand(cp), newComponentDiffCommand(cp))
	return cmd
}

func listComponents(components []model.Component, formatSpecified bool, format string, w io.Writer) error {
	if !formatSpecified {
		fmt.Fprintf(w, "%-30s %s\n", "COMPONENT", "FILES")
		for _, c := range components {
			fmt.Fprintf(w, "%-30s %s\n", c.Name, strings.Join(c.Files, ", "))
		}
		return nil
	}
	switch format {
	case "yaml":
		b, err := yaml.Marshal(components)
		if err != nil {
			return err
		}
		fmt.Fprintln(w, "---")
		fmt.Fprintf(w, "%s\n", b)
		return nil
	case "json":
		encoder := json.NewEncoder(w)
		encoder.SetIndent("", "  ")
		return encoder.Encode(components)
	default:
		return cmd.NewUsageError(fmt.Sprintf("listComponents: unsupported format %q", format))
	}
}

type componentListCommandConfig struct {
	cmd.AppContext
	format  string
	objects bool
}

func doComponentList(args []string, config componentListCommandConfig) error {
	if len(args) != 1 {
		return cmd.NewUsageError("exactly one environment required")
	}
	env := args[0]
	if config.objects {
		envCtx, err := config.EnvContext(env)
		if err != nil {
			return err
		}
		objects, err := filteredObjects(envCtx, nil, filterParams{})
		if err != nil {
			return err
		}
		return showNames(objects, config.format != "", config.format, config.Stdout())
	}
	components, err := config.App().ComponentsForEnvironment(env, nil, nil)
	if err != nil {
		return err
	}
	return listComponents(components, config.format != "", config.format, config.Stdout())
}

func newComponentListCommand(cp ctxProvider) *cobra.Command {
	c := &cobra.Command{
		Use:     "list [-objects] <environment>",
		Short:   "list all components for an environment, optionally listing all objects as well",
		Example: componentListExamples(),
	}

	config := componentListCommandConfig{}
	c.Flags().BoolVarP(&config.objects, "objects", "O", false, "set to true to also list objects in each component")
	c.Flags().StringVarP(&config.format, "format", "o", "", "use json|yaml to display machine readable input")

	c.RunE = func(c *cobra.Command, args []string) error {
		config.AppContext = cp()
		return cmd.WrapError(doComponentList(args, config))
	}
	return c
}

type componentDiffCommandConfig struct {
	cmd.AppContext
	objects bool
}

func doComponentDiff(args []string, config componentDiffCommandConfig) error {
	var leftEnv, rightEnv cmd.EnvContext
	var err error
	switch len(args) {
	case 1:
		leftEnv, err = config.EnvContext("_")
		if err != nil {
			return err
		}
		rightEnv, err = config.EnvContext(args[0])
		if err != nil {
			return err
		}
	case 2:
		leftEnv, err = config.EnvContext(args[0])
		if err != nil {
			return err
		}
		rightEnv, err = config.EnvContext(args[1])
		if err != nil {
			return err
		}
	default:
		return cmd.NewUsageError("one or two environments required")
	}

	getComponents := func(envCtx cmd.EnvContext) (str string, name string, err error) {
		env := envCtx.Env()
		comps, err := config.App().ComponentsForEnvironment(env, nil, nil)
		if err != nil {
			return
		}
		var buf bytes.Buffer
		if err = listComponents(comps, false, "", &buf); err != nil {
			return
		}
		str = buf.String()
		name = "environment: " + env
		if env == model.Baseline {
			name = "baseline"
		}
		return
	}

	getObjects := func(envCtx cmd.EnvContext) (str string, name string, err error) {
		objs, err := filteredObjects(envCtx, nil, filterParams{})
		if err != nil {
			return
		}
		var buf bytes.Buffer
		if err = showNames(objs, false, "", &buf); err != nil {
			return
		}
		str = buf.String()
		name = "environment: " + envCtx.Env()
		if envCtx.Env() == model.Baseline {
			name = "baseline"
		}
		return
	}
	var left, right, leftName, rightName string

	if config.objects {
		left, leftName, err = getObjects(leftEnv)
		if err != nil {
			return err
		}
		right, rightName, err = getObjects(rightEnv)
		if err != nil {
			return err
		}
	} else {
		left, leftName, err = getComponents(leftEnv)
		if err != nil {
			return err
		}
		right, rightName, err = getComponents(rightEnv)
		if err != nil {
			return err
		}
	}

	opts := diff.Options{Context: -1, LeftName: leftName, RightName: rightName, Colorize: config.Colorize()}
	d, err := diff.Strings(left, right, opts)
	if err != nil {
		return err
	}
	fmt.Fprintln(config.Stdout(), string(d))
	return nil
}

func newComponentDiffCommand(cp ctxProvider) *cobra.Command {
	c := &cobra.Command{
		Use:     "diff [-objects] <environment>|_ [<environment>|_]",
		Short:   "diff component lists across two environments or between the baseline (use _ for baseline) and an environment",
		Example: componentDiffExamples(),
	}

	config := componentDiffCommandConfig{}
	c.Flags().BoolVarP(&config.objects, "objects", "O", false, "set to true to also list objects in each component")

	c.RunE = func(c *cobra.Command, args []string) error {
		config.AppContext = cp()
		return cmd.WrapError(doComponentDiff(args, config))
	}
	return c
}
