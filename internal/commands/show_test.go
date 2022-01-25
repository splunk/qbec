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
	"encoding/base64"
	"regexp"
	"strings"
	"testing"

	"github.com/splunk/qbec/internal/cmd"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestShowBasic(t *testing.T) {
	s := newScaffold(t)
	defer s.reset()
	err := s.executeCommand("show", "dev")
	require.NoError(t, err)
	out, err := s.yamlOutput()
	require.NoError(t, err)
	a := assert.New(t)
	a.True(len(out) > 0)
	s.assertOutputLineMatch(regexp.MustCompile(`^\s+name: svc2`))
	s.assertOutputLineMatch(regexp.MustCompile(`^\s+qbec\.io/application: example1`))
	s.assertOutputLineMatch(regexp.MustCompile(`^\s+qbec\.io/component: service2`))
	s.assertOutputLineMatch(regexp.MustCompile(`^\s+qbec\.io/environment: dev`))
	pos1 := strings.Index(s.stdout(), "name: foo-system")
	a.True(pos1 > 0)
	pos2 := strings.Index(s.stdout(), "name: 100-default")
	a.True(pos2 > 0)
	a.True(pos1 < pos2) // namespace before psp in std sort
}

func TestShowBasicClean(t *testing.T) {
	s := newScaffold(t)
	defer s.reset()
	err := s.executeCommand("show", "dev", "--clean")
	require.NoError(t, err)
	out, err := s.yamlOutput()
	require.NoError(t, err)
	a := assert.New(t)
	a.True(len(out) > 0)
	s.assertOutputLineMatch(regexp.MustCompile(`\s+name: svc2`))
	s.assertOutputLineNoMatch(regexp.MustCompile(`qbec\.io/application`))
	s.assertOutputLineNoMatch(regexp.MustCompile(`qbec\.io/component`))
	s.assertOutputLineNoMatch(regexp.MustCompile(`qbec\.io/environment`))
	pos1 := strings.Index(s.stdout(), "name: foo-system")
	a.True(pos1 > 0)
	pos2 := strings.Index(s.stdout(), "name: 100-default")
	a.True(pos2 > 0)
	a.True(pos1 < pos2) // namespace before psp in std sort
}

func TestShowApplySort(t *testing.T) {
	s := newScaffold(t)
	defer s.reset()
	err := s.executeCommand("show", "dev", "--sort-apply")
	require.NoError(t, err)
	out, err := s.yamlOutput()
	require.NoError(t, err)
	assert.True(t, len(out) > 0)
	pos1 := strings.Index(s.stdout(), "name: foo-system")
	pos2 := strings.Index(s.stdout(), "name: 100-default")
	assert.True(t, pos1 > pos2) // namespace after psp in apply sort
}

func TestShowApplySortBaseline(t *testing.T) {
	s := newScaffold(t)
	defer s.reset()
	err := s.executeCommand("show", "_", "--sort-apply")
	require.NoError(t, err)
	out, err := s.yamlOutput()
	require.NoError(t, err)
	a := assert.New(t)
	a.True(len(out) > 0)
	pos1 := strings.Index(s.stdout(), "name: foo-system")
	pos2 := strings.Index(s.stdout(), "name: 100-default")
	a.True(pos2 > pos1) // no apply sort
	a.Contains(s.stderr(), "[warn] cannot sort in apply order for baseline environment")
}

func TestShowBasicJSON(t *testing.T) {
	s := newScaffold(t)
	defer s.reset()
	err := s.executeCommand("show", "dev", "-o", "json")
	require.NoError(t, err)
	var data interface{}
	err = s.jsonOutput(&data)
	require.NoError(t, err)
	s.assertOutputLineMatch(regexp.MustCompile(`\s+"name": "svc2-cm"`))
}

func TestShowObjects(t *testing.T) {
	s := newScaffold(t)
	defer s.reset()
	err := s.executeCommand("show", "dev", "-O")
	require.NoError(t, err)
	s.assertOutputLineMatch(regexp.MustCompile(`service2\s+ConfigMap\s+svc2-cm\s+bar-system`))
	s.assertOutputLineMatch(regexp.MustCompile(`cluster-objects\s+Namespace\s+bar-system`))
}

func TestShowObjectsAsYAML(t *testing.T) {
	s := newScaffold(t)
	defer s.reset()
	err := s.executeCommand("show", "dev", "-O", "-o", "yaml")
	require.NoError(t, err)
	out, err := s.yamlOutput()
	require.NoError(t, err)
	assert.True(t, len(out) > 0)
	s.assertOutputLineMatch(regexp.MustCompile(`\s+name: svc2`))
}

func TestShowObjectsAsJSON(t *testing.T) {
	s := newScaffold(t)
	defer s.reset()
	err := s.executeCommand("show", "dev", "-O", "-o", "json")
	require.NoError(t, err)
	var data interface{}
	err = s.jsonOutput(&data)
	require.NoError(t, err)
	s.assertOutputLineMatch(regexp.MustCompile(`\s+"name": "svc2-cm"`))
}

func TestShowObjectsComponentFilter(t *testing.T) {
	s := newScaffold(t)
	defer s.reset()
	err := s.executeCommand("show", "dev", "-c", "cluster-objects")
	require.NoError(t, err)
	out, err := s.yamlOutput()
	require.NoError(t, err)
	assert.True(t, len(out) > 0)
	s.assertOutputLineMatch(regexp.MustCompile(`\s+name: bar-system`))
	s.assertOutputLineNoMatch(regexp.MustCompile(`\s+name: svc2`))
}

func TestShowObjectsComponentFilter2(t *testing.T) {
	s := newScaffold(t)
	defer s.reset()
	err := s.executeCommand("show", "dev", "-C", "cluster-objects")
	require.NoError(t, err)
	out, err := s.yamlOutput()
	require.NoError(t, err)
	assert.True(t, len(out) > 0)
	s.assertOutputLineNoMatch(regexp.MustCompile(`\s+name: bar-system`))
	s.assertOutputLineMatch(regexp.MustCompile(`\s+name: svc2`))
}

func TestShowObjectsKindFilter(t *testing.T) {
	s := newScaffold(t)
	defer s.reset()
	err := s.executeCommand("show", "dev", "-k", "secret")
	require.NoError(t, err)
	out, err := s.yamlOutput()
	require.NoError(t, err)
	assert.True(t, len(out) > 0)
	s.assertOutputLineNoMatch(regexp.MustCompile(`\s+name: bar-system`))
	s.assertOutputLineMatch(regexp.MustCompile(`\s+name: svc2-secret`))
}

func TestShowObjectsKindFilter2(t *testing.T) {
	s := newScaffold(t)
	defer s.reset()
	err := s.executeCommand("show", "dev", "-K", "secret")
	require.NoError(t, err)
	out, err := s.yamlOutput()
	require.NoError(t, err)
	assert.True(t, len(out) > 0)
	s.assertOutputLineNoMatch(regexp.MustCompile(`\s+name: svc2-secret`))
	s.assertOutputLineMatch(regexp.MustCompile(`\s+name: bar-system`))
}

func TestShowObjectsKindFilter3(t *testing.T) {
	s := newScaffold(t)
	defer s.reset()
	err := s.executeCommand("show", "dev", "-k", "garbage")
	require.NoError(t, err)
	out, err := s.yamlOutput()
	require.NoError(t, err)
	assert.True(t, len(out) == 0)
	assert.Contains(t, s.stderr(), "matches for kind filter, check for typos and abbreviations")
}

func TestShowHiddenSecrets(t *testing.T) {
	s := newScaffold(t)
	defer s.reset()
	secretValue := base64.StdEncoding.EncodeToString([]byte("bar"))
	redactedValue := base64.RawStdEncoding.EncodeToString([]byte("redacted."))
	err := s.executeCommand("show", "dev", "-k", "secret")
	require.NoError(t, err)
	s.assertOutputLineMatch(regexp.MustCompile(redactedValue))
	s.assertOutputLineNoMatch(regexp.MustCompile(secretValue))
}

func TestShowOpenSecrets(t *testing.T) {
	s := newScaffold(t)
	defer s.reset()
	secretValue := base64.StdEncoding.EncodeToString([]byte("bar"))
	redactedValue := base64.RawStdEncoding.EncodeToString([]byte("redacted."))
	err := s.executeCommand("show", "dev", "-k", "secret", "-S")
	require.NoError(t, err)
	s.assertOutputLineNoMatch(regexp.MustCompile(redactedValue))
	s.assertOutputLineMatch(regexp.MustCompile(secretValue))
}

func TestShowHiddenStringSecrets(t *testing.T) {
	s := newCustomScaffold(t, "testdata/projects/string-secrets")
	defer s.reset()
	secretValue := "foobar"
	redactedValue := base64.RawStdEncoding.EncodeToString([]byte("redacted."))
	err := s.executeCommand("show", "local")
	require.NoError(t, err)
	s.assertOutputLineMatch(regexp.MustCompile(redactedValue))
	s.assertOutputLineNoMatch(regexp.MustCompile(secretValue))
}

func TestShowOpenStringSecrets(t *testing.T) {
	s := newCustomScaffold(t, "testdata/projects/string-secrets")
	defer s.reset()
	secretValue := "foobar"
	redactedValue := base64.RawStdEncoding.EncodeToString([]byte("redacted."))
	err := s.executeCommand("show", "local", "-S")
	require.NoError(t, err)
	s.assertOutputLineNoMatch(regexp.MustCompile(redactedValue))
	s.assertOutputLineMatch(regexp.MustCompile(secretValue))
}

func TestShowNegative(t *testing.T) {
	tests := []struct {
		name     string
		args     []string
		asserter func(s *scaffold, err error)
		dir      string
	}{
		{
			name: "no env",
			args: []string{"show"},
			asserter: func(s *scaffold, err error) {
				a := assert.New(s.t)
				a.True(cmd.IsUsageError(err))
				a.Equal("exactly one environment required, but provided: []", err.Error())
			},
		},
		{
			name: "2 envs",
			args: []string{"show", "dev", "prod"},
			asserter: func(s *scaffold, err error) {
				a := assert.New(s.t)
				a.True(cmd.IsUsageError(err))
				a.Equal("exactly one environment required, but provided: [dev prod]", err.Error())
			},
		},
		{
			name: "bad env",
			args: []string{"show", "foo"},
			asserter: func(s *scaffold, err error) {
				a := assert.New(s.t)
				a.False(cmd.IsUsageError(err))
				a.Equal("invalid environment \"foo\"", err.Error())
			},
		},
		{
			name: "bad format",
			args: []string{"show", "dev", "-o", "table"},
			asserter: func(s *scaffold, err error) {
				a := assert.New(s.t)
				a.True(cmd.IsUsageError(err))
				a.Equal(`invalid output format: "table"`, err.Error())
			},
		},
		{
			name: "c and C",
			args: []string{"show", "dev", "-c", "cluster-objects", "-C", "service2"},
			asserter: func(s *scaffold, err error) {
				a := assert.New(s.t)
				a.True(cmd.IsUsageError(err))
				a.Equal(`cannot include as well as exclude components, specify one or the other`, err.Error())
			},
		},
		{
			name: "k and K",
			args: []string{"show", "dev", "-k", "namespace", "-K", "secret"},
			asserter: func(s *scaffold, err error) {
				a := assert.New(s.t)
				a.True(cmd.IsUsageError(err))
				a.Equal(`cannot include as well as exclude kinds, specify one or the other`, err.Error())
			},
		},
		{
			name: "duplicate objects",
			args: []string{"show", "dev"},
			asserter: func(s *scaffold, err error) {
				a := assert.New(s.t)
				a.True(cmd.IsRuntimeError(err))
				a.Equal(`duplicate objects ConfigMap cm1 (component: x) and ConfigMap cm1 (component: y)`, err.Error())
			},
			dir: "testdata/dups",
		},
		{
			name: "bad force namespace",
			args: []string{"show", "dev", "--force:k8s-namespace=__current__"},
			asserter: func(s *scaffold, err error) {
				a := assert.New(s.t)
				a.True(cmd.IsUsageError(err))
				a.Equal(`current namespace can only be forced when the context is also forced to current`, err.Error())
			},
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			s := newCustomScaffold(t, test.dir)
			defer s.reset()
			err := s.executeCommand(test.args...)
			require.Error(t, err)
			test.asserter(s, err)
		})
	}
}
