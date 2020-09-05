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

func TestGlobImportStr(t *testing.T) {
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
