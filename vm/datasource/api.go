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

// Package datasource declares the data source interface.
package datasource

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

// ConfigProvider returns the value of the supplied variable as a JSON string.
// A config provider is used at the time of data source creation to allow the data source to be
// correctly configured.
type ConfigProvider func(varName string) (string, error)
