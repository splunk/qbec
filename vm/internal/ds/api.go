package ds

import (
	"io"

	"github.com/splunk/qbec/vm/datasource"
)

// DataSourceWithLifecycle represents an external data source that implements the methods needed by the data source
// as well as lifecycle methods to handle initialization and clean up.
type DataSourceWithLifecycle interface {
	datasource.DataSource
	Init(c datasource.ConfigProvider) error
	io.Closer
}
