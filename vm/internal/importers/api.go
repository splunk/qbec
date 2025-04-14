// Copyright 2025 Splunk Inc.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package importers

import "github.com/google/go-jsonnet"

// ExtendedImporter extends the jsonnet importer interface to add a new method that can determine whether
// an importer can be used for a path.
type ExtendedImporter interface {
	jsonnet.Importer
	CanProcess(path string) bool
}
