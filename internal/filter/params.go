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

// Package filter provides filtering operations for various object types.
package filter

import (
	"fmt"

	"github.com/pkg/errors"
	"github.com/spf13/pflag"
	"github.com/splunk/qbec/internal/model"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

// Namespaced provides metadata relating to whether a K8s type is namespaced or cluster scoped.
type Namespaced interface {
	IsNamespaced(gvk schema.GroupVersionKind) (bool, error)
}

// Params are filter parameters
type Params struct {
	includes              []string
	excludes              []string
	excludeClusterObjects bool
	kindFilter            model.Filter
	componentFilter       model.Filter
	namespaceFilter       model.Filter
}

// NewParams sets up options in the supplied flags and returns a function to return a Params.
func NewParams(flags *pflag.FlagSet, includeAllFilters bool) func() (Params, error) {
	var includes, excludes, kindIncludes, kindExcludes, nsIncludes, nsExcludes []string
	var includeClusterScopedObjects bool

	flags.StringArrayVarP(&includes, "component", "c", nil, "include just this component")
	flags.StringArrayVarP(&excludes, "exclude-component", "C", nil, "exclude this component")
	if includeAllFilters {
		flags.StringArrayVarP(&kindIncludes, "kind", "k", nil, "include objects with this kind")
		flags.StringArrayVarP(&kindExcludes, "exclude-kind", "K", nil, "exclude objects with this kind")
		flags.StringArrayVarP(&nsIncludes, "include-namespace", "p", nil, "include objects with this namespace")
		flags.StringArrayVarP(&nsExcludes, "exclude-namespace", "P", nil, "exclude objects with this namespace")
		flags.BoolVar(&includeClusterScopedObjects, "include-cluster-objects", true, "include cluster scoped objects, false by default when namespace filters present")
	}
	return func() (Params, error) {
		of, err := model.NewKindFilter(kindIncludes, kindExcludes)
		if err != nil {
			return Params{}, err
		}
		cf, err := model.NewComponentFilter(includes, excludes)
		if err != nil {
			return Params{}, err
		}
		nf, err := model.NewStringFilter("namespaces", nsIncludes, nsExcludes)
		if err != nil {
			return Params{}, err
		}
		if nf.HasFilters() {
			if !flags.Changed("include-cluster-objects") {
				includeClusterScopedObjects = false
			}
		}
		return Params{
			includes:              includes,
			excludes:              excludes,
			kindFilter:            of,
			componentFilter:       cf,
			namespaceFilter:       nf,
			excludeClusterObjects: !includeClusterScopedObjects,
		}, nil
	}
}

// ComponentIncludes returns the components reauested to be included
func (f Params) ComponentIncludes() []string {
	return f.includes
}

// ComponentExcludes returns the components reauested to be excluded
func (f Params) ComponentExcludes() []string {
	return f.excludes
}

// GVKFilter returns true if the supplied GVK should be included.
func (f Params) GVKFilter(gvk schema.GroupVersionKind) bool {
	return f.kindFilter != nil && f.kindFilter.ShouldInclude(gvk.Kind)
}

// HasNamespaceFilters returns true if filters based on namespace scope are in effect.
func (f Params) HasNamespaceFilters() bool {
	return (f.namespaceFilter != nil && f.namespaceFilter.HasFilters()) || f.excludeClusterObjects
}

// Match returns true if the current filters match the supplied object. The client can be nil
// if namespace scope filters are not in effect.
func (f Params) Match(o model.K8sQbecMeta, client Namespaced, defaultNS string) (bool, error) {
	if f.HasNamespaceFilters() && client == nil {
		return false, fmt.Errorf("no namespace metadata when namespace filters present")
	}
	if f.kindFilter != nil && !f.kindFilter.ShouldInclude(o.GetKind()) {
		return false, nil
	}
	if f.componentFilter != nil && !f.componentFilter.ShouldInclude(o.Component()) {
		return false, nil
	}
	if !f.HasNamespaceFilters() {
		return true, nil
	}
	return f.applyNamespaceFilters(o, client, defaultNS)
}

func (f Params) applyNamespaceFilters(o model.K8sQbecMeta, client Namespaced, defaultNs string) (bool, error) {
	isNamespaced, err := client.IsNamespaced(o.GroupVersionKind())
	if err != nil {
		return false, errors.Wrap(err, "namespace filter")
	}
	if !isNamespaced {
		return !f.excludeClusterObjects, nil
	}
	if f.namespaceFilter == nil {
		return true, nil
	}
	ns := o.GetNamespace()
	if ns == "" {
		ns = defaultNs
	}
	return f.namespaceFilter.ShouldInclude(ns), nil
}
