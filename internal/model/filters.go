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

package model

import (
	"fmt"
	"regexp"

	"github.com/pkg/errors"
	"github.com/spf13/pflag"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

// Namespaced provides metadata relating to whether a K8s type is namespaced or cluster scoped.
type Namespaced interface {
	IsNamespaced(gvk schema.GroupVersionKind) (bool, error)
}

// Filters is a collection of filters.
type Filters struct {
	includes              []string
	excludes              []string
	regexpObjectIncludes  []*regexp.Regexp
	regexpObjectExcludes  []*regexp.Regexp
	excludeClusterObjects bool
	kindFilter            Filter
	componentFilter       Filter
	namespaceFilter       Filter
}

// NewFilters sets up options in the supplied flags and returns a function to return filters.
func NewFilters(flags *pflag.FlagSet, includeAllFilters bool) func() (Filters, error) {
	var includes, excludes, kindIncludes, kindExcludes, nsIncludes, nsExcludes, regexpObjectIncludes, regexpObjectExcludes []string
	var includeClusterScopedObjects bool

	flags.StringArrayVarP(&includes, "component", "c", nil, "include just this component")
	flags.StringArrayVarP(&excludes, "exclude-component", "C", nil, "exclude this component")
	flags.StringArrayVarP(&regexpObjectIncludes, "object", "t", nil, "include k8s components matching this regexp")
	flags.StringArrayVarP(&regexpObjectExcludes, "exclude-object", "T", nil, "exclude k8s components matching this regexp")
	if includeAllFilters {
		flags.StringArrayVarP(&kindIncludes, "kind", "k", nil, "include objects with this kind")
		flags.StringArrayVarP(&kindExcludes, "exclude-kind", "K", nil, "exclude objects with this kind")
		flags.StringArrayVarP(&nsIncludes, "include-namespace", "p", nil, "include objects with this namespace")
		flags.StringArrayVarP(&nsExcludes, "exclude-namespace", "P", nil, "exclude objects with this namespace")
		flags.BoolVar(&includeClusterScopedObjects, "include-cluster-objects", true, "include cluster scoped objects, false by default when namespace filters present")
	}
	return func() (Filters, error) {
		of, err := newKindFilter(kindIncludes, kindExcludes)
		if err != nil {
			return Filters{}, err
		}
		cf, err := NewComponentFilter(includes, excludes)
		if err != nil {
			return Filters{}, err
		}
		nf, err := newStringFilter("namespaces", nsIncludes, nsExcludes)
		if err != nil {
			return Filters{}, err
		}
		if nf.HasFilters() {
			if !flags.Changed("include-cluster-objects") {
				includeClusterScopedObjects = false
			}
		}

		regexpIncludes := make([]*regexp.Regexp, 0, len(regexpObjectIncludes))
		regexpExcludes := make([]*regexp.Regexp, 0, len(regexpObjectExcludes))
		for _, re := range regexpObjectIncludes {
			regexpIncludes = append(regexpIncludes, regexp.MustCompile(fmt.Sprintf(`(?i)^%s$`, re)))
		}
		for _, re := range regexpObjectExcludes {
			regexpExcludes = append(regexpExcludes, regexp.MustCompile(fmt.Sprintf(`(?i)^%s$`, re)))
		}

		return Filters{
			includes:              includes,
			excludes:              excludes,
			regexpObjectIncludes:  regexpIncludes,
			regexpObjectExcludes:  regexpExcludes,
			kindFilter:            of,
			componentFilter:       cf,
			namespaceFilter:       nf,
			excludeClusterObjects: !includeClusterScopedObjects,
		}, nil
	}
}

// ComponentIncludes returns the components requested to be included
func (f Filters) ComponentIncludes() []string {
	return f.includes
}

// ComponentExcludes returns the components requested to be excluded
func (f Filters) ComponentExcludes() []string {
	return f.excludes
}

// GVKFilter returns true if the supplied GVK should be included.
func (f Filters) GVKFilter(gvk schema.GroupVersionKind) bool {
	return f.kindFilter != nil && f.kindFilter.ShouldInclude(gvk.Kind)
}

// ObjectFilter returns true if the name matches all include regexes and does not match all exclude regexes
// This code was inspired and adapted from grafana tanka
// https://github.com/grafana/tanka/blob/a6a63ac17f713d5fd64bb6d7972bc84c6b5902a6/pkg/process/filter.go#L74
func (f Filters) ObjectFilter(object K8sQbecMeta) bool {
	kindName := object.GetKind() + "/" + object.GetName()
	for _, re := range f.regexpObjectIncludes {
		if !re.MatchString(kindName) {
			return false
		}
	}
	for _, re := range f.regexpObjectExcludes {
		if re.MatchString(kindName) {
			return false
		}
	}
	return true
}

// HasNamespaceFilters returns true if filters based on namespace scope are in effect.
func (f Filters) HasNamespaceFilters() bool {
	return (f.namespaceFilter != nil && f.namespaceFilter.HasFilters()) || f.excludeClusterObjects
}

// Match returns true if the current filters match the supplied object. The client can be nil
// if namespace scope filters are not in effect.
func (f Filters) Match(o K8sQbecMeta, client Namespaced, defaultNS string) (bool, error) {
	if !f.ObjectFilter(o) {
		return false, nil
	}
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

func (f Filters) applyNamespaceFilters(o K8sQbecMeta, client Namespaced, defaultNs string) (bool, error) {
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
