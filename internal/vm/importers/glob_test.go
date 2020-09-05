package importers

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"testing"

	"github.com/google/go-jsonnet"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func makeVM() (*jsonnet.VM, *GlobImporter) {
	vm := jsonnet.MakeVM()
	gi := NewGlobImporter()
	vm.Importer(
		NewCompositeImporter(
			gi,
			NewFileImporter(&jsonnet.FileImporter{}),
		),
	)
	return vm, gi
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

func evaluateFile(t *testing.T, file string) outputData {
	vm, _ := makeVM()
	b, err := ioutil.ReadFile(file)
	require.NoError(t, err)
	return evaluateVirtual(t, vm, file, string(b))
}

func TestGlobRealFiles(t *testing.T) {
	tests := []struct {
		name      string
		keyMapper func(k string) string
	}{
		{
			name:      "simple",
			keyMapper: func(k string) string { return fmt.Sprintf("../bag-of-files/%s.json", k) },
		},
		{
			name:      "explicit",
			keyMapper: func(k string) string { return fmt.Sprintf("../bag-of-files/%s.json", k) },
		},
		{
			name:      "dir1",
			keyMapper: func(k string) string { return fmt.Sprintf("bag-of-files/%s.json", k) },
		},
		{
			name:      "dir0",
			keyMapper: func(k string) string { return fmt.Sprintf("%s.json", k) },
		},
		{
			name:      "dir0-strip",
			keyMapper: func(k string) string { return k },
		},
		{
			name:      "dir-big",
			keyMapper: func(k string) string { return fmt.Sprintf("../bag-of-files/%s.json", k) },
		},
	}
	for _, test := range tests {
		name := test.name
		t.Run(name, func(t *testing.T) {
			assertData := func(data outputData, keyMapper func(string) string) {
				for _, k := range []string{"a", "b", "z"} {
					relFile := keyMapper(k)
					val, ok := data[relFile]
					require.True(t, ok)
					mVal, ok := val.(map[string]interface{})
					require.True(t, ok)
					_, ok = mVal[k]
					assert.True(t, ok)
				}
			}
			file := fmt.Sprintf("testdata/example1/caller/%s.jsonnet", name)
			data := evaluateFile(t, file)
			assertData(data, test.keyMapper)
		})
	}
}

func TestDuplicateFileName(t *testing.T) {
	data := evaluateFile(t, "testdata/example2/caller/good.jsonnet")
	_, firstOk := data["inc1/a"]
	require.True(t, firstOk)
	_, secondOk := data["inc2/a"]
	require.True(t, secondOk)

	vm, _ := makeVM()
	file := "testdata/example2/caller/bad.jsonnet"
	b, err := ioutil.ReadFile(file)
	require.NoError(t, err)
	_, err = vm.EvaluateSnippet(file, string(b))
	require.Error(t, err)
	assert.Contains(t, err.Error(), "RUNTIME ERROR: glob:../inc%3F/*.json?dirs=0&strip-extension=true: at least 2 files '../inc1/a.json' and '../inc2/a.json' map to the same key 'a'")
}

func TestGlobNoMatch(t *testing.T) {
	data := evaluateFile(t, "testdata/example1/caller/no-match.jsonnet")
	require.Equal(t, 0, len(data))
}

func TestGlobImportStr(t *testing.T) {
	vm, _ := makeVM()
	data, err := vm.EvaluateSnippet("testdata/example1/caller/synthesized.jsonnet", `importstr 'glob:../bag-of-files/*.json?dirs=0&strip-extension=true'`)
	require.NoError(t, err)
	var str string
	err = json.Unmarshal([]byte(data), &str)
	require.NoError(t, err)
	assert.Equal(t, `{
	'a': import '../bag-of-files/a.json',
	'b': import '../bag-of-files/b.json',
	'z': import '../bag-of-files/z.json',
}`, str)
}

func TestGlobImportStrVerb(t *testing.T) {
	vm, _ := makeVM()
	data := evaluateVirtual(t, vm, "testdata/example1/caller/synthesized.jsonnet", `import 'glob:../bag-of-files/*.json?dirs=0&strip-extension=true&verb=importstr'`)
	for _, k := range []string{"a", "b", "z"} {
		val, ok := data[k]
		require.True(t, ok)
		mVal, ok := val.(string)
		require.True(t, ok)
		assert.Contains(t, mVal, fmt.Sprintf("%q", k))
	}
}

func TestGlobInternalCaching(t *testing.T) {
	a := assert.New(t)
	vm, gi := makeVM()
	_ = evaluateVirtual(t, vm, "testdata/example1/caller/synthesized.jsonnet", `import 'glob:../bag-of-files/*.json'`)
	a.Equal(1, len(gi.cache))
	_ = evaluateVirtual(t, vm, "testdata/example1/caller/inner/synthesized.jsonnet", `import 'glob:../../bag-of-files/*.json'`)
	a.Equal(1, len(gi.cache))
	_ = evaluateVirtual(t, vm, "testdata/example1/caller/inner/synthesized.jsonnet", `import 'glob:../../bag-of-files/*.json?verb=import&strip-extension=false'`)
	a.Equal(1, len(gi.cache))
	_ = evaluateVirtual(t, vm, "testdata/example1/caller/inner/synthesized.jsonnet", `import 'glob:../../bag-of-files/*.json?verb=import&strip-extension=false&dirs=-2'`)
	a.Equal(1, len(gi.cache))
	_ = evaluateVirtual(t, vm, "testdata/example1/caller/inner/synthesized.jsonnet", `import 'glob:../../bag-of-files/*.json?verb=importstr'`)
	a.Equal(2, len(gi.cache))
	_ = evaluateVirtual(t, vm, "testdata/example1/caller/inner/synthesized.jsonnet", `import 'glob:../../bag-of-files/*.json?dirs=1'`)
	a.Equal(3, len(gi.cache))
	_ = evaluateVirtual(t, vm, "testdata/example1/caller/inner/synthesized.jsonnet", `import 'glob:../../bag-of-files/*.json?strip-extension=true'`)
	a.Equal(4, len(gi.cache))
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
			name:     "bad uri",
			expr:     fmt.Sprintf(`import 'glob:%c*.json'`, 0),
			asserter: checkMsg(`RUNTIME ERROR: parse "glob:\x00*.json": net/url: invalid control character in URL`),
		},
		{
			name:     "bad path",
			expr:     `import 'glob://../../bag-of-files/*.json'`,
			asserter: checkMsg(`RUNTIME ERROR: unable to parse URI "glob://../../bag-of-files/*.json", ensure you did not use '/' or '//' after 'glob:'`),
		},
		{
			name:     "bad param",
			expr:     `import 'glob:../../bag-of-files/*.json?foo=bar'`,
			asserter: checkMsg(`RUNTIME ERROR: glob:../../bag-of-files/*.json?foo=bar: invalid query parameter 'foo', allowed values are: dirs, strip-extension, verb`),
		},
		{
			name:     "bad verb",
			expr:     `import 'glob:../../bag-of-files/*.json?verb=frobnicate'`,
			asserter: checkMsg(`RUNTIME ERROR: glob:../../bag-of-files/*.json?verb=frobnicate: 'verb' parameter for glob import must be one of 'import' or 'importstr', found 'frobnicate'`),
		},
		{
			name:     "bad dirs",
			expr:     `import 'glob:../../bag-of-files/*.json?dirs=k'`,
			asserter: checkMsg(`RUNTIME ERROR: glob:../../bag-of-files/*.json?dirs=k: invalid value 'k' for 'dirs' parameter`),
		},
		{
			name:     "bad strip",
			expr:     `import 'glob:../../bag-of-files/*.json?strip-extension=TRUE'`,
			asserter: checkMsg(`RUNTIME ERROR: glob:../../bag-of-files/*.json?strip-extension=TRUE: invalid value 'TRUE' for 'strip-extension' parameter, must be 'true' or 'false'`),
		},
		{
			name:     "bad escape",
			expr:     `import 'glob:../../bag-of-files/*%3.json?strip-extension=TRUE'`,
			asserter: checkMsg(`RUNTIME ERROR: unable to unescape URI "glob:../../bag-of-files/*%3.json?strip-extension=TRUE", invalid URL escape "%3.`),
		},
		{
			name:     "bad pattern",
			expr:     `import 'glob:[.json'`,
			asserter: checkMsg(`RUNTIME ERROR: unable to expand glob "testdata/example1/caller/[.json", syntax error in pattern`),
		},
	}
	file := "testdata/example1/caller/synthesized.jsonnet"
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			test.asserter(t, evaluateVirtualErr(t, file, test.expr))
		})
	}
}
