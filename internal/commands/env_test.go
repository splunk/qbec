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
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestEnvListBasic(t *testing.T) {
	s := newScaffold(t)
	defer s.reset()
	err := s.executeCommand("env", "list")
	require.Nil(t, err)
	lines := strings.Split(strings.Trim(s.stdout(), "\n"), "\n")
	a := assert.New(t)
	require.Equal(t, 2, len(lines))
	a.Equal("dev", lines[0])
	a.Equal("prod", lines[1])
}

func TestEnvListYAML(t *testing.T) {
	s := newScaffold(t)
	defer s.reset()
	err := s.executeCommand("env", "list", "-o", "yaml")
	require.Nil(t, err)
	out, err := s.yamlOutput()
	require.Nil(t, err)
	assert.True(t, len(out) > 0)
}

func TestEnvListJSON(t *testing.T) {
	s := newScaffold(t)
	defer s.reset()
	err := s.executeCommand("env", "list", "-o", "json")
	require.Nil(t, err)
	var data interface{}
	err = s.jsonOutput(&data)
	require.Nil(t, err)
}

func TestEnvVarsBasic(t *testing.T) {
	s := newScaffold(t)
	defer s.reset()
	err := s.executeCommand("env", "vars", "dev")
	require.Nil(t, err)
	s.assertOutputLineMatch(regexp.MustCompile(`KUBECONFIG='kube.config';`))
	s.assertOutputLineMatch(regexp.MustCompile(`KUBE_CLUSTER='dev.server.com';`))
	s.assertOutputLineMatch(regexp.MustCompile(`KUBE_CONTEXT='foo'`))
	s.assertOutputLineMatch(regexp.MustCompile(`KUBE_NAMESPACE='my-ns';`))
	s.assertOutputLineMatch(regexp.MustCompile(`export KUBECONFIG KUBE_CLUSTER KUBE_CONTEXT KUBE_NAMESPACE KUBECTL_ARGS`))
}

func TestEnvVarsYAML(t *testing.T) {
	s := newScaffold(t)
	defer s.reset()
	err := s.executeCommand("env", "vars", "dev", "-o", "yaml")
	require.Nil(t, err)
	out, err := s.yamlOutput()
	require.Nil(t, err)
	assert.True(t, len(out) > 0)
}

func TestEnvVarsJSON(t *testing.T) {
	s := newScaffold(t)
	defer s.reset()
	err := s.executeCommand("env", "vars", "dev", "-o", "json")
	require.Nil(t, err)
	var data interface{}
	err = s.jsonOutput(&data)
	require.Nil(t, err)
}

func TestEnvNegative(t *testing.T) {
	tests := []struct {
		name     string
		args     []string
		asserter func(s *scaffold, err error)
	}{
		{
			name: "list with env",
			args: []string{"env", "list", "dev"},
			asserter: func(s *scaffold, err error) {
				a := assert.New(s.t)
				a.True(isUsageError(err))
				a.Equal("extra arguments specified", err.Error())
			},
		},
		{
			name: "list bad format",
			args: []string{"env", "list", "-o", "table"},
			asserter: func(s *scaffold, err error) {
				a := assert.New(s.t)
				a.True(isUsageError(err))
				a.Equal(`listEnvironments: unsupported format "table"`, err.Error())
			},
		},
		{
			name: "vars no env",
			args: []string{"env", "vars"},
			asserter: func(s *scaffold, err error) {
				a := assert.New(s.t)
				a.True(isUsageError(err))
				a.Equal(`exactly one environment required`, err.Error())
			},
		},
		{
			name: "vars two envs",
			args: []string{"env", "vars", "dev", "prod"},
			asserter: func(s *scaffold, err error) {
				a := assert.New(s.t)
				a.True(isUsageError(err))
				a.Equal(`exactly one environment required`, err.Error())
			},
		},
		{
			name: "vars bad env",
			args: []string{"env", "vars", "foo"},
			asserter: func(s *scaffold, err error) {
				a := assert.New(s.t)
				a.False(isUsageError(err))
				a.Equal(`invalid environment: "foo"`, err.Error())
			},
		},
		{
			name: "vars bad format",
			args: []string{"env", "vars", "-o", "table", "dev"},
			asserter: func(s *scaffold, err error) {
				a := assert.New(s.t)
				a.True(isUsageError(err))
				a.Equal(`environmentVars: unsupported format "table"`, err.Error())
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
