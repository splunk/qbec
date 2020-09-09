package importers

import "github.com/google/go-jsonnet"

// ExtendedImporter extends the jsonnet importer interface to add a new method that can determine whether
// an importer can be used for a path.
type ExtendedImporter interface {
	jsonnet.Importer
	CanProcess(path string) bool
}
