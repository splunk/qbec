/*
Copyright 2021 Splunk Inc.

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
package vm

import (
	"testing"

	"github.com/google/go-jsonnet"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestVMScratchVariableSet(t *testing.T) {
	a := assert.New(t)
	c := VariableSet{}
	a.Nil(c.Vars())
	a.Nil(c.TopLevelVars())

	exts := []Var{
		NewVar("ext-foo", "bar"),
		NewCodeVar("ext-code-foo", "true"),
	}
	tlas := []Var{
		NewVar("tla-foo", "bar"),
		NewVar("tls-bar", "baz"),
		NewCodeVar("tla-code-foo", "100"),
		NewCodeVar("tla-code-bar", "true"),
	}

	c = c.WithVars(exts...).
		WithTopLevelVars(tlas...)
	a.Equal(2, len(c.Vars()))
	a.Equal(4, len(c.TopLevelVars()))
	a.True(c.HasTopLevelVar("tla-foo"))
	a.True(c.HasTopLevelVar("tla-code-foo"))
	a.False(c.HasTopLevelVar("ext-foo"))
	a.False(c.HasTopLevelVar("ext-code-foo"))

	a.False(c.HasVar("tla-foo"))
	a.False(c.HasVar("tla-code-foo"))
	a.True(c.HasVar("ext-foo"))
	a.True(c.HasVar("ext-code-foo"))

	c = c.WithoutTopLevel()
	a.False(c.HasTopLevelVar("tla-foo"))
	a.False(c.HasTopLevelVar("tla-code-foo"))
}

func TestVMNoopVariableSet(t *testing.T) {
	c := VariableSet{}
	newC := c.WithoutTopLevel().WithVars().
		WithTopLevelVars()
	assert.Equal(t, &newC, &c)
}

func TestVMBadCodeVar(t *testing.T) {
	c := VariableSet{}.WithVars(NewCodeVar("foo", "{ foo: bar"))
	jvm := jsonnet.MakeVM()
	c.register(jvm)
	_, err := jvm.EvaluateAnonymousSnippet("foo.jsonnet", `std.extVar('foo')`)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "<extvar:foo>:1:11 Expected a comma before next field")
}

func TestVMBadTLACodeVar(t *testing.T) {
	c := VariableSet{}.WithTopLevelVars(NewCodeVar("foo", "{ foo: bar"))
	jvm := jsonnet.MakeVM()
	c.register(jvm)
	_, err := jvm.EvaluateAnonymousSnippet("foo.jsonnet", `function (foo) foo`)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "<top-level-arg:foo>:1:11 Expected a comma before next field")
}

func TestVMBadCodeVarNoRef(t *testing.T) {
	c := VariableSet{}.WithVars(NewCodeVar("foo", "{ foo: bar"))
	jvm := jsonnet.MakeVM()
	c.register(jvm)
	ret, err := jvm.EvaluateAnonymousSnippet("foo.jsonnet", `10`)
	require.NoError(t, err)
	assert.Equal(t, "10\n", ret)
}
