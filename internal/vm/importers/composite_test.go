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
