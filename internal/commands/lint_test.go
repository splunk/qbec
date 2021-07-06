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

package commands

import (
	"runtime"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLintCompressLines(t *testing.T) {
	tests := []struct {
		name   string
		input  string
		output []string
	}{
		{
			name:   "no empty",
			input:  "a\nb\nc\n",
			output: []string{"a", "b", "c", ""},
		},
		{
			name:   "emptiness",
			input:  "\n\na\n\n\n\nb\nc\n\n\n\n",
			output: []string{"", "a", "", "b", "c", ""},
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			actual := compressLines(test.input)
			assert.EqualValues(t, test.output, actual)
		})
	}
}

func TestLintBasic(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("import of a file called _.libsonnet doesn't seem to work in the linter in windows")
	}
	s := newScaffold(t)
	defer s.reset()
	err := s.executeCommand("alpha", "lint")
	require.NoError(t, err)
}

func TestLintWithApp(t *testing.T) {
	s := newCustomScaffold(t, "testdata/projects/lint-app")
	defer s.reset()
	err := s.executeCommand("alpha", "lint", "components", "lib")
	require.Error(t, err)
	assert.Equal(t, "2 errors encountered", err.Error())
}

func TestLintWithAppAndExcludes(t *testing.T) {
	s := newCustomScaffold(t, "testdata/projects/lint-app")
	defer s.reset()
	err := s.executeCommand("alpha", "lint", "components", "lib", "-x", "lib/foo.libsonnet")
	require.Error(t, err)
	assert.Equal(t, "1 error encountered", err.Error())
}

func TestLintWithoutApp(t *testing.T) {
	s := newCustomScaffold(t, "testdata/projects/lint-app")
	defer s.reset()
	err := s.executeCommand("alpha", "lint", "lib/bar.libsonnet", "--app=false")
	require.Error(t, err)
	assert.Equal(t, "1 error encountered", err.Error())
}
