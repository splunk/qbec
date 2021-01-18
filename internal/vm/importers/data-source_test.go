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
