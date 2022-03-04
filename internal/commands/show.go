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
	"context"
	"encoding/json"
	"fmt"
	"io"
	"strings"

	"github.com/ghodss/yaml"
	"github.com/spf13/cobra"
	"github.com/splunk/qbec/internal/cmd"
	"github.com/splunk/qbec/internal/model"
	"github.com/splunk/qbec/internal/objsort"
	"github.com/splunk/qbec/internal/sio"
	"github.com/splunk/qbec/internal/types"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
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
	case "json":
		encoder := json.NewEncoder(w)
		encoder.SetIndent("", "  ")
		return encoder.Encode(objects)
	default:
		b, err := yaml.Marshal(objects)
		if err != nil {
			return err
		}
		fmt.Fprintln(w, "---")
		fmt.Fprintf(w, "%s\n", b)
		return nil
	}
}

type showCommandConfig struct {
	cmd.AppContext
	showSecrets     bool
	format          string
	formatSpecified bool
	sortAsApply     bool
	namesOnly       bool
	filterFunc      func() (filterParams, error)
}

func removeMetadataKey(un *unstructured.Unstructured, name string) {
	meta := un.Object["metadata"]
	if m, ok := meta.(map[string]interface{}); ok {
		delete(m, name)
	}
}

func cleanMeta(obj model.K8sLocalObject) *unstructured.Unstructured {
	un := obj.ToUnstructured()
	annotations := un.GetAnnotations()
	labels := un.GetLabels()
	deleteQbecKeys := func(obj map[string]string) {
		for k := range obj {
			if strings.HasPrefix(k, model.QBECMetadataPrefix) {
				delete(obj, k)
			}
		}
	}
	deleteQbecKeys(labels)
	deleteQbecKeys(annotations)
	if len(labels) == 0 {
		removeMetadataKey(un, "labels")
	} else {
		un.SetLabels(labels)
	}
	if len(annotations) == 0 {
		removeMetadataKey(un, "annotations")
	} else {
		un.SetAnnotations(annotations)
	}
	return un
}

func doShow(ctx context.Context, args []string, config showCommandConfig) error {
	if len(args) != 1 {
		return cmd.NewUsageError(fmt.Sprintf("exactly one environment required, but provided: %q", args))
	}
	env := args[0]
	format := config.format
	if format != "json" && format != "yaml" {
		return cmd.NewUsageError(fmt.Sprintf("invalid output format: %q", format))
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

	envCtx, err := config.EnvContext(env)
	if err != nil {
		return err
	}

	objects, err := filteredObjects(ctx, envCtx, filterOpts{kf: keyFunc, fp: fp})
	if err != nil {
		return err
	}

	if !config.showSecrets {
		for i, o := range objects {
			objects[i], _ = types.HideSensitiveLocalInfo(o)
		}
	}

	if config.sortAsApply {
		if env == model.Baseline {
			sio.Warnln("cannot sort in apply order for baseline environment")
		} else {
			client, err := envCtx.Client()
			if err != nil {
				return err
			}
			objects = objsort.Sort(objects, sortConfig(client.IsNamespaced))
		}
	}

	if config.namesOnly {
		return showNames(objects, config.formatSpecified, format, config.Stdout())
	}

	var displayObjects []*unstructured.Unstructured
	mapper := func(o model.K8sLocalObject) *unstructured.Unstructured { return o.ToUnstructured() }

	if cleanEvalMode {
		mapper = cleanMeta
	}

	for _, o := range objects {
		displayObjects = append(displayObjects, mapper(o))
	}

	switch format {
	case "json":
		encoder := json.NewEncoder(config.Stdout())
		encoder.SetIndent("", "  ")
		return encoder.Encode(displayObjects)
	default:
		for _, o := range displayObjects {
			b, err := yaml.Marshal(o)
			if err != nil {
				return err
			}
			fmt.Fprintln(config.Stdout(), "---")
			fmt.Fprintf(config.Stdout(), "%s\n", b)
		}
		return nil
	}
}

func newShowCommand(cp ctxProvider) *cobra.Command {
	c := &cobra.Command{
		Use:     "show <environment>",
		Short:   "show output in YAML or JSON format for one or more components",
		Example: showExamples(),
	}

	config := showCommandConfig{
		filterFunc: addFilterParams(c, true),
	}

	var clean bool
	c.Flags().StringVarP(&config.format, "format", "o", "yaml", "Output format. Supported values are: json, yaml")
	c.Flags().BoolVarP(&config.namesOnly, "objects", "O", false, "Only print names of objects instead of their contents")
	c.Flags().BoolVar(&config.sortAsApply, "sort-apply", false, "sort output in apply order (requires cluster access)")
	c.Flags().BoolVar(&clean, "clean", false, "do not display qbec-generated labels and annotations")
	c.Flags().BoolVarP(&config.showSecrets, "show-secrets", "S", false, "do not obfuscate secret values in the output")

	c.RunE = func(c *cobra.Command, args []string) error {
		config.AppContext = cp()
		config.formatSpecified = c.Flags().Changed("format")
		cleanEvalMode = clean
		return cmd.WrapError(doShow(c.Context(), args, config))
	}
	return c
}
