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

package importers

import (
	"testing"

	"github.com/google/go-jsonnet"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCompositeImporter(t *testing.T) {
	a := assert.New(t)
	vm := jsonnet.MakeVM()
	c := NewCompositeImporter(
		NewGlobImporter("import"),
		NewGlobImporter("importstr"),
		NewFileImporter(&jsonnet.FileImporter{}),
	)
	vm.Importer(c)
	a.True(c.CanProcess("glob-import:*.libsonnet"))
	a.True(c.CanProcess("glob-importstr:*.yaml"))
	a.True(c.CanProcess("a.yaml"))

	_, err := vm.EvaluateFile("testdata/example1/caller/import-a.jsonnet")
	require.NoError(t, err)

	_, err = vm.EvaluateFile("testdata/example1/caller/import-all-json.jsonnet")
	require.NoError(t, err)

	vm = jsonnet.MakeVM()
	c = NewCompositeImporter(
		NewGlobImporter("import"),
		NewGlobImporter("importstr"),
	)
	vm.Importer(c)
	a.True(c.CanProcess("glob-import:*.libsonnet"))
	a.True(c.CanProcess("glob-importstr:*.yaml"))
	a.False(c.CanProcess("a.yaml"))

	_, err = vm.EvaluateAnonymousSnippet("testdata/example1/caller/caller.jsonnet", `import '../bag-of-files/a.json'`)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "RUNTIME ERROR: no importer for path ../bag-of-files/a.json")
}
