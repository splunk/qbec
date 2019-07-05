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
	"io"

	"github.com/ghodss/yaml"
	"github.com/spf13/cobra"
	"github.com/splunk/qbec/internal/model"
	"github.com/splunk/qbec/internal/objsort"
	"github.com/splunk/qbec/internal/sio"
)

type metaOnly struct {
	model.K8sLocalObject
}

func (n *metaOnly) MarshalJSON() ([]byte, error) {
	gvk := n.GroupVersionKind()
	m := map[string]string{
		"apiVersion":  gvk.GroupVersion().String(),
		"component":   n.Component(),
		"environment": n.Environment(),
		"kind":        gvk.Kind,
		"name":        model.NameForDisplay(n),
	}
	if n.GetNamespace() != "" {
		m["namespace"] = n.GetNamespace()
	}
	return json.Marshal(m)
}

func showNames(objects []model.K8sLocalObject, formatSpecified bool, format string, w io.Writer) error {
	if !formatSpecified { // render as table
		fmt.Fprintf(w, "%-30s %-30s %-40s %s\n", "COMPONENT", "KIND", "NAME", "NAMESPACE")
		for _, o := range objects {
			name := model.NameForDisplay(o)
			fmt.Fprintf(w, "%-30s %-30s %-40s %s\n", o.Component(), o.GroupVersionKind().Kind, name, o.GetNamespace())
		}
		return nil
	}

	out := make([]model.K8sLocalObject, 0, len(objects))
	for _, o := range objects {
		out = append(out, &metaOnly{o})
	}
	objects = out

	switch format {
	case "yaml":
		b, err := yaml.Marshal(objects)
		if err != nil {
			return err
		}
		fmt.Fprintln(w, "---")
		fmt.Fprintf(w, "%s\n", b)
		return nil
	case "json":
		encoder := json.NewEncoder(w)
		encoder.SetIndent("", "  ")
		return encoder.Encode(objects)
	default:
		return fmt.Errorf("showNames: unsupport format %q", format)
	}
}

type showCommandConfig struct {
	*Config
	showSecrets     bool
	format          string
	formatSpecified bool
	sortAsApply     bool
	namesOnly       bool
	filterFunc      func() (filterParams, error)
}

func doShow(args []string, config showCommandConfig) error {
	if len(args) != 1 {
		return newUsageError("exactly one environment required")
	}
	env := args[0]
	format := config.format
	if format != "json" && format != "yaml" {
		return newUsageError(fmt.Sprintf("invalid output format: %q", format))
	}
	fp, err := config.filterFunc()
	if err != nil {
		return err
	}

	// shallow duplicate check
	keyFunc := func(obj model.K8sMeta) string {
		gvk := obj.GroupVersionKind()
		ns := obj.GetNamespace()
		return fmt.Sprintf("%s:%s:%s:%s", gvk.Group, gvk.Kind, ns, obj.GetName())
	}

	objects, err := filteredObjects(config.Config, env, keyFunc, fp)
	if err != nil {
		return err
	}

	if !config.showSecrets {
		for i, o := range objects {
			objects[i], _ = model.HideSensitiveLocalInfo(o)
		}
	}

	if config.sortAsApply {
		if env == model.Baseline {
			sio.Warnln("cannot sort in apply order for baseline environment")
		} else {
			client, err := config.Client(env)
			if err != nil {
				return err
			}
			objects = objsort.Sort(objects, sortConfig(client.IsNamespaced))
		}
	}

	if config.namesOnly {
		return showNames(objects, config.formatSpecified, format, config.Stdout())
	}

	switch format {
	case "yaml":
		for _, o := range objects {
			b, err := yaml.Marshal(o)
			if err != nil {
				return err
			}
			fmt.Fprintln(config.Stdout(), "---")
			fmt.Fprintf(config.Stdout(), "%s\n", b)
		}
		return nil
	case "json":
		encoder := json.NewEncoder(config.Stdout())
		encoder.SetIndent("", "  ")
		return encoder.Encode(objects)
	default:
		return fmt.Errorf("show: unsupported format %q", format)
	}
}

func newShowCommand(cp ConfigProvider) *cobra.Command {
	cmd := &cobra.Command{
		Use:     "show <environment>",
		Short:   "show output in YAML or JSON format for one or more components",
		Example: showExamples(),
	}

	config := showCommandConfig{
		filterFunc: addFilterParams(cmd, true),
	}

	cmd.Flags().StringVarP(&config.format, "format", "o", "yaml", "Output format. Supported values are: json, yaml")
	cmd.Flags().BoolVarP(&config.namesOnly, "objects", "O", false, "Only print names of objects instead of their contents")
	cmd.Flags().BoolVar(&config.sortAsApply, "sort-apply", false, "sort output in apply order (requires cluster access)")
	cmd.Flags().BoolVarP(&config.showSecrets, "show-secrets", "S", false, "do not obfuscate secret values in the output")

	cmd.RunE = func(c *cobra.Command, args []string) error {
		config.Config = cp()
		config.formatSpecified = c.Flags().Changed("format")
		return wrapError(doShow(args, config))
	}
	return cmd
}
