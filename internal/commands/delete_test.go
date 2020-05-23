package commands

import (
	"testing"

	"github.com/splunk/qbec/internal/model"
	"github.com/splunk/qbec/internal/remote"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

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

func TestDeleteRemote(t *testing.T) {
	s := newScaffold(t)
	defer s.reset()
	d := &dg{cmValue: "baz", secretValue: "baz"}
	s.client.getFunc = d.get
	s.client.listFunc = stdLister
	s.client.deleteFunc = func(obj model.K8sMeta, opts remote.DeleteOptions) (*remote.SyncResult, error) {
		return &remote.SyncResult{Type: remote.SyncDeleted}, nil
	}
	err := s.executeCommand("delete", "dev")
	require.NoError(t, err)
	stats := s.outputStats()
	a := assert.New(t)
	a.EqualValues([]interface{}{"Deployment:bar-system:svc2-previous-deploy", "Deployment:bar-system:svc2-deploy"}, stats["deleted"])
}

func TestDeleteRemoteComponentFilter(t *testing.T) {
	s := newScaffold(t)
	defer s.reset()
	d := &dg{cmValue: "baz", secretValue: "baz"}
	s.client.getFunc = d.get
	s.client.listFunc = stdLister
	s.client.deleteFunc = func(obj model.K8sMeta, opts remote.DeleteOptions) (*remote.SyncResult, error) {
		return &remote.SyncResult{Type: remote.SyncDeleted}, nil
	}
	err := s.executeCommand("delete", "dev", "-c", "service2")
	require.NoError(t, err)
	stats := s.outputStats()
	a := assert.New(t)
	a.EqualValues([]interface{}{"Deployment:bar-system:svc2-previous-deploy"}, stats["deleted"])
}

func TestDeleteLocal(t *testing.T) {
	s := newScaffold(t)
	defer s.reset()
	d := &dg{cmValue: "baz", secretValue: "baz"}
	s.client.getFunc = d.get
	s.client.deleteFunc = func(obj model.K8sMeta, opts remote.DeleteOptions) (*remote.SyncResult, error) {
		return &remote.SyncResult{Type: remote.SyncDeleted}, nil
	}
	err := s.executeCommand("delete", "dev", "--local", "-C", "cluster-objects")
	require.NoError(t, err)
	stats := s.outputStats()
	a := assert.New(t)
	a.EqualValues([]interface{}{"Deployment:bar-system:svc2-deploy", "Secret:bar-system:svc2-secret", "ConfigMap:bar-system:svc2-cm"}, stats["deleted"])
}

func TestDeleteNegative(t *testing.T) {
	tests := []struct {
		name     string
		args     []string
		asserter func(s *scaffold, err error)
	}{
		{
			name: "no env",
			args: []string{"delete"},
			asserter: func(s *scaffold, err error) {
				a := assert.New(s.t)
				a.True(isUsageError(err))
				a.Equal("exactly one environment required", err.Error())
			},
		},
		{
			name: "2 envs",
			args: []string{"delete", "dev", "prod"},
			asserter: func(s *scaffold, err error) {
				a := assert.New(s.t)
				a.True(isUsageError(err))
				a.Equal("exactly one environment required", err.Error())
			},
		},
		{
			name: "bad env",
			args: []string{"delete", "foo"},
			asserter: func(s *scaffold, err error) {
				a := assert.New(s.t)
				a.False(isUsageError(err))
				a.Equal("invalid environment \"foo\"", err.Error())
			},
		},
		{
			name: "baseline env",
			args: []string{"delete", "_"},
			asserter: func(s *scaffold, err error) {
				a := assert.New(s.t)
				a.True(isUsageError(err))
				a.Equal("cannot delete baseline environment, use a real environment", err.Error())
			},
		},
		{
			name: "c and C",
			args: []string{"delete", "dev", "-c", "cluster-objects", "-C", "service2"},
			asserter: func(s *scaffold, err error) {
				a := assert.New(s.t)
				a.True(isUsageError(err))
				a.Equal(`cannot include as well as exclude components, specify one or the other`, err.Error())
			},
		},
		{
			name: "k and K",
			args: []string{"delete", "dev", "-k", "namespace", "-K", "secret"},
			asserter: func(s *scaffold, err error) {
				a := assert.New(s.t)
				a.True(isUsageError(err))
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
