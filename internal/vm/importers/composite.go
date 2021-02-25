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
