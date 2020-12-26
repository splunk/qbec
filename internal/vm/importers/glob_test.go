package importers

import (
	"encoding/json"
	"fmt"
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

func evaluateVirtual(t *testing.T, vm *jsonnet.VM, virtFile string, code string) outputData {
	jsonStr, err := vm.EvaluateSnippet(virtFile, code)
	require.NoError(t, err)
	t.Logf("input from '%s'\n%s\noutput:%s\n", virtFile, code, jsonStr)
	var data outputData
	err = json.Unmarshal([]byte(jsonStr), &data)
	require.NoError(t, err)
	return data
}

func evaluateVirtualErr(t *testing.T, virtFile string, code string) error {
	vm, _ := makeVM()
	_, err := vm.EvaluateSnippet(virtFile, code)
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
	data, err := vm.EvaluateSnippet("testdata/example1/caller/synthesized.jsonnet", `importstr 'glob-import:../*.json'`)
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
	_ = evaluateVirtual(t, vm, "testdata/example1/caller/inner/synthesized.jsonnet", `import 'glob-import:../../[a,b].json'`)
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
