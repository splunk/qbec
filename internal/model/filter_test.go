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
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestComponentFilterIncludes(t *testing.T) {
	filter, err := NewComponentFilter([]string{"foo", "bar"}, []string{})
	require.Nil(t, err)
	a := assert.New(t)
	a.True(filter.HasFilters())
	a.True(filter.ShouldInclude("foo"))
	a.True(filter.ShouldInclude("bar"))
	a.False(filter.ShouldInclude("baz"))
}

func TestComponentFilterExcludes(t *testing.T) {
	filter, err := NewComponentFilter(nil, []string{"foo", "bar"})
	require.Nil(t, err)
	a := assert.New(t)
	a.True(filter.HasFilters())
	a.False(filter.ShouldInclude("foo"))
	a.False(filter.ShouldInclude("bar"))
	a.True(filter.ShouldInclude("baz"))
}

func TestComponentFilterOpen(t *testing.T) {
	filter, err := NewComponentFilter(nil, nil)
	require.Nil(t, err)
	a := assert.New(t)
	a.False(filter.HasFilters())
	a.True(filter.ShouldInclude("foo"))
	a.True(filter.ShouldInclude("bar"))
	a.True(filter.ShouldInclude("baz"))
}

func TestComponentFilterBad(t *testing.T) {
	_, err := NewComponentFilter([]string{"foo", "bar"}, []string{"baz"})
	require.NotNil(t, err)
	require.Equal(t, "cannot include as well as exclude components, specify one or the other", err.Error())
}

func TestKindFilterIncludes(t *testing.T) {
	filter, err := newKindFilter([]string{"foo", "icy"}, []string{})
	require.Nil(t, err)
	a := assert.New(t)
	a.True(filter.HasFilters())
	a.True(filter.ShouldInclude("FOO"))
	a.True(filter.ShouldInclude("foo"))
	a.True(filter.ShouldInclude("icy"))
	a.False(filter.ShouldInclude("baz"))
}

func TestKindFilterIncludesPlural(t *testing.T) {
	filter, err := newKindFilter([]string{"foos", "icies", "classes"}, []string{})
	require.Nil(t, err)
	a := assert.New(t)
	a.True(filter.HasFilters())
	a.True(filter.ShouldInclude("foo"))
	a.True(filter.ShouldInclude("icy"))
	a.True(filter.ShouldInclude("class"))
	a.True(filter.ShouldInclude("ICIES"))
	a.True(filter.ShouldInclude("ICY"))
	a.False(filter.ShouldInclude("baz"))
}

func TestKindFilterExcludes(t *testing.T) {
	filter, err := newKindFilter(nil, []string{"foo", "bar"})
	require.Nil(t, err)
	a := assert.New(t)
	a.True(filter.HasFilters())
	a.False(filter.ShouldInclude("foo"))
	a.False(filter.ShouldInclude("bar"))
	a.True(filter.ShouldInclude("baz"))
}

func TestKindFilterExcludesPlural(t *testing.T) {
	filter, err := newKindFilter(nil, []string{"foos", "bars"})
	require.Nil(t, err)
	a := assert.New(t)
	a.True(filter.HasFilters())
	a.False(filter.ShouldInclude("foo"))
	a.False(filter.ShouldInclude("foos"))
	a.False(filter.ShouldInclude("bar"))
	a.False(filter.ShouldInclude("BARS"))
	a.True(filter.ShouldInclude("baz"))
}

func TestKindFilterOpen(t *testing.T) {
	filter, err := newKindFilter(nil, nil)
	require.Nil(t, err)
	a := assert.New(t)
	a.False(filter.HasFilters())
	a.True(filter.ShouldInclude("foo"))
	a.True(filter.ShouldInclude("bar"))
	a.True(filter.ShouldInclude("baz"))
}

func TestKindFilterBad(t *testing.T) {
	_, err := newKindFilter([]string{"foo", "bar"}, []string{"baz"})
	require.NotNil(t, err)
	require.Equal(t, "cannot include as well as exclude kinds, specify one or the other", err.Error())
}
