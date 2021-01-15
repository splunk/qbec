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

package vm

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestVMEvalFile(t *testing.T) {
	vm := New([]string{"testdata/vmlib"})
	out, err := vm.EvalFile(
		"testdata/vmtest.jsonnet",
		VariableSet{}.WithVars(
			NewVar("foo", "fooVal"),
			NewCodeVar("bar", "true"),
		),
	)
	require.NoError(t, err)
	var data struct {
		Foo string `json:"foo"`
		Bar bool   `json:"bar"`
	}
	err = json.Unmarshal([]byte(out), &data)
	require.NoError(t, err)
	assert.Equal(t, "fooVal", data.Foo)
	assert.True(t, data.Bar)
}

func TestVMEvalNonExistentFile(t *testing.T) {
	vm := New(nil)
	_, err := vm.EvalFile("testdata/does-not-exist.jsonnet", VariableSet{})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "testdata/does-not-exist.jsonnet: file not found")
}

func TestVMEvalDir(t *testing.T) {
	vm := New(nil)
	_, err := vm.EvalFile("testdata", VariableSet{})
	require.Error(t, err)
	assert.Equal(t, err.Error(), "file 'testdata' was a directory")
}
