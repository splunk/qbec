package importers

import (
	"testing"

	"github.com/google/go-jsonnet"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCompositeImporter(t *testing.T) {
	vm := jsonnet.MakeVM()
	vm.Importer(
		NewCompositeImporter(
			NewGlobImporter(),
			NewFileImporter(&jsonnet.FileImporter{}),
		),
	)
	_, err := vm.EvaluateSnippet("testdata/example1/caller/caller.json", `import '../bag-of-files/a.json'`)
	require.NoError(t, err)

	_, err = vm.EvaluateSnippet("testdata/example1/caller/caller.json", `import 'glob:../bag-of-files/*.json'`)
	require.NoError(t, err)

	vm = jsonnet.MakeVM()
	vm.Importer(
		NewCompositeImporter(
			NewGlobImporter(),
		),
	)
	_, err = vm.EvaluateSnippet("testdata/example1/caller/caller.json", `import '../bag-of-files/a.json'`)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "RUNTIME ERROR: no importer for path ../bag-of-files/a.json")
}
