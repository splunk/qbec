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

package model

import (
	"fmt"
	"strings"

	"k8s.io/gengo/namer"
	"k8s.io/gengo/types"
)

// Filter filters inputs.
type Filter interface {
	HasFilters() bool            // returns true if filtering is needed
	ShouldInclude(s string) bool // returns true if the supplied string matches the filter
}

type aliasFn func(string) []string

type baseFilter struct {
	includes map[string]bool
	excludes map[string]bool
	aliasFn  aliasFn
}

func (b *baseFilter) HasFilters() bool {
	return len(b.includes) > 0 || len(b.excludes) > 0
}

func (b *baseFilter) ShouldInclude(s string) bool {
	check := b.aliasFn(s)
	for _, name := range check {
		if b.includes[name] {
			return true
		}
		if b.excludes[name] {
			return false
		}
	}
	return len(b.includes) == 0
}

func newBaseFilter(pluralKind string, includes, excludes []string, fn aliasFn) (*baseFilter, error) {
	if len(includes) > 0 && len(excludes) > 0 {
		return nil, fmt.Errorf("cannot include as well as exclude %s, specify one or the other", pluralKind)
	}
	toMap := func(list []string) map[string]bool {
		ret := map[string]bool{}
		for _, item := range list {
			ret[item] = true
		}
		return ret
	}
	if fn == nil {
		fn = func(s string) []string { return []string{s} }
	}
	return &baseFilter{
		includes: toMap(includes),
		excludes: toMap(excludes),
		aliasFn:  fn,
	}, nil
}

// NewComponentFilter returns a filter for component names.
func NewComponentFilter(includes, excludes []string) (Filter, error) {
	return newStringFilter("components", includes, excludes)
}

// newStringFilter returns a filter for exact string matches.
func newStringFilter(kind string, includes, excludes []string) (Filter, error) {
	nf, err := newBaseFilter(kind, includes, excludes, nil)
	if err != nil {
		return nil, err
	}
	return nf, nil
}

// newKindFilter returns a filter for object kinds that ignores case and takes
// pluralization into account.
func newKindFilter(includes, excludes []string) (Filter, error) {
	aliases := func(s string) []string {
		n := namer.NewAllLowercasePluralNamer(nil)
		kind := strings.ToLower(s)
		plural := n.Name(&types.Type{Name: types.Name{Name: kind}})
		return []string{kind, plural}
	}
	mapLower := func(input []string) []string {
		var ret []string
		for _, s := range input {
			ret = append(ret, strings.ToLower(s))
		}
		return ret
	}
	bf, err := newBaseFilter("kinds", mapLower(includes), mapLower(excludes), aliases)
	if err != nil {
		return nil, err
	}
	return bf, nil
}
