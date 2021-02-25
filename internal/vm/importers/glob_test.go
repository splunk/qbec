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
	"fmt"
	"io/ioutil"
	"os"
	"testing"

	"github.com/google/go-jsonnet"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func makeVM() (*jsonnet.VM, *GlobImporter) {
	vm := jsonnet.MakeVM()
	g1 := NewGlobImporter("import")
	g2 := NewGlobImporter("importstr")
	vm.Importer(
		NewCompositeImporter(
			g1,
			g2,
			NewFileImporter(&jsonnet.FileImporter{}),
		),
	)
	return vm, g1
}

type outputData map[string]interface{}

func generateFile(t *testing.T, virtFile string, code string) (outFile string) {
	file := virtFile + ".generated"
	err := ioutil.WriteFile(file, []byte(code), 0644)
	require.NoError(t, err)
	return file
}

func evaluateVirtual(t *testing.T, vm *jsonnet.VM, virtFile string, code string) outputData {
	file := generateFile(t, virtFile, code)
	defer os.Remove(file)
	jsonStr, err := vm.EvaluateFile(file)
	require.NoError(t, err)
	t.Logf("input from '%s'\n%s\noutput:%s\n", virtFile, code, jsonStr)
	var data outputData
	err = json.Unmarshal([]byte(jsonStr), &data)
	require.NoError(t, err)
	return data
}

func evaluateVirtualErr(t *testing.T, virtFile string, code string) error {
	file := generateFile(t, virtFile, code)
	defer os.Remove(file)
	vm, _ := makeVM()
	_, err := vm.EvaluateFile(file)
	require.Error(t, err)
	return err
}

func TestGlobSimple(t *testing.T) {
	vm, _ := makeVM()
	data := evaluateVirtual(t, vm, "testdata/caller.jsonnet", `import 'glob-import:example1/*.json'`)
	for _, k := range []string{"a", "b", "z"} {
		relFile := fmt.Sprintf("example1/%s.json", k)
		val, ok := data[relFile]
		require.True(t, ok)
		mVal, ok := val.(map[string]interface{})
		require.True(t, ok)
		_, ok = mVal[k]
		assert.True(t, ok)
	}
}

func TestGlobDoublestar(t *testing.T) {
	vm, _ := makeVM()
	data := evaluateVirtual(t, vm, "testdata/caller.jsonnet", `import 'glob-import:example2/**/*.json'`)
	expectedJSON := `
{
   "example2/inc1/a.json": {
	  "a": "a"
   },
   "example2/inc1/subdir/a.json": {
	  "a": "inner a"
   },
   "example2/inc2/a.json": {
	  "a": "long form a"
   }
}
`
	var expected interface{}
	err := json.Unmarshal([]byte(expectedJSON), &expected)
	require.Nil(t, err)
	assert.EqualValues(t, expected, data)
}

func TestDuplicateFileName(t *testing.T) {
	vm, _ := makeVM()
	data := evaluateVirtual(t, vm, "testdata/example2/caller.jsonnet", `import 'glob-import:inc?/*.json'`)
	_, firstOk := data["inc1/a.json"]
	require.True(t, firstOk)
	_, secondOk := data["inc2/a.json"]
	require.True(t, secondOk)
}

func TestGlobNoMatch(t *testing.T) {
	vm, _ := makeVM()
	data := evaluateVirtual(t, vm, "testdata/example1/caller/no-match.jsonnet", `import 'glob-import:*.json'`)
	require.Equal(t, 0, len(data))
}

func TestGlobImportStr(t *testing.T) {
	vm, _ := makeVM()
	file := generateFile(t, "testdata/example1/caller/synthesized.jsonnet", `importstr 'glob-import:../*.json'`)
	defer os.Remove(file)
	data, err := vm.EvaluateFile(file)
	require.NoError(t, err)
	var str string
	err = json.Unmarshal([]byte(data), &str)
	require.NoError(t, err)
	assert.Equal(t, `{
	'../a.json': import '../a.json',
	'../b.json': import '../b.json',
	'../z.json': import '../z.json',
}`, str)
}

func TestGlobImportStrVerb(t *testing.T) {
	vm, _ := makeVM()
	data := evaluateVirtual(t, vm, "testdata/example1/caller/synthesized.jsonnet", `import 'glob-importstr:../*.json'`)
	for _, k := range []string{"a", "b", "z"} {
		val, ok := data[fmt.Sprintf("../%s.json", k)]
		require.True(t, ok)
		mVal, ok := val.(string)
		require.True(t, ok)
		assert.Contains(t, mVal, fmt.Sprintf("%q", k))
	}
}

func TestGlobInternalCaching(t *testing.T) {
	a := assert.New(t)
	vm, gi := makeVM()
	_ = evaluateVirtual(t, vm, "testdata/example1/caller/synthesized.jsonnet", `import 'glob-import:../*.json'`)
	a.Equal(1, len(gi.cache))
	_ = evaluateVirtual(t, vm, "testdata/example1/caller/synthesized2.jsonnet", `import 'glob-import:../*.json'`)
	a.Equal(1, len(gi.cache))
	_ = evaluateVirtual(t, vm, "testdata/example1/caller2/synthesized.jsonnet", `import 'glob-import:../*.json'`)
	a.Equal(1, len(gi.cache))
	_ = evaluateVirtual(t, vm, "testdata/example1/caller/inner/synthesized.jsonnet", `import 'glob-import:../../*.json'`)
	a.Equal(2, len(gi.cache))
	_ = evaluateVirtual(t, vm, "testdata/example1/caller/inner/synthesized2.jsonnet", `import 'glob-import:../../[a,b].json'`)
	a.Equal(3, len(gi.cache))
}

func TestGlobNegativeCases(t *testing.T) {
	checkMsg := func(m string) func(t *testing.T, err error) {
		return func(t *testing.T, err error) {
			assert.Contains(t, err.Error(), m)
		}
	}
	tests := []struct {
		name     string
		expr     string
		asserter func(t *testing.T, err error)
	}{
		{
			name:     "bad path",
			expr:     `import 'glob-import:/bag-of-files/*.json'`,
			asserter: checkMsg(`RUNTIME ERROR: invalid glob pattern '/bag-of-files/*.json', cannot be absolute`),
		},
		{
			name: "bad pattern",
			expr: `import 'glob-import:../[.json'`,
			asserter: func(t *testing.T, err error) {
				assert.Contains(t, err.Error(), `RUNTIME ERROR: unable to expand glob`)
				assert.Contains(t, err.Error(), `[.json", syntax error in pattern`)
			},
		},
	}
	file := "testdata/example1/caller/synthesized.jsonnet"
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			test.asserter(t, evaluateVirtualErr(t, file, test.expr))
		})
	}
}

func TestGlobInit(t *testing.T) {
	require.Panics(t, func() { _ = NewGlobImporter("foobar") })
}
