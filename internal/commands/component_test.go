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

func TestComponentListBasic(t *testing.T) {
	s := newScaffold(t)
	defer s.reset()
	err := s.executeCommand("component", "list", "dev")
	require.Nil(t, err)
	lines := strings.Split(strings.Trim(s.stdout(), "\n"), "\n")
	a := assert.New(t)
	a.Equal(4, len(lines))
	s.assertOutputLineMatch(regexp.MustCompile(`COMPONENT\s+FILE`))
	s.assertOutputLineMatch(regexp.MustCompile(`cluster-objects\s+components/cluster-objects.yaml`))
	s.assertOutputLineMatch(regexp.MustCompile(`service2\s+components/service2.jsonnet`))
	s.assertOutputLineMatch(regexp.MustCompile(`test-job\s+components/test-job.yaml`))
}

func TestComponentListYAML(t *testing.T) {
	s := newScaffold(t)
	defer s.reset()
	err := s.executeCommand("component", "list", "dev", "-o", "yaml")
	require.Nil(t, err)
	out, err := s.yamlOutput()
	require.Nil(t, err)
	assert.True(t, len(out) > 0)
}

func TestComponentListJSON(t *testing.T) {
	s := newScaffold(t)
	defer s.reset()
	err := s.executeCommand("component", "list", "dev", "-o", "json")
	require.Nil(t, err)
	var data interface{}
	err = s.jsonOutput(&data)
	require.Nil(t, err)
}

func TestComponentListObjects(t *testing.T) {
	s := newScaffold(t)
	defer s.reset()
	err := s.executeCommand("component", "list", "dev", "-O")
	require.Nil(t, err)
	s.assertOutputLineMatch(regexp.MustCompile(`COMPONENT\s+KIND\s+NAME\s+NAMESPACE`))
	s.assertOutputLineMatch(regexp.MustCompile(`cluster-objects\s+ClusterRole\s+allow-root-psp-policy`))
	s.assertOutputLineMatch(regexp.MustCompile(`service2\s+ConfigMap\s+svc2-cm\s+bar-system`))
}

func TestComponentDiffBasic(t *testing.T) {
	s := newScaffold(t)
	defer s.reset()
	err := s.executeCommand("component", "diff", "dev")
	require.Nil(t, err)
	s.assertOutputLineMatch(regexp.MustCompile(`--- baseline`))
	s.assertOutputLineMatch(regexp.MustCompile(`\+\+\+ environment: dev`))
	s.assertOutputLineMatch(regexp.MustCompile(`\+service2\s+components/service2.jsonnet`))
}

func TestComponentDiff2Envs(t *testing.T) {
	s := newScaffold(t)
	defer s.reset()
	err := s.executeCommand("component", "diff", "dev", "prod")
	require.Nil(t, err)
	s.assertOutputLineMatch(regexp.MustCompile(`--- environment: dev`))
	s.assertOutputLineMatch(regexp.MustCompile(`\+\+\+ environment: prod`))
	s.assertOutputLineMatch(regexp.MustCompile(`\++service1\s+components/service1.jsonnet`))
}

func TestComponentDiff2EnvObjects(t *testing.T) {
	s := newScaffold(t)
	defer s.reset()
	err := s.executeCommand("component", "diff", "dev", "prod", "-O")
	require.Nil(t, err)
	s.assertOutputLineMatch(regexp.MustCompile(`--- environment: dev`))
	s.assertOutputLineMatch(regexp.MustCompile(`\+\+\+ environment: prod`))
	s.assertOutputLineMatch(regexp.MustCompile(`\+service1\s+ConfigMap\s+svc1-cm\s+foo-system`))
}

func TestComponentDiffObjects(t *testing.T) {
	s := newScaffold(t)
	defer s.reset()
	err := s.executeCommand("component", "diff", "dev", "-O")
	require.NoError(t, err)
	s.assertOutputLineMatch(regexp.MustCompile(`--- baseline`))
	s.assertOutputLineMatch(regexp.MustCompile(`\+\+\+ environment: dev`))
	s.assertOutputLineMatch(regexp.MustCompile(`\+service2\s+ConfigMap\s+svc2-cm\s+bar-system`))
	s.assertOutputLineMatch(regexp.MustCompile(`-service1\s+ConfigMap\s+svc1-cm\s+foo-system`))
}

func TestComponentNegative(t *testing.T) {
	tests := []struct {
		name     string
		args     []string
		asserter func(s *scaffold, err error)
	}{
		{
			name: "list no env",
			args: []string{"component", "list"},
			asserter: func(s *scaffold, err error) {
				a := assert.New(s.t)
				a.True(isUsageError(err))
				a.Equal("exactly one environment required", err.Error())
			},
		},
		{
			name: "list 2 envs",
			args: []string{"component", "list", "dev", "prod"},
			asserter: func(s *scaffold, err error) {
				a := assert.New(s.t)
				a.True(isUsageError(err))
				a.Equal("exactly one environment required", err.Error())
			},
		},
		{
			name: "list bad format",
			args: []string{"component", "list", "dev", "-o", "table"},
			asserter: func(s *scaffold, err error) {
				a := assert.New(s.t)
				a.True(isUsageError(err))
				a.Equal(`listComponents: unsupported format "table"`, err.Error())
			},
		},
		{
			name: "list bad env",
			args: []string{"component", "list", "foo"},
			asserter: func(s *scaffold, err error) {
				a := assert.New(s.t)
				a.False(isUsageError(err))
				a.Equal(`invalid environment "foo"`, err.Error())
			},
		},
		{
			name: "diff no env",
			args: []string{"component", "diff"},
			asserter: func(s *scaffold, err error) {
				a := assert.New(s.t)
				a.True(isUsageError(err))
				a.Equal(`one or two environments required`, err.Error())
			},
		},
		{
			name: "diff 3 envs",
			args: []string{"component", "diff", "dev", "prod", "_"},
			asserter: func(s *scaffold, err error) {
				a := assert.New(s.t)
				a.True(isUsageError(err))
				a.Equal(`one or two environments required`, err.Error())
			},
		},
		{
			name: "diff bad envs",
			args: []string{"component", "diff", "foo"},
			asserter: func(s *scaffold, err error) {
				a := assert.New(s.t)
				a.False(isUsageError(err))
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
