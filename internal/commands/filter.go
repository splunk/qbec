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
	"fmt"

	"github.com/spf13/cobra"
	"github.com/splunk/qbec/internal/cmd"
	"github.com/splunk/qbec/internal/eval"
	"github.com/splunk/qbec/internal/model"
	"github.com/splunk/qbec/internal/sio"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

type filterParams struct {
	includes              []string
	excludes              []string
	excludeClusterObjects bool
	kindFilter            model.Filter
	componentFilter       model.Filter
	namespaceFilter       model.Filter
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

func addFilterParams(c *cobra.Command, includeAllFilters bool) func() (filterParams, error) {
	var includes, excludes, kindIncludes, kindExcludes, nsIncludes, nsExcludes []string
	var includeClusterScopedObjects bool

	c.Flags().StringArrayVarP(&includes, "component", "c", nil, "include just this component")
	c.Flags().StringArrayVarP(&excludes, "exclude-component", "C", nil, "exclude this component")
	if includeAllFilters {
		c.Flags().StringArrayVarP(&kindIncludes, "kind", "k", nil, "include objects with this kind")
		c.Flags().StringArrayVarP(&kindExcludes, "exclude-kind", "K", nil, "exclude objects with this kind")
		c.Flags().StringArrayVarP(&nsIncludes, "include-namespace", "p", nil, "include objects with this namespace")
		c.Flags().StringArrayVarP(&nsExcludes, "exclude-namespace", "P", nil, "exclude objects with this namespace")
		c.Flags().BoolVar(&includeClusterScopedObjects, "include-cluster-objects", true, "include cluster scoped objects, false by default when namespace filters present")
	}
	return func() (filterParams, error) {
		of, err := model.NewKindFilter(kindIncludes, kindExcludes)
		if err != nil {
			return filterParams{}, cmd.NewUsageError(err.Error())
		}
		cf, err := model.NewComponentFilter(includes, excludes)
		if err != nil {
			return filterParams{}, cmd.NewUsageError(err.Error())
		}
		nf, err := model.NewNamespaceFilter(nsIncludes, nsExcludes)
		if err != nil {
			return filterParams{}, cmd.NewUsageError(err.Error())
		}
		if nf.HasFilters() {
			if !c.Flags().Changed("include-cluster-objects") {
				includeClusterScopedObjects = false
			}
		}
		return filterParams{
			includes:              includes,
			excludes:              excludes,
			kindFilter:            of,
			componentFilter:       cf,
			namespaceFilter:       nf,
			excludeClusterObjects: !includeClusterScopedObjects,
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

func checkDuplicates(ctx context.Context, objects []model.K8sLocalObject, kf keyFunc) error {
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

var cleanEvalMode bool

type filterElement struct {
	fn     func(object model.K8sLocalObject) (bool, error)
	warnFn func(orignalCount int)
}

func filteredObjects(ctx context.Context, envCtx cmd.EnvContext, kf keyFunc, fp filterParams) ([]model.K8sLocalObject, error) {
	components, err := envCtx.App().ComponentsForEnvironment(envCtx.Env(), fp.includes, fp.excludes)
	if err != nil {
		return nil, err
	}
	output, err := eval.Components(components, envCtx.EvalContext(cleanEvalMode), envCtx.ObjectProducer())
	if err != nil {
		return nil, err
	}
	if err := checkDuplicates(ctx, output, kf); err != nil {
		return nil, err
	}

	var filters []filterElement
	of := fp.kindFilter
	if of != nil && of.HasFilters() {
		filters = append(filters, filterElement{
			fn: func(o model.K8sLocalObject) (bool, error) {
				return of.ShouldInclude(o.GetKind()), nil
			},
			warnFn: func(count int) {
				sio.Warnf("0 of %d matches for kind filter, check for typos and abbreviations\n", count)
			},
		})
	}
	nf := fp.namespaceFilter
	hasNSFilters := nf != nil && nf.HasFilters()

	if hasNSFilters || fp.excludeClusterObjects {
		client, err := envCtx.Client()
		if err != nil {
			return nil, err
		}
		defaultNs := envCtx.App().DefaultNamespace(envCtx.Env())
		filters = append(filters, filterElement{
			fn: func(o model.K8sLocalObject) (bool, error) {
				isNamespaced, err := client.IsNamespaced(o.GroupVersionKind())
				if err != nil {
					return false, err
				}
				if !isNamespaced {
					return !fp.excludeClusterObjects, nil
				}
				if !hasNSFilters {
					return true, nil
				}
				ns := o.GetNamespace()
				if ns == "" {
					ns = defaultNs
				}
				return nf.ShouldInclude(ns), nil
			},
			warnFn: func(count int) {
				sio.Warnf("0 of %d matches for namespace filter, check for typos\n", count)
			},
		})
	}

	ret := output
	for _, filter := range filters {
		prev := ret
		ret = nil
		for _, o := range prev {
			b, err := filter.fn(o)
			if err != nil {
				return nil, err
			}
			if b {
				ret = append(ret, o)
			}
		}
		if len(prev) > 0 && len(ret) == 0 {
			filter.warnFn(len(prev))
			return ret, nil
		}
	}

	return ret, nil
}
