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

import "github.com/splunk/qbec/internal/vm/importers"

// DataSource represents an external data source that implements the methods needed by the data source importer
// as well as lifecycle methods that can be used by the caller to start and stop it.
type DataSource interface {
	importers.DataSource
	// Start starts the data source with a configuration that is typically the external vars set
	// by the user. The implementation may choose to consume these variables as it chooses.
	// Once Start returns, it is assumed that the data source is ready for resolution requests.
	Start(vars map[string]interface{}) error
	// Close releases temporary resources used by the implementation.
	Close() error
}
