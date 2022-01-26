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
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	"github.com/ghodss/yaml"
	"github.com/spf13/cobra"
	"github.com/splunk/qbec/internal/cmd"
)

func newEnvCommand(cp ctxProvider) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "env <subcommand>",
		Short: "environment lists and details",
	}
	cmd.AddCommand(newEnvListCommand(cp), newEnvVarsCommand(cp), newEnvPropsCommand(cp))
	return cmd
}

type envListCommandConfig struct {
	cmd.AppContext
	format string
}

func newEnvListCommand(cp ctxProvider) *cobra.Command {
	c := &cobra.Command{
		Use:     "list [-o <format>]",
		Short:   "list all environments in short, json or yaml format",
		Example: envListExamples(),
	}

	config := envListCommandConfig{}
	c.Flags().StringVarP(&config.format, "format", "o", "", "use json|yaml to display machine readable output")

	c.RunE = func(c *cobra.Command, args []string) error {
		config.AppContext = cp()
		return cmd.WrapError(doEnvList(args, config))
	}
	return c
}

type displayEnv struct {
	Name             string `json:"name"`
	Server           string `json:"server"`
	DefaultNamespace string `json:"defaultNamespace"`
}

type displayEnvList struct {
	Environments []displayEnv `json:"environments"`
}

func listEnvironments(config envListCommandConfig) error {
	app := config.App()
	var list []displayEnv
	for name, obj := range app.Environments() {
		defNs := obj.DefaultNamespace
		if defNs == "" {
			defNs = "default"
		}
		list = append(list, displayEnv{
			Name:             name,
			Server:           obj.Server,
			DefaultNamespace: defNs,
		})
	}
	sort.Slice(list, func(i, j int) bool {
		return list[i].Name < list[j].Name
	})

	wrapper := displayEnvList{list}
	w := config.Stdout()

	switch config.format {
	case "json":
		enc := json.NewEncoder(w)
		enc.SetIndent("", "  ")
		_ = enc.Encode(wrapper)
	case "yaml":
		b, _ := yaml.Marshal(wrapper)
		_, _ = w.Write(b)
	case "":
		for _, e := range list {
			fmt.Fprintln(w, e.Name)
		}
	default:
		return cmd.NewUsageError(fmt.Sprintf("listEnvironments: unsupported format %q", config.format))
	}
	return nil
}

func doEnvList(args []string, config envListCommandConfig) error {
	if len(args) != 0 {
		return cmd.NewUsageError("extra arguments specified")
	}
	return listEnvironments(config)
}

func newEnvVarsCommand(cp ctxProvider) *cobra.Command {
	c := &cobra.Command{
		Use:     "vars [-o <format>] <env>",
		Short:   "print variables for kubeconfig, context and cluster for an environment",
		Example: envVarsExamples(),
	}

	config := envVarsCommandConfig{}
	c.Flags().StringVarP(&config.format, "format", "o", "", "use json|yaml to display machine readable output")

	c.RunE = func(c *cobra.Command, args []string) error {
		config.AppContext = cp()
		return cmd.WrapError(doEnvVars(args, config))
	}
	return c
}

type envVarsCommandConfig struct {
	cmd.AppContext
	format string
}

func doEnvVars(args []string, config envVarsCommandConfig) error {
	if len(args) != 1 {
		return cmd.NewUsageError(fmt.Sprintf("exactly one environment required: %v", args))
	}
	if _, ok := config.App().Environments()[args[0]]; !ok {
		return fmt.Errorf("invalid environment: %q", args[0])
	}
	return environmentVars(args[0], config)
}

func environmentVars(name string, config envVarsCommandConfig) error {
	envCtx, err := config.EnvContext(name)
	if err != nil {
		return err
	}
	attrs, err := envCtx.KubeAttributes()
	if err != nil {
		return err
	}
	w := config.Stdout()
	switch config.format {
	case "json":
		enc := json.NewEncoder(w)
		enc.SetIndent("", "  ")
		_ = enc.Encode(attrs)
	case "yaml":
		b, _ := yaml.Marshal(attrs)
		_, _ = w.Write(b)
	case "":
		var kcArgs []string
		addArg := func(name, value string) {
			if value != "" {
				kcArgs = append(kcArgs, fmt.Sprintf(`--%s=%s`, name, value))
			}
		}
		addArg("context", attrs.Context)
		addArg("cluster", attrs.Cluster)
		addArg("namespace", attrs.Namespace)

		var lines []string
		var vars []string
		addLine := func(name, value string) {
			lines = append(lines, fmt.Sprintf(`%s='%s'`, name, value))
			vars = append(vars, name)
		}
		addLine("KUBECONFIG", attrs.ConfigFile)
		addLine("KUBE_CLUSTER", attrs.Cluster)
		addLine("KUBE_CONTEXT", attrs.Context)
		addLine("KUBE_NAMESPACE", attrs.Namespace)
		addLine("KUBECTL_ARGS", strings.Join(kcArgs, " "))

		for _, l := range lines {
			fmt.Fprintf(w, "%s;\n", l)
		}
		fmt.Fprintf(w, "export %s\n", strings.Join(vars, " "))
	default:
		return cmd.NewUsageError(fmt.Sprintf("environmentVars: unsupported format %q", config.format))
	}
	return nil
}

func newEnvPropsCommand(cp ctxProvider) *cobra.Command {
	c := &cobra.Command{
		Use:   "props [-o <format>] <env>",
		Short: "print properties for an environment",
	}

	config := envPropsCommandConfig{}
	c.Flags().StringVarP(&config.format, "format", "o", "", "use json|yaml to display machine readable output")

	c.RunE = func(c *cobra.Command, args []string) error {
		config.AppContext = cp()
		return cmd.WrapError(doEnvProps(args, config))
	}
	return c
}

type envPropsCommandConfig struct {
	cmd.AppContext
	format string
}

func doEnvProps(args []string, config envPropsCommandConfig) error {
	if len(args) != 1 {
		return cmd.NewUsageError(fmt.Sprintf("exactly one environment required, but provided: %q", args))
	}
	if _, ok := config.App().Environments()[args[0]]; !ok {
		return fmt.Errorf("invalid environment: %q", args[0])
	}
	return environmentProps(args[0], config)
}

func environmentProps(name string, config envPropsCommandConfig) error {
	props, err := config.App().Properties(name)
	if err != nil {
		return err
	}
	w := config.Stdout()
	switch config.format {
	case "json":
		enc := json.NewEncoder(w)
		enc.SetIndent("", "  ")
		_ = enc.Encode(props)
	case "", "yaml":
		b, _ := yaml.Marshal(props)
		_, _ = w.Write(b)
	default:
		return cmd.NewUsageError(fmt.Sprintf("environmentVars: unsupported format %q", config.format))
	}
	return nil
}
