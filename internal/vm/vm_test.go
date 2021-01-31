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

	"github.com/splunk/qbec/internal/vm/importers"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestVMEvalFile(t *testing.T) {
	vm := New(Config{LibPaths: []string{"testdata/vmlib"}})
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
	vm := New(Config{})
	_, err := vm.EvalFile("testdata/does-not-exist.jsonnet", VariableSet{})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "testdata/does-not-exist.jsonnet: file not found")
}

func TestVMEvalDir(t *testing.T) {
	vm := New(Config{})
	_, err := vm.EvalFile("testdata", VariableSet{})
	require.Error(t, err)
	assert.Equal(t, err.Error(), "file 'testdata' was a directory")
}

type replay struct {
	name string
}

func (d *replay) Name() string {
	return d.name
}

func (d *replay) Resolve(path string) (string, error) {
	out := struct {
		Source string `json:"source"`
		Path   string `json:"path"`
	}{d.name, path}
	b, err := json.MarshalIndent(out, "", "  ")
	if err != nil {
		return "", err
	}
	return string(b), nil
}

func TestVMDataSource(t *testing.T) {
	jvm := New(Config{
		LibPaths:    []string{},
		DataSources: []importers.DataSource{&replay{name: "replay"}},
	})
	jsonCode, err := jvm.EvalFile("testdata/data-sources/replay.jsonnet", VariableSet{})
	require.NoError(t, err)
	var data map[string]interface{}
	err = json.Unmarshal([]byte(jsonCode), &data)
	require.NoError(t, err)
	assert.Equal(t, "replay", data["source"])
	assert.Equal(t, "/foo/bar", data["path"])
}

func TestVMTwoDataSources(t *testing.T) {
	jvm := New(Config{
		LibPaths: []string{},
		DataSources: []importers.DataSource{
			&replay{name: "replay"},
			&replay{name: "replay2"},
		},
	})
	jsonCode, err := jvm.EvalFile("testdata/data-sources/replay2.jsonnet", VariableSet{})
	require.NoError(t, err)
	var data []map[string]interface{}
	err = json.Unmarshal([]byte(jsonCode), &data)
	require.NoError(t, err)
	require.Equal(t, 2, len(data))
	assert.Equal(t, "replay", data[0]["source"])
	assert.Equal(t, "/foo/bar", data[0]["path"])
	assert.Equal(t, "replay2", data[1]["source"])
	assert.Equal(t, "/bar/baz", data[1]["path"])
}
