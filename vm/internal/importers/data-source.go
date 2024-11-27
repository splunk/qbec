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
	"strings"

	"github.com/google/go-jsonnet"
	"github.com/pkg/errors"
	"github.com/splunk/qbec/vm/datasource"
)

const dsPrefix = "data"

type sourceEntry struct {
	contents jsonnet.Contents
	foundAt  string
	err      error
}

// DataSourceImporter implements an importer that delegates to a data source
// for resolution.
type DataSourceImporter struct {
	delegate datasource.DataSource
	cache    map[string]*sourceEntry
	exact    string
	prefix   string
}

// NewDataSourceImporter returns an importer that can resolve paths for the specified datasource.
// It processes entries of the form
//
//	data://{name}[/{path-to-be-resolved}]
//
// If no path is provided, it is set to "/"
func NewDataSourceImporter(source datasource.DataSource) *DataSourceImporter {
	exact := fmt.Sprintf("%s://%s", dsPrefix, source.Name())
	ret := &DataSourceImporter{
		delegate: source,
		cache:    map[string]*sourceEntry{},
		exact:    exact,
		prefix:   exact + "/",
	}
	return ret
}

// CanProcess implements the interface method
func (d *DataSourceImporter) CanProcess(path string) bool {
	return path == d.exact || strings.HasPrefix(path, d.prefix)
}

// Import implements the interface method. For the datasource importer, imports are always considered absolute
// and the import-from path has no significance.
func (d *DataSourceImporter) Import(_, importedPath string) (contents jsonnet.Contents, foundAt string, err error) {
	target := importedPath[len(d.exact):]
	if target == "" {
		target = "/"
	}
	entry, ok := d.cache[target]
	if ok {
		return entry.contents, entry.foundAt, entry.err
	}
	ds := d.delegate
	content, err := ds.Resolve(target)
	err = errors.Wrapf(err, "data source %s, target=%s", ds.Name(), target) // nil ok
	entry = &sourceEntry{
		contents: jsonnet.MakeContents(content),
		foundAt:  importedPath,
		err:      err,
	}
	d.cache[target] = entry
	return entry.contents, entry.foundAt, entry.err
}
