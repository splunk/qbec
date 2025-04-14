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

package filematcher

import (
	"fmt"
	"io/fs"
	"path/filepath"
	"sort"
	"strings"
)

// Match returns a sorted list of files and dirs matching a glob pattern
func Match(pattern string) ([]string, error) {
	if IsRemoteFile(pattern) {
		return []string{pattern}, nil
	}
	files, err := filepath.Glob(pattern)
	if err != nil {
		return nil, err
	}
	// files is nil when pattern does not match any files
	if files == nil {
		return nil, fmt.Errorf("%s: %w", pattern, fs.ErrNotExist)
	}
	var envFiles []string
	for _, f := range files {
		abs, err := filepath.Abs(f)
		if err != nil {
			return nil, err
		}
		envFiles = append(envFiles, abs)
	}
	sort.Strings(envFiles)
	return envFiles, nil
}

// IsRemoteFile distinguishes remote files from local files
func IsRemoteFile(file string) bool {
	return strings.HasPrefix(file, "http://") || strings.HasPrefix(file, "https://")
}
