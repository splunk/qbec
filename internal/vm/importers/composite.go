package importers

import (
	"fmt"

	"github.com/google/go-jsonnet"
)

// CompositeImporter tries multiple extended importers in sequence for a given path
type CompositeImporter struct {
	importers []ExtendedImporter
}

// NewCompositeImporter creates a composite importer with the supplied extended importers. Note that if
// two importers could match the same path, the first one will be used so order is important.
func NewCompositeImporter(importers ...ExtendedImporter) *CompositeImporter {
	return &CompositeImporter{importers: importers}
}

// Import implements the interface method by delegating to installed importers in sequence
func (c *CompositeImporter) Import(importedFrom, importedPath string) (contents jsonnet.Contents, foundAt string, err error) {
	for _, importer := range c.importers {
		if importer.CanProcess(importedPath) {
			return importer.Import(importedFrom, importedPath)
		}
	}
	return contents, foundAt, fmt.Errorf("no importer for path %s", importedPath)
}

// CanProcess implements the interface method of the ExtendedImporter
func (c *CompositeImporter) CanProcess(importedPath string) bool {
	for _, importer := range c.importers {
		if importer.CanProcess(importedPath) {
			return true
		}
	}
	return false
}
