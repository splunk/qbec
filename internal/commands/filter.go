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
	"fmt"

	"github.com/spf13/cobra"
	"github.com/splunk/qbec/internal/eval"
	"github.com/splunk/qbec/internal/model"
	"github.com/splunk/qbec/internal/sio"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

type filterParams struct {
	includes        []string
	excludes        []string
	kindFilter      model.Filter
	componentFilter model.Filter
}

func (f filterParams) GVKFilter(gvk schema.GroupVersionKind) bool {
	return f.kindFilter == nil || f.kindFilter.ShouldInclude(gvk.Kind)
}

func (f filterParams) Includes(o model.K8sQbecMeta) bool {
	if !(f.kindFilter == nil || f.kindFilter.ShouldInclude(o.GetKind())) {
		return false
	}
	if !(f.componentFilter == nil || f.componentFilter.ShouldInclude(o.Component())) {
		return false
	}
	return true
}

func addFilterParams(cmd *cobra.Command, includeKindFilters bool) func() (filterParams, error) {
	var includes, excludes, kindIncludes, kindExcludes []string

	cmd.Flags().StringArrayVarP(&includes, "component", "c", nil, "include just this component")
	cmd.Flags().StringArrayVarP(&excludes, "exclude-component", "C", nil, "exclude this component")
	if includeKindFilters {
		cmd.Flags().StringArrayVarP(&kindIncludes, "kind", "k", nil, "include objects with this kind")
		cmd.Flags().StringArrayVarP(&kindExcludes, "exclude-kind", "K", nil, "exclude objects with this kind")
	}
	return func() (filterParams, error) {
		of, err := model.NewKindFilter(kindIncludes, kindExcludes)
		if err != nil {
			return filterParams{}, newUsageError(err.Error())
		}
		cf, err := model.NewComponentFilter(includes, excludes)
		if err != nil {
			return filterParams{}, newUsageError(err.Error())
		}
		return filterParams{
			includes:        includes,
			excludes:        excludes,
			kindFilter:      of,
			componentFilter: cf,
		}, nil
	}
}

// keyFunc is a function that provides a string key for an object
type keyFunc func(object model.K8sMeta) string

func displayName(obj model.K8sLocalObject) string {
	group := obj.GroupVersionKind().Group
	if group != "" {
		group += "/"
	}
	ns := obj.GetNamespace()
	if ns != "" {
		ns += "/"
	}
	return fmt.Sprintf("%s%s %s%s (component: %s)", group, obj.GetKind(), ns, obj.GetName(), obj.Component())
}

func checkDuplicates(objects []model.K8sLocalObject, kf keyFunc) error {
	if kf == nil {
		return nil
	}
	objectsByKey := map[string]model.K8sLocalObject{}
	for _, o := range objects {
		if o.GetName() == "" { // generated name
			continue
		}
		key := kf(o)
		if prev, ok := objectsByKey[key]; ok {
			return fmt.Errorf("duplicate objects %s and %s", displayName(prev), displayName(o))
		}
		objectsByKey[key] = o
	}
	return nil
}

func filteredObjects(cfg *config, env string, kf keyFunc, fp filterParams) ([]model.K8sLocalObject, error) {
	components, err := cfg.App().ComponentsForEnvironment(env, fp.includes, fp.excludes)
	if err != nil {
		return nil, err
	}
	props, err := cfg.App().Properties(env)
	if err != nil {
		return nil, err
	}
	output, err := eval.Components(components, cfg.EvalContext(env, props))
	if err != nil {
		return nil, err
	}
	if err := checkDuplicates(output, kf); err != nil {
		return nil, err
	}
	of := fp.kindFilter
	if of == nil || !of.HasFilters() {
		return output, nil
	}
	var ret []model.K8sLocalObject
	for _, o := range output {
		if of.ShouldInclude(o.GetKind()) {
			ret = append(ret, o)
		}
	}
	if len(output) > 0 && len(ret) == 0 {
		sio.Warnf("0 of %d matches for kind filter, check for typos and abbreviations\n", len(output))
	}
	return ret, nil
}
