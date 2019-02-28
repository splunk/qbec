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

	"github.com/ghodss/yaml"
	"github.com/spf13/cobra"
	"github.com/splunk/qbec/internal/diff"
	"github.com/splunk/qbec/internal/model"
)

func newComponentCommand(op OptionsProvider) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "component <subcommand>",
		Short: "component lists and diffs",
	}
	cmd.AddCommand(newComponentListCommand(op), newComponentDiffCommand(op))
	return cmd
}

func listComponents(components []model.Component, formatSpecified bool, format string, w io.Writer) error {
	if !formatSpecified {
		fmt.Fprintf(w, "%-30s %s\n", "COMPONENT", "FILE")
		for _, c := range components {
			fmt.Fprintf(w, "%-30s %s\n", c.Name, c.File)
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
		return newUsageError(fmt.Sprintf("listComponents: unsupported format %q", format))
	}
}

type componentListCommandConfig struct {
	StdOptions
	format  string
	objects bool
}

func doComponentList(args []string, config componentListCommandConfig) error {
	if len(args) != 1 {
		return newUsageError("exactly one environment required")
	}
	env := args[0]
	if config.objects {
		objects, err := filteredObjects(config, env, filterParams{})
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

func newComponentListCommand(op OptionsProvider) *cobra.Command {
	cmd := &cobra.Command{
		Use:     "list [-objects] <environment>",
		Short:   "list all components for an environment, optionally listing all objects as well",
		Example: componentListExamples(),
	}

	config := componentListCommandConfig{}
	cmd.Flags().BoolVarP(&config.objects, "objects", "O", false, "set to true to also list objects in each component")
	cmd.Flags().StringVarP(&config.format, "format", "o", "", "use json|yaml to display machine readable input")

	cmd.RunE = func(c *cobra.Command, args []string) error {
		config.StdOptions = op()
		return wrapError(doComponentList(args, config))
	}
	return cmd
}

type componentDiffCommandConfig struct {
	StdOptions
	objects bool
}

func doComponentDiff(args []string, config componentDiffCommandConfig) error {
	var leftEnv, rightEnv string
	switch len(args) {
	case 1:
		leftEnv = model.Baseline
		rightEnv = args[0]
	case 2:
		leftEnv = args[0]
		rightEnv = args[1]
	default:
		return newUsageError("one or two environments required")
	}

	getComponents := func(env string) (str string, name string, err error) {
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

	getObjects := func(env string) (str string, name string, err error) {
		objs, err := filteredObjects(config, env, filterParams{})
		if err != nil {
			return
		}
		var buf bytes.Buffer
		if err = showNames(objs, false, "", &buf); err != nil {
			return
		}
		str = buf.String()
		name = "environment: " + env
		if env == model.Baseline {
			name = "baseline"
		}
		return
	}
	var left, right, leftName, rightName string
	var err error

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

func newComponentDiffCommand(op OptionsProvider) *cobra.Command {
	cmd := &cobra.Command{
		Use:     "diff [-objects] <environment>|_ [<environment>|_]",
		Short:   "diff component lists across two environments or between the baseline (use _ for baseline) and an environment",
		Example: componentDiffExamples(),
	}

	config := componentDiffCommandConfig{}
	cmd.Flags().BoolVarP(&config.objects, "objects", "O", false, "set to true to also list objects in each component")

	cmd.RunE = func(c *cobra.Command, args []string) error {
		config.StdOptions = op()
		return wrapError(doComponentDiff(args, config))
	}
	return cmd
}
