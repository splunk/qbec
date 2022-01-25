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

	"github.com/splunk/qbec/internal/cmd"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestEnvListBasic(t *testing.T) {
	s := newScaffold(t)
	defer s.reset()
	err := s.executeCommand("env", "list")
	require.NoError(t, err)
	lines := strings.Split(strings.Trim(s.stdout(), "\n"), "\n")
	a := assert.New(t)
	require.Equal(t, 4, len(lines))
	a.Equal("dev", lines[0])
	a.Equal("local", lines[1])
	a.Equal("prod", lines[2])
	a.Equal("stage", lines[3])
}

func TestEnvListYAML(t *testing.T) {
	s := newScaffold(t)
	defer s.reset()
	err := s.executeCommand("env", "list", "-o", "yaml", "--k8s:kubeconfig=kubeconfig.yaml")
	require.NoError(t, err)
	out, err := s.yamlOutput()
	require.NoError(t, err)
	assert.True(t, len(out) > 0)
}

func TestEnvListJSON(t *testing.T) {
	s := newScaffold(t)
	defer s.reset()
	err := s.executeCommand("env", "list", "-o", "json", "--k8s:kubeconfig=kubeconfig.yaml")
	require.NoError(t, err)
	var data interface{}
	err = s.jsonOutput(&data)
	require.NoError(t, err)
}

func TestEnvVarsBasic(t *testing.T) {
	s := newScaffold(t)
	defer s.reset()
	err := s.executeCommand("env", "vars", "dev", "--k8s:kubeconfig=kubeconfig.yaml")
	require.NoError(t, err)
	s.assertOutputLineMatch(regexp.MustCompile(`KUBECONFIG='kubeconfig.yaml';`))
	s.assertOutputLineMatch(regexp.MustCompile(`KUBE_CLUSTER='dev';`))
	s.assertOutputLineMatch(regexp.MustCompile(`KUBE_CONTEXT='dev'`))
	s.assertOutputLineMatch(regexp.MustCompile(`KUBE_NAMESPACE='default';`))
	s.assertOutputLineMatch(regexp.MustCompile(`export KUBECONFIG KUBE_CLUSTER KUBE_CONTEXT KUBE_NAMESPACE KUBECTL_ARGS`))
}

func TestEnvVarsYAML(t *testing.T) {
	s := newScaffold(t)
	defer s.reset()
	err := s.executeCommand("env", "vars", "dev", "-o", "yaml", "--k8s:kubeconfig=kubeconfig.yaml")
	require.NoError(t, err)
	out, err := s.yamlOutput()
	require.NoError(t, err)
	assert.True(t, len(out) > 0)
}

func TestEnvVarsJSON(t *testing.T) {
	s := newScaffold(t)
	defer s.reset()
	err := s.executeCommand("env", "vars", "dev", "-o", "json", "--k8s:kubeconfig=kubeconfig.yaml")
	require.NoError(t, err)
	var data interface{}
	err = s.jsonOutput(&data)
	require.NoError(t, err)
}

func TestEnvPropsYAML(t *testing.T) {
	s := newScaffold(t)
	defer s.reset()
	err := s.executeCommand("env", "props", "dev")
	require.NoError(t, err)
	out, err := s.yamlOutput()
	require.NoError(t, err)
	assert.True(t, len(out) > 0)
}

func TestEnvPropsJSON(t *testing.T) {
	s := newScaffold(t)
	defer s.reset()
	err := s.executeCommand("env", "props", "dev", "-o", "json")
	require.NoError(t, err)
	var data interface{}
	err = s.jsonOutput(&data)
	require.NoError(t, err)
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
				a.True(cmd.IsUsageError(err))
				a.Equal("extra arguments specified", err.Error())
			},
		},
		{
			name: "list bad format",
			args: []string{"env", "list", "-o", "table"},
			asserter: func(s *scaffold, err error) {
				a := assert.New(s.t)
				a.True(cmd.IsUsageError(err))
				a.Equal(`listEnvironments: unsupported format "table"`, err.Error())
			},
		},
		{
			name: "vars no env",
			args: []string{"env", "vars", "--k8s:kubeconfig=kubeconfig.yaml"},
			asserter: func(s *scaffold, err error) {
				a := assert.New(s.t)
				a.True(cmd.IsUsageError(err))
				a.Equal(`exactly one environment required: []`, err.Error())
			},
		},
		{
			name: "vars two envs",
			args: []string{"env", "vars", "dev", "prod", "--k8s:kubeconfig=kubeconfig.yaml"},
			asserter: func(s *scaffold, err error) {
				a := assert.New(s.t)
				a.True(cmd.IsUsageError(err))
				a.Equal(`exactly one environment required: [dev prod]`, err.Error())
			},
		},
		{
			name: "vars bad env",
			args: []string{"env", "vars", "foo", "--k8s:kubeconfig=kubeconfig.yaml"},
			asserter: func(s *scaffold, err error) {
				a := assert.New(s.t)
				a.False(cmd.IsUsageError(err))
				a.Equal(`invalid environment: "foo"`, err.Error())
			},
		},
		{
			name: "vars bad format",
			args: []string{"env", "vars", "-o", "table", "dev", "--k8s:kubeconfig=kubeconfig.yaml"},
			asserter: func(s *scaffold, err error) {
				a := assert.New(s.t)
				a.True(cmd.IsUsageError(err))
				a.Equal(`environmentVars: unsupported format "table"`, err.Error())
			},
		},
		{
			name: "props no env",
			args: []string{"env", "props"},
			asserter: func(s *scaffold, err error) {
				a := assert.New(s.t)
				a.True(cmd.IsUsageError(err))
				a.Equal(`exactly one environment required, but provided: []`, err.Error())
			},
		},
		{
			name: "props two envs",
			args: []string{"env", "props", "dev", "prod"},
			asserter: func(s *scaffold, err error) {
				a := assert.New(s.t)
				a.True(cmd.IsUsageError(err))
				a.Equal(`exactly one environment required, but provided: [dev prod]`, err.Error())
			},
		},
		{
			name: "props bad env",
			args: []string{"env", "props", "foo"},
			asserter: func(s *scaffold, err error) {
				a := assert.New(s.t)
				a.False(cmd.IsUsageError(err))
				a.Equal(`invalid environment: "foo"`, err.Error())
			},
		},
		{
			name: "props bad format",
			args: []string{"env", "props", "-o", "table", "dev"},
			asserter: func(s *scaffold, err error) {
				a := assert.New(s.t)
				a.True(cmd.IsUsageError(err))
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
