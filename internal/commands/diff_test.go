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
	"fmt"
	"regexp"
	"testing"

	"github.com/splunk/qbec/internal/cmd"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDiffBasicNoDiffs(t *testing.T) {
	s := newScaffold(t)
	defer s.reset()
	d := &dg{cmValue: "bar"}
	s.client.getFunc = d.get
	err := s.executeCommand("diff", "dev", "-k", "configmaps", "--ignore-all-annotations", "--ignore-all-labels", "--show-deletes=false")
	require.NoError(t, err)
}

func testDiffBasic(t *testing.T, errorExit bool) {
	s := newScaffold(t)
	defer s.reset()
	d := &dg{cmValue: "baz", secretValue: "baz"}
	s.client.getFunc = d.get
	s.client.listFunc = stdLister
	err := s.executeCommand("diff", "dev", fmt.Sprintf("--error-exit=%v", errorExit))

	a := assert.New(t)
	if errorExit {
		require.Error(t, err)
		a.True(regexp.MustCompile(`\d+ object\(s\) different`).MatchString(err.Error()))
	} else {
		require.NoError(t, err)
	}

	stats := s.outputStats()
	a.EqualValues([]interface{}{"ConfigMap:bar-system:svc2-cm", "Secret:bar-system:svc2-secret"}, stats["changes"])
	a.EqualValues([]interface{}{"Deployment:bar-system:svc2-previous-deploy"}, stats["deletions"])
	adds, ok := stats["additions"].([]interface{})
	require.True(t, ok)
	a.Contains(adds, "Job::tj-<xxxxx>")
	secretValue := base64.StdEncoding.EncodeToString([]byte("baz"))
	redactedValue := base64.RawStdEncoding.EncodeToString([]byte("redacted."))
	a.Contains(s.stdout(), redactedValue)
	a.Contains(s.stdout(), "qbec.io/component: service2")
	a.NotContains(s.stdout(), secretValue)
}

func TestDiffBasic(t *testing.T) {
	testDiffBasic(t, true)
}

func TestDiffBasicNoErrorExit(t *testing.T) {
	testDiffBasic(t, false)
}

func TestDiffGetFail(t *testing.T) {
	s := newScaffold(t)
	defer s.reset()
	err := s.executeCommand("diff", "dev", "--parallel=1")
	require.NotNil(t, err)
	a := assert.New(t)
	a.Contains(err.Error(), "not implemented")
}

func TestDiffListFail(t *testing.T) {
	s := newScaffold(t)
	defer s.reset()
	d := &dg{cmValue: "baz", secretValue: "baz"}
	s.client.getFunc = d.get
	err := s.executeCommand("diff", "dev")
	require.NotNil(t, err)
	a := assert.New(t)
	a.Contains(err.Error(), "not implemented")
}

func TestDiffBasicNoLabels(t *testing.T) {
	s := newScaffold(t)
	defer s.reset()
	d := &dg{cmValue: "baz", secretValue: "baz"}
	s.client.getFunc = d.get
	err := s.executeCommand("diff", "dev", "--ignore-all-labels", "-S", "--show-deletes=false", "--error-exit=true")
	require.NotNil(t, err)
	a := assert.New(t)
	secretValue := base64.StdEncoding.EncodeToString([]byte("baz"))
	redactedValue := base64.RawStdEncoding.EncodeToString([]byte("redacted."))
	a.NotContains(s.stdout(), redactedValue)
	a.Contains(s.stdout(), secretValue)
	a.NotContains(s.stdout(), "qbec.io/environment")
}

func TestDiffBasicNoSpecificLabels(t *testing.T) {
	s := newScaffold(t)
	defer s.reset()
	d := &dg{cmValue: "baz", secretValue: "baz"}
	s.client.getFunc = d.get
	err := s.executeCommand("diff", "dev", "--ignore-label", "qbec.io/environment", "--show-deletes=false")
	require.NoError(t, err)
	a := assert.New(t)
	a.NotContains(s.stdout(), "qbec.io/environment:")
	a.Contains(s.stdout(), "qbec.io/application:")
}

func TestDiffBasicNoSpecificAnnotation(t *testing.T) {
	s := newScaffold(t)
	defer s.reset()
	d := &dg{cmValue: "baz", secretValue: "baz"}
	s.client.getFunc = d.get
	err := s.executeCommand("diff", "dev", "--ignore-annotation", "ann/foo", "--show-deletes=false")
	require.NoError(t, err)
	a := assert.New(t)
	a.NotContains(s.stdout(), "ann/foo")
	a.Contains(s.stdout(), "ann/bar")
}

func TestDiffBasicNoAnnotations(t *testing.T) {
	s := newScaffold(t)
	defer s.reset()
	d := &dg{cmValue: "baz", secretValue: "baz"}
	s.client.getFunc = d.get
	err := s.executeCommand("diff", "dev", "--ignore-all-annotations", "--show-deletes=false")
	require.NoError(t, err)
	a := assert.New(t)
	a.NotContains(s.stdout(), "ann/foo")
	a.NotContains(s.stdout(), "ann/bar")
}

func TestDiffNegative(t *testing.T) {
	tests := []struct {
		name     string
		args     []string
		asserter func(s *scaffold, err error)
	}{
		{
			name: "no env",
			args: []string{"diff"},
			asserter: func(s *scaffold, err error) {
				a := assert.New(s.t)
				a.True(cmd.IsUsageError(err))
				a.Equal("exactly one environment required, but provided: []", err.Error())
			},
		},
		{
			name: "2 envs",
			args: []string{"diff", "dev", "prod"},
			asserter: func(s *scaffold, err error) {
				a := assert.New(s.t)
				a.True(cmd.IsUsageError(err))
				a.Equal("exactly one environment required, but provided: [\"dev\" \"prod\"]", err.Error())
			},
		},
		{
			name: "bad env",
			args: []string{"diff", "foo"},
			asserter: func(s *scaffold, err error) {
				a := assert.New(s.t)
				a.False(cmd.IsUsageError(err))
				a.Equal("invalid environment \"foo\"", err.Error())
			},
		},
		{
      name: "empty string env",
      args: []string{"apply", ""},
      asserter: func(s *scaffold, err error) {
        a := assert.New(s.t)
        a.False(cmd.IsUsageError(err))
        a.Equal("invalid environment \"\"", err.Error())
      },
    },
		{
			name: "baseline env",
			args: []string{"diff", "_"},
			asserter: func(s *scaffold, err error) {
				a := assert.New(s.t)
				a.True(cmd.IsUsageError(err))
				a.Equal("cannot diff baseline environment, use a real environment", err.Error())
			},
		},
		{
			name: "c and C",
			args: []string{"diff", "dev", "-c", "cluster-objects", "-C", "service2"},
			asserter: func(s *scaffold, err error) {
				a := assert.New(s.t)
				a.True(cmd.IsUsageError(err))
				a.Equal(`cannot include as well as exclude components, specify one or the other`, err.Error())
			},
		},
		{
			name: "k and K",
			args: []string{"diff", "dev", "-k", "namespace", "-K", "secret"},
			asserter: func(s *scaffold, err error) {
				a := assert.New(s.t)
				a.True(cmd.IsUsageError(err))
				a.Equal(`cannot include as well as exclude kinds, specify one or the other`, err.Error())
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
