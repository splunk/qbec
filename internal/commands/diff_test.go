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
	"testing"

	"github.com/splunk/qbec/internal/model"
	"github.com/splunk/qbec/internal/remote"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

type dg struct {
	cmValue     string
	secretValue string
}

func (d *dg) get(obj model.K8sMeta) (*unstructured.Unstructured, error) {
	switch {
	case obj.GetName() == "svc2-cm":
		return &unstructured.Unstructured{
			Object: map[string]interface{}{
				"apiVersion": "v1",
				"kind":       "ConfigMap",
				"metadata": map[string]interface{}{
					"creationTimestamp": "xxx",
					"namespace":         "bar-system",
					"name":              "svc2-cm",
					"annotations": map[string]interface{}{
						"ann/foo": "bar",
						"ann/bar": "baz",
					},
				},
				"data": map[string]interface{}{
					"foo": d.cmValue,
				},
			},
		}, nil
	case obj.GetName() == "svc2-secret":
		return &unstructured.Unstructured{
			Object: map[string]interface{}{
				"apiVersion": "v1",
				"kind":       "Secret",
				"metadata": map[string]interface{}{
					"creationTimestamp": "xxx",
					"namespace":         "bar-system",
					"name":              "svc2-secret",
				},
				"data": map[string]interface{}{
					"foo": base64.StdEncoding.EncodeToString([]byte(d.secretValue)),
				},
			},
		}, nil
	default:
		return nil, remote.ErrNotFound
	}

}

func TestDiffBasicNoDiffs(t *testing.T) {
	s := newScaffold(t)
	defer s.reset()
	d := &dg{cmValue: "bar"}
	s.opts.client.getFunc = d.get
	err := s.executeCommand("diff", "dev", "-k", "configmaps", "--ignore-all-annotations", "--ignore-all-labels", "--show-deletes=false")
	require.Nil(t, err)
}

func TestDiffBasic(t *testing.T) {
	s := newScaffold(t)
	defer s.reset()
	d := &dg{cmValue: "baz", secretValue: "baz"}
	s.opts.client.getFunc = d.get
	err := s.executeCommand("diff", "dev", "--show-deletes=false")
	require.NotNil(t, err)
	stats := s.outputStats()
	a := assert.New(t)
	a.EqualValues([]interface{}{"ConfigMap:bar-system:svc2-cm", "Secret:bar-system:svc2-secret"}, stats["changes"])
	secretValue := base64.StdEncoding.EncodeToString([]byte("baz"))
	redactedValue := base64.RawStdEncoding.EncodeToString([]byte("redacted."))
	a.Contains(s.stdout(), redactedValue)
	a.Contains(s.stdout(), "qbec.io/component: service2")
	a.NotContains(s.stdout(), secretValue)
}

func TestDiffBasicNoLabels(t *testing.T) {
	s := newScaffold(t)
	defer s.reset()
	d := &dg{cmValue: "baz", secretValue: "baz"}
	s.opts.client.getFunc = d.get
	err := s.executeCommand("diff", "dev", "--ignore-all-labels", "-S", "--show-deletes=false")
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
	s.opts.client.getFunc = d.get
	err := s.executeCommand("diff", "dev", "--ignore-label", "qbec.io/environment", "--show-deletes=false")
	require.NotNil(t, err)
	a := assert.New(t)
	a.NotContains(s.stdout(), "qbec.io/environment:")
	a.Contains(s.stdout(), "qbec.io/application:")
}

func TestDiffBasicNoSpecificAnnotation(t *testing.T) {
	s := newScaffold(t)
	defer s.reset()
	d := &dg{cmValue: "baz", secretValue: "baz"}
	s.opts.client.getFunc = d.get
	err := s.executeCommand("diff", "dev", "--ignore-annotation", "ann/foo", "--show-deletes=false")
	require.NotNil(t, err)
	a := assert.New(t)
	a.NotContains(s.stdout(), "ann/foo")
	a.Contains(s.stdout(), "ann/bar")
}

func TestDiffBasicNoAnnotations(t *testing.T) {
	s := newScaffold(t)
	defer s.reset()
	d := &dg{cmValue: "baz", secretValue: "baz"}
	s.opts.client.getFunc = d.get
	err := s.executeCommand("diff", "dev", "--ignore-all-annotations", "--show-deletes=false")
	require.NotNil(t, err)
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
				a.True(isUsageError(err))
				a.Equal("exactly one environment required", err.Error())
			},
		},
		{
			name: "2 envs",
			args: []string{"diff", "dev", "prod"},
			asserter: func(s *scaffold, err error) {
				a := assert.New(s.t)
				a.True(isUsageError(err))
				a.Equal("exactly one environment required", err.Error())
			},
		},
		{
			name: "bad env",
			args: []string{"diff", "foo"},
			asserter: func(s *scaffold, err error) {
				a := assert.New(s.t)
				a.False(isUsageError(err))
				a.Equal("invalid environment \"foo\"", err.Error())
			},
		},
		{
			name: "baseline env",
			args: []string{"diff", "_"},
			asserter: func(s *scaffold, err error) {
				a := assert.New(s.t)
				a.True(isUsageError(err))
				a.Equal("cannot diff baseline environment, use a real environment", err.Error())
			},
		},
		{
			name: "c and C",
			args: []string{"diff", "dev", "-c", "cluster-objects", "-C", "service2"},
			asserter: func(s *scaffold, err error) {
				a := assert.New(s.t)
				a.True(isUsageError(err))
				a.Equal(`cannot include as well as exclude components, specify one or the other`, err.Error())
			},
		},
		{
			name: "k and K",
			args: []string{"diff", "dev", "-k", "namespace", "-K", "secret"},
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
