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

// Package api defines the data source interface
package api

import (
	"io"

	"github.com/splunk/qbec/internal/vm/importers"
)

// ConfigProvider returns the value of the supplied variable as a JSON string.
type ConfigProvider func(varName string) (string, error)

// DataSource represents an external data source that implements the methods needed by the data source importer
// as well as lifecycle methods to handle clean up of temporary resources.
type DataSource interface {
	Init(c ConfigProvider) error
	importers.DataSource
	io.Closer
}
