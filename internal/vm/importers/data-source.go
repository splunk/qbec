package importers

import (
	"fmt"
	"strings"

	"github.com/google/go-jsonnet"
	"github.com/pkg/errors"
)

const dsPrefix = "data"

type sourceEntry struct {
	contents jsonnet.Contents
	foundAt  string
	err      error
}

// DataSource is a named delegate that can resolve import paths. Multiple VMs may
// access a single instance of a data source. Thus, data source implementations must
// be safe for concurrent use.
type DataSource interface {
	// Name returns the name of this data source and is used to determine if
	// an import path should be processed by the data source importer.
	Name() string
	// Resolve resolves the absolute path defined for the data source to a string.
	Resolve(path string) (string, error)
}

// DataSourceImporter implements an importer that delegates to a data source
// for resolution.
type DataSourceImporter struct {
	delegate DataSource
	cache    map[string]*sourceEntry
	exact    string
	prefix   string
}

// NewDataSourceImporter returns an importer that can resolve paths for the specified datasource.
// It processes entries of the form
//    data-source://{name}[/{path-to-be-resolved}]
// If no path is provided, it is set to "/"
func NewDataSourceImporter(source DataSource) *DataSourceImporter {
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
