// Copyright 2021 Splunk Inc.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

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
