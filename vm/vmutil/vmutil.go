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

// Package vmutil exposes specific functions used in the native implementation of the VM for general purpose use.
package vmutil

import (
	"io"

	"github.com/splunk/qbec/vm/internal/natives"
)

// ParseJSON parses the contents of the reader into a data object and returns it.
func ParseJSON(reader io.Reader) (interface{}, error) {
	return natives.ParseJSON(reader)
}

// ParseYAMLDocuments parses the contents of the reader into an array of
// objects, one for each non-nil document in the input.
func ParseYAMLDocuments(reader io.Reader) ([]interface{}, error) {
	return natives.ParseYAMLDocuments(reader)
}

// RenderYAMLDocuments renders the supplied data as a series of YAML documents if the input is an array
// or a single document when it is not. Nils are excluded from output.
// If the caller wants an array to be rendered as a single document,
// they need to wrap it in an array first. Note that this function is not a drop-in replacement for
// data that requires ghodss/yaml to be rendered correctly.
func RenderYAMLDocuments(data interface{}, writer io.Writer) (retErr error) {
	return natives.RenderYAMLDocuments(data, writer)
}
