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

	"github.com/stretchr/testify/assert"
)

func TestVMScratchVariableSet(t *testing.T) {
	a := assert.New(t)
	c := VariableSet{}
	a.NotNil(c.Vars())
	a.NotNil(c.CodeVars())
	a.NotNil(c.TopLevelVars())
	a.NotNil(c.TopLevelCodeVars())

	tla, tlacode, extStr, extCode := map[string]string{"tla-foo": "bar", "tls-bar": "baz"},
		map[string]string{"tla-code-foo": "100", "tla-code-bar": "true"},
		map[string]string{"ext-foo": "bar"},
		map[string]string{"ext-code-foo": "true"}

	c = c.WithVars(extStr).
		WithCodeVars(extCode).
		WithTopLevelVars(tla).
		WithTopLevelCodeVars(tlacode)

	a.EqualValues(extStr, c.Vars())
	a.EqualValues(extCode, c.CodeVars())
	a.EqualValues(tla, c.TopLevelVars())
	a.EqualValues(tlacode, c.TopLevelCodeVars())
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
	newC := c.WithoutTopLevel().WithVars(nil).WithCodeVars(map[string]string{}).
		WithTopLevelVars(nil).WithTopLevelCodeVars(nil)
	assert.Equal(t, &newC, &c)
}
