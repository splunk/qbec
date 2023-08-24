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
	"context"
	"fmt"
	"regexp"
	"testing"

	"github.com/splunk/qbec/internal/cmd"
	"github.com/splunk/qbec/internal/model"
	"github.com/splunk/qbec/internal/remote"
	"github.com/splunk/qbec/internal/rollout"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

func TestNsWrap(t *testing.T) {
	obj := model.NewK8sObject(map[string]interface{}{
		"kind":       "foo",
		"apiversion": "apps/v1",
		"metadata": map[string]interface{}{
			"namespace": "foo",
			"name":      "foo",
		},
	})
	w := nsWrap{K8sMeta: obj, ns: "bar"}
	a := assert.New(t)
	a.Equal("foo", w.GetNamespace())
	a.Equal("foo", w.GetName())
	obj = model.NewK8sObject(map[string]interface{}{
		"kind":       "foo",
		"apiversion": "apps/v1",
		"metadata": map[string]interface{}{
			"name": "foo",
		},
	})
	w = nsWrap{K8sMeta: obj, ns: "bar"}
	a.Equal("bar", w.GetNamespace())
	a.Equal("foo", w.GetName())
}

func TestApplyBasic(t *testing.T) {
	s := newScaffold(t)
	defer s.reset()
	origWait := applyWaitFn
	applyWaitFn = func(objects []model.K8sMeta, wp rollout.WatchProvider, opts rollout.WaitOptions) (finalErr error) {
		return nil
	}
	defer func() { applyWaitFn = origWait }()
	first := true
	var captured remote.SyncOptions
	s.client.syncFunc = func(ctx context.Context, obj model.K8sLocalObject, opts remote.SyncOptions) (*remote.SyncResult, error) {
		if first {
			first = false
			captured = opts
		}
		switch {
		case obj.GetName() == "svc2-cm":
			return &remote.SyncResult{Type: remote.SyncUpdated, Details: "data updated"}, nil
		case obj.GetName() == "svc2-secret":
			return &remote.SyncResult{Type: remote.SyncCreated, Details: "some yaml"}, nil
		case obj.GetName() == "svc2-deploy":
			return &remote.SyncResult{Type: remote.SyncObjectsIdentical, Details: "sync skipped"}, nil
		case obj.GetName() == "":
			return &remote.SyncResult{Type: remote.SyncCreated, GeneratedName: obj.GetGenerateName() + "1234", Details: "created"}, nil
		default:
			return &remote.SyncResult{Type: remote.SyncObjectsIdentical, Details: "sync skipped"}, nil
		}
	}
	s.client.listFunc = stdLister
	s.client.deleteFunc = func(ctx context.Context, obj model.K8sMeta, opts remote.DeleteOptions) (*remote.SyncResult, error) {
		return &remote.SyncResult{Type: remote.SyncDeleted}, nil
	}
	err := s.executeCommand("apply", "dev", "--wait")
	require.NoError(t, err)
	stats := s.outputStats()
	a := assert.New(t)
	a.False(captured.DryRun)
	a.False(captured.DisableCreate)
	a.False(captured.ShowSecrets)
	a.True(stats["same"].(float64) > 0)
	a.EqualValues(8, stats["same"])
	a.EqualValues([]interface{}{"Secret:bar-system:svc2-secret", "Job::tj-1234"}, stats["created"])
	a.EqualValues([]interface{}{"ConfigMap:bar-system:svc2-cm"}, stats["updated"])
	a.EqualValues([]interface{}{"Deployment:bar-system:svc2-previous-deploy"}, stats["deleted"])
	s.assertErrorLineMatch(regexp.MustCompile(`update ConfigMap:bar-system:svc2-cm`))
}

func TestApplyFlags(t *testing.T) {
	s := newScaffold(t)
	defer s.reset()
	first := true
	var captured remote.SyncOptions
	s.client.syncFunc = func(ctx context.Context, obj model.K8sLocalObject, opts remote.SyncOptions) (*remote.SyncResult, error) {
		if first {
			first = false
			captured = opts
		}
		switch {
		case obj.GetName() == "svc2-cm":
			return &remote.SyncResult{Type: remote.SyncUpdated, Details: "data updated"}, nil
		case obj.GetName() == "svc2-secret":
			return &remote.SyncResult{Type: remote.SyncSkip, Details: "create skipped"}, nil
		default:
			return &remote.SyncResult{Type: remote.SyncObjectsIdentical, Details: "sync skipped"}, nil
		}
	}
	err := s.executeCommand("apply", "dev", "-S", "-n", "--skip-create", "--gc=false", "--wait-all=false")
	require.NoError(t, err)
	stats := s.outputStats()
	a := assert.New(t)
	a.True(captured.ShowSecrets)
	a.True(captured.DryRun)
	a.True(captured.DisableCreate)
	a.EqualValues(nil, stats["created"])
	a.EqualValues([]interface{}{"Secret:bar-system:svc2-secret"}, stats["skipped"])
	a.EqualValues([]interface{}{"ConfigMap:bar-system:svc2-cm"}, stats["updated"])
	s.assertErrorLineMatch(regexp.MustCompile(`\[dry-run\] update ConfigMap:bar-system:svc2-cm`))
	s.assertErrorLineMatch(regexp.MustCompile(`\*\* dry-run mode, nothing was actually changed \*\*`))
}

func TestApplyNamespaceClusterFilters(t *testing.T) {
	tests := []struct {
		name       string
		filterArgs []string
		assertFn   func(t *testing.T, s *scaffold, err error)
	}{
		{
			name:       "first-only",
			filterArgs: []string{"-p", "first"},
			assertFn: func(t *testing.T, s *scaffold, err error) {
				require.NoError(t, err)
				stats := s.outputStats()
				assert.EqualValues(t, []interface{}{"ConfigMap:first:first-cm"}, stats["created"])
			},
		},
		{
			name:       "second-only",
			filterArgs: []string{"-p", "second"},
			assertFn: func(t *testing.T, s *scaffold, err error) {
				require.NoError(t, err)
				stats := s.outputStats()
				assert.EqualValues(t, []interface{}{"ConfigMap::second-cm", "Secret:second:second-secret"}, stats["created"])
			},
		},
		{
			name:       "exclude-second-add-cluster",
			filterArgs: []string{"-P", "second", "--include-cluster-objects"},
			assertFn: func(t *testing.T, s *scaffold, err error) {
				require.NoError(t, err)
				stats := s.outputStats()
				assert.EqualValues(t, []interface{}{"Namespace::first", "Namespace::second", "ConfigMap:first:first-cm"}, stats["created"])
			},
		},
		{
			name:       "turn-off-cluster",
			filterArgs: []string{"--include-cluster-objects=false"},
			assertFn: func(t *testing.T, s *scaffold, err error) {
				require.NoError(t, err)
				stats := s.outputStats()
				assert.EqualValues(t, []interface{}{"ConfigMap:first:first-cm", "ConfigMap::second-cm", "Secret:second:second-secret"}, stats["created"])
			},
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			s := newCustomScaffold(t, "testdata/projects/multi-ns")
			defer s.reset()
			s.client.syncFunc = func(ctx context.Context, obj model.K8sLocalObject, opts remote.SyncOptions) (*remote.SyncResult, error) {
				return &remote.SyncResult{
					Type: remote.SyncCreated,
				}, nil
			}
			s.client.getFunc = func(ctx context.Context, obj model.K8sMeta) (*unstructured.Unstructured, error) {
				return nil, nil
			}
			args := append([]string{"apply", "local", "-n", "--gc=false"}, test.filterArgs...)
			err := s.executeCommand(args...)
			test.assertFn(t, s, err)
		})
	}
}

func TestApplyNamespaceFilterMetadataError(t *testing.T) {
	s := newCustomScaffold(t, "testdata/projects/multi-ns")
	defer s.reset()
	s.client.nsFunc = func(kind schema.GroupVersionKind) (bool, error) {
		return false, fmt.Errorf("no metadata found")
	}
	err := s.executeCommand("apply", "local", "-n", "--gc=false", "-p", "first")
	require.Error(t, err)
	assert.Equal(t, "namespace filter: no metadata found", err.Error())
}

func TestApplyNegative(t *testing.T) {
	tests := []struct {
		name     string
		args     []string
		asserter func(s *scaffold, err error)
	}{
		{
			name: "no env",
			args: []string{"apply"},
			asserter: func(s *scaffold, err error) {
				a := assert.New(s.t)
				a.True(cmd.IsUsageError(err))
				a.Equal("exactly one environment required, but provided: []", err.Error())
			},
		},
		{
			name: "2 envs",
			args: []string{"apply", "dev", "prod"},
			asserter: func(s *scaffold, err error) {
				a := assert.New(s.t)
				a.True(cmd.IsUsageError(err))
				a.Equal("exactly one environment required, but provided: [\"dev\" \"prod\"]", err.Error())
			},
		},
		{
			name: "bad env",
			args: []string{"apply", "foo"},
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
			args: []string{"apply", "_"},
			asserter: func(s *scaffold, err error) {
				a := assert.New(s.t)
				a.True(cmd.IsUsageError(err))
				a.Equal("cannot apply baseline environment, use a real environment", err.Error())
			},
		},
		{
			name: "c and C",
			args: []string{"apply", "dev", "-c", "cluster-objects", "-C", "service2"},
			asserter: func(s *scaffold, err error) {
				a := assert.New(s.t)
				a.True(cmd.IsUsageError(err))
				a.Equal(`cannot include as well as exclude components, specify one or the other`, err.Error())
			},
		},
		{
			name: "k and K",
			args: []string{"apply", "dev", "-k", "namespace", "-K", "secret"},
			asserter: func(s *scaffold, err error) {
				a := assert.New(s.t)
				a.True(cmd.IsUsageError(err))
				a.Equal(`cannot include as well as exclude kinds, specify one or the other`, err.Error())
			},
		},
		{
			name: "p and P",
			args: []string{"apply", "dev", "-p", "first", "-P", "second"},
			asserter: func(s *scaffold, err error) {
				a := assert.New(s.t)
				a.True(cmd.IsUsageError(err))
				a.Equal(`cannot include as well as exclude namespaces, specify one or the other`, err.Error())
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
