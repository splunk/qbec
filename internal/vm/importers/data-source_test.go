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

package importers

import (
	"encoding/json"
	"testing"

	"github.com/google/go-jsonnet"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type replay struct{}

func (r replay) Name() string                        { return "replay" }
func (r replay) Resolve(path string) (string, error) { return path, nil }

func TestDataSourceImporterBasic(t *testing.T) {
	imp := NewDataSourceImporter(replay{})
	vm := jsonnet.MakeVM()
	vm.Importer(NewCompositeImporter(imp))
	jsonCode, err := vm.EvaluateAnonymousSnippet("test.jsonnet", `{ foo: importstr 'data://replay/foo/bar' }`)
	require.NoError(t, err)
	var data struct {
		Foo string `json:"foo"`
	}
	err = json.Unmarshal([]byte(jsonCode), &data)
	require.NoError(t, err)
	assert.Equal(t, "/foo/bar", data.Foo)
}

func TestDataSourceImporterNoPathc(t *testing.T) {
	imp := NewDataSourceImporter(replay{})
	vm := jsonnet.MakeVM()
	vm.Importer(NewCompositeImporter(imp))
	jsonCode, err := vm.EvaluateAnonymousSnippet("test.jsonnet", `{ foo: importstr 'data://replay', bar: importstr 'data://replay' }`)
	require.NoError(t, err)
	var data struct {
		Foo string `json:"foo"`
	}
	err = json.Unmarshal([]byte(jsonCode), &data)
	require.NoError(t, err)
	assert.Equal(t, "/", data.Foo)
	assert.Equal(t, 1, len(imp.cache))
}
