/*
   Copyright 2019 Splunk Inc.

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
	"regexp"
	"testing"

	"github.com/splunk/qbec/internal/cmd"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParamListBasic(t *testing.T) {
	s := newScaffold(t)
	defer s.reset()
	orig := maxDisplayValueLength
	defer func() { maxDisplayValueLength = orig }()
	maxDisplayValueLength = 10
	err := s.executeCommand("param", "list", "dev")
	require.NoError(t, err)
	s.assertOutputLineMatch(regexp.MustCompile(`COMPONENT\s+NAME\s+VALUE`))
	s.assertOutputLineMatch(regexp.MustCompile(`service2\s+cpu\s+"50m"`))
	s.assertOutputLineMatch(regexp.MustCompile(`service2\s+memory\s+"8Gi"`))
	s.assertOutputLineMatch(regexp.MustCompile(`service2\s+longVal\s+"a real\.\.\.`))
}

func TestParamListFilter(t *testing.T) {
	s := newScaffold(t)
	defer s.reset()
	err := s.executeCommand("param", "list", "dev", "-C", "service2")
	require.NoError(t, err)
	s.assertOutputLineMatch(regexp.MustCompile(`COMPONENT\s+NAME\s+VALUE`))
	s.assertOutputLineNoMatch(regexp.MustCompile(`service2`))
}

func TestParamListYAML(t *testing.T) {
	s := newScaffold(t)
	defer s.reset()
	err := s.executeCommand("param", "list", "dev", "-o", "yaml")
	require.NoError(t, err)
	out, err := s.yamlOutput()
	require.NoError(t, err)
	assert.True(t, len(out) > 0)
}

func TestParamListJSON(t *testing.T) {
	s := newScaffold(t)
	defer s.reset()
	err := s.executeCommand("param", "list", "dev", "-o", "json")
	require.NoError(t, err)
	var data interface{}
	err = s.jsonOutput(&data)
	require.NoError(t, err)
}

func TestParamDiffBasic(t *testing.T) {
	s := newScaffold(t)
	defer s.reset()
	err := s.executeCommand("param", "diff", "dev")
	require.NoError(t, err)
	s.assertOutputLineMatch(regexp.MustCompile(`--- baseline`))
	s.assertOutputLineMatch(regexp.MustCompile(`\+\+\+ environment: dev`))
	s.assertOutputLineMatch(regexp.MustCompile(`-service2\s+cpu\s+"100m"`))
	s.assertOutputLineMatch(regexp.MustCompile(`\+service2\s+cpu\s+"50m"`))
}

func TestParamDiff2Envs(t *testing.T) {
	s := newScaffold(t)
	defer s.reset()
	err := s.executeCommand("params", "diff", "dev", "prod")
	require.NoError(t, err)
	s.assertOutputLineMatch(regexp.MustCompile(`--- environment: dev`))
	s.assertOutputLineMatch(regexp.MustCompile(`\+\+\+ environment: prod`))
	s.assertOutputLineMatch(regexp.MustCompile(`-service1\s+cpu\s+"10m"`))
	s.assertOutputLineMatch(regexp.MustCompile(`\+service1\s+cpu\s+"1"`))
}

func TestParamNegative(t *testing.T) {
	tests := []struct {
		name     string
		args     []string
		asserter func(s *scaffold, err error)
	}{
		{
			name: "list no env",
			args: []string{"param", "list"},
			asserter: func(s *scaffold, err error) {
				a := assert.New(s.t)
				a.True(cmd.IsUsageError(err))
				a.Equal("exactly one environment required, but provided: []", err.Error())
			},
		},
		{
			name: "list 2 envs",
			args: []string{"param", "list", "dev", "prod"},
			asserter: func(s *scaffold, err error) {
				a := assert.New(s.t)
				a.True(cmd.IsUsageError(err))
				a.Equal("exactly one environment required, but provided: [dev prod]", err.Error())
			},
		},
		{
			name: "list bad format",
			args: []string{"param", "list", "dev", "-o", "table"},
			asserter: func(s *scaffold, err error) {
				a := assert.New(s.t)
				a.True(cmd.IsUsageError(err))
				a.Equal(`listParams: unsupported format "table"`, err.Error())
			},
		},
		{
			name: "list bad env",
			args: []string{"param", "list", "foo"},
			asserter: func(s *scaffold, err error) {
				a := assert.New(s.t)
				a.False(cmd.IsUsageError(err))
				a.Equal(`invalid environment "foo"`, err.Error())
			},
		},
		{
			name: "diff no env",
			args: []string{"param", "diff"},
			asserter: func(s *scaffold, err error) {
				a := assert.New(s.t)
				a.True(cmd.IsUsageError(err))
				a.Equal(`one or two environments required`, err.Error())
			},
		},
		{
			name: "diff 3 envs",
			args: []string{"param", "diff", "dev", "prod", "_"},
			asserter: func(s *scaffold, err error) {
				a := assert.New(s.t)
				a.True(cmd.IsUsageError(err))
				a.Equal(`one or two environments required`, err.Error())
			},
		},
		{
			name: "diff bad envs",
			args: []string{"param", "diff", "foo"},
			asserter: func(s *scaffold, err error) {
				a := assert.New(s.t)
				a.False(cmd.IsUsageError(err))
				a.Equal(`invalid environment "foo"`, err.Error())
			},
		},
		{
			name: "diff bad envs2",
			args: []string{"param", "diff", "dev", "foo"},
			asserter: func(s *scaffold, err error) {
				a := assert.New(s.t)
				a.False(cmd.IsUsageError(err))
				a.Equal(`invalid environment "foo"`, err.Error())
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			s := newScaffold(t)
			defer s.reset()
			err := s.executeCommand(test.args...)
			require.NotNil(t, err)
			test.asserter(s, err)
		})
	}
}
