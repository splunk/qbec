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

package fswalk

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"github.com/spf13/pflag"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type proc struct {
	extensions []string
	errorsFor  []string
	seenFiles  []string
}

func (p *proc) Matches(path string, file fs.FileInfo, userSpecified bool) bool {
	if userSpecified {
		return true
	}
	for _, e := range p.extensions {
		if strings.HasSuffix(path, e) {
			return true
		}
	}
	return false
}
func (p *proc) Process(path string, file fs.FileInfo) error {
	p.seenFiles = append(p.seenFiles, path)
	if file.IsDir() {
		return fmt.Errorf("processor got a dir")
	}
	sort.Strings(p.seenFiles)
	for _, e := range p.errorsFor {
		if strings.HasSuffix(path, e) {
			return fmt.Errorf("error processing %s", path)
		}
	}
	return nil
}

func TestProcess(t *testing.T) {
	tests := []struct {
		name            string
		p               *proc
		specified       []string
		exclusions      []string
		continueOnError bool
		assertFn        func(t *testing.T, p *proc, err error)
	}{
		{
			name: "happy",
			p: &proc{
				extensions: []string{".libsonnet", ".jsonnet"},
			},
			specified: []string{"testdata"},
			assertFn: func(t *testing.T, p *proc, err error) {
				require.NoError(t, err)
				assert.Equal(t, 3, len(p.seenFiles))
			},
		},
		{
			name: "happy-specified",
			p: &proc{
				extensions: []string{".libsonnet", ".jsonnet"},
			},
			specified: []string{"testdata", "testdata/0.txt"},
			assertFn: func(t *testing.T, p *proc, err error) {
				require.NoError(t, err)
				assert.Equal(t, 4, len(p.seenFiles))
			},
		},
		{
			name: "errors no continue",
			p: &proc{
				extensions: []string{".libsonnet", ".jsonnet"},
				errorsFor:  []string{"1.jsonnet", "2.jsonnet"},
			},
			specified: []string{"testdata"},
			assertFn: func(t *testing.T, p *proc, err error) {
				require.Error(t, err)
				assert.Equal(t, 1, len(p.seenFiles))
				assert.Contains(t, err.Error(), "error processing")
			},
		},
		{
			name: "errors continue",
			p: &proc{
				extensions: []string{".libsonnet", ".jsonnet"},
				errorsFor:  []string{"1.jsonnet", "2.jsonnet"},
			},
			specified:       []string{"testdata"},
			continueOnError: true,
			assertFn: func(t *testing.T, p *proc, err error) {
				require.Error(t, err)
				assert.Equal(t, 3, len(p.seenFiles))
				assert.Contains(t, err.Error(), "2 errors encountered")
			},
		},
		{
			name: "single error continue",
			p: &proc{
				extensions: []string{".libsonnet", ".jsonnet"},
				errorsFor:  []string{"1.jsonnet"},
			},
			specified:       []string{"testdata"},
			continueOnError: true,
			assertFn: func(t *testing.T, p *proc, err error) {
				require.Error(t, err)
				assert.Equal(t, 3, len(p.seenFiles))
				assert.Contains(t, err.Error(), "1 error encountered")
			},
		},
		{
			name: "exclude dir",
			p: &proc{
				extensions: []string{".libsonnet", ".jsonnet"},
			},
			specified:  []string{"testdata"},
			exclusions: []string{filepath.Join("testdata", "dir2")},
			assertFn: func(t *testing.T, p *proc, err error) {
				require.NoError(t, err)
				assert.Equal(t, 2, len(p.seenFiles))
			},
		},
		{
			name: "exclude file",
			p: &proc{
				extensions: []string{".libsonnet", ".jsonnet"},
			},
			specified:  []string{"testdata"},
			exclusions: []string{filepath.Join("**", "1a.libsonnet")},
			assertFn: func(t *testing.T, p *proc, err error) {
				require.NoError(t, err)
				assert.Equal(t, 2, len(p.seenFiles))
			},
		},
		{
			name: "bad args",
			p: &proc{
				extensions: []string{".libsonnet", ".jsonnet"},
			},
			specified:       []string{"not-present"},
			continueOnError: true,
			assertFn: func(t *testing.T, p *proc, err error) {
				require.Error(t, err)
				assert.Equal(t, 0, len(p.seenFiles))
				assert.IsType(t, &os.PathError{}, err)
			},
		},
		{
			name: "bad pattern",
			p: &proc{
				extensions: []string{".libsonnet", ".jsonnet"},
			},
			specified:       []string{"testdata"},
			exclusions:      []string{"[1-2.txt"},
			continueOnError: true,
			assertFn: func(t *testing.T, p *proc, err error) {
				require.Error(t, err)
				assert.Equal(t, 0, len(p.seenFiles))
				assert.Equal(t, "init options: exclude [1-2.txt: syntax error in pattern", err.Error())
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			err := Process(test.specified, Options{ContinueOnError: test.continueOnError, Exclusions: test.exclusions, VerboseWalk: true}, test.p)
			test.assertFn(t, test.p, err)
		})
	}
}

func TestFlags(t *testing.T) {
	fs := pflag.NewFlagSet("foo", pflag.ContinueOnError)
	fn := AddExclusions(fs)
	err := fs.Parse([]string{"--exclude", "foo", "-x", "bar"})
	require.NoError(t, err)
	assert.EqualValues(t, []string{"foo", "bar"}, fn())
}
