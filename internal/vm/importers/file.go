package importers

import "github.com/google/go-jsonnet"

// NewFileImporter creates an extended file importer, wrapping the one supplied.
func NewFileImporter(jfi *jsonnet.FileImporter) *ExtendedFileImporter {
	return &ExtendedFileImporter{FileImporter: jfi}
}

// ExtendedFileImporter wraps a file importer and declares that it can import any path.
type ExtendedFileImporter struct {
	*jsonnet.FileImporter
}

// CanProcess implements the interface method.
func (e *ExtendedFileImporter) CanProcess(_ string) bool {
	return true
}
