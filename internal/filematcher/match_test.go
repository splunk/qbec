// Copyright 2025 Splunk Inc.
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

package filematcher_test

import (
	"fmt"
	"path/filepath"
	"testing"

	"github.com/splunk/qbec/internal/filematcher"
	"github.com/stretchr/testify/assert"
)

func TestMatch(t *testing.T) {
	cwd, err := filepath.Abs(".")
	assert.Nil(t, err)
	var tests = []struct {
		pattern       string
		expectedMatch bool
		expectedFiles []string
	}{
		{"testdata/*/env.yaml", false, nil},
		{"testdata/env.yaml", true, []string{filepath.Join(cwd, "testdata/env.yaml")}},
		{"testdata/non-existentenv.yaml", false, nil},
		{"testdata/*.yaml", true, []string{filepath.Join(cwd, "testdata/.env.yaml"), filepath.Join(cwd, "testdata/1env.yaml"), filepath.Join(cwd, "testdata/env.yaml"), filepath.Join(cwd, "testdata/env1.yaml")}},
		{"testdata", true, []string{filepath.Join(cwd, "testdata")}},
		{"https://testdata", true, []string{"https://testdata"}},
		{"testdata/testDirForGlobPatterns/*", true, []string{filepath.Join(cwd, "testdata/testDirForGlobPatterns/.keep"), filepath.Join(cwd, "testdata/testDirForGlobPatterns/childDir")}},
	}
	for i, test := range tests {
		t.Run(fmt.Sprintf("%d", i), func(t *testing.T) {
			files, err := filematcher.Match(test.pattern)
			assert.Equal(t, test.expectedMatch, err == nil)
			assert.Equal(t, test.expectedFiles, files)
		})
	}

}
