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
	"github.com/splunk/qbec/internal/remote/k8smeta"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

type v struct{}

func (v *v) Validate(obj *unstructured.Unstructured) []error {
	if obj.GetName() == "svc2-cm" {
		return []error{fmt.Errorf("bad config map")}
	}
	return nil
}

func factory(ctx context.Context, gvk schema.GroupVersionKind) (k8smeta.Validator, error) {
	if gvk.Kind == "PodSecurityPolicy" {
		return nil, k8smeta.ErrSchemaNotFound
	}
	return &v{}, nil
}

func TestValidateAll(t *testing.T) {
	s := newScaffold(t)
	defer s.reset()
	s.client.validatorFunc = factory
	err := s.executeCommand("validate", "dev")
	require.NotNil(t, err)
	s.assertOutputLineMatch(regexp.MustCompile(`✔ ClusterRole::allow-root-psp-policy is valid`))
	s.assertOutputLineMatch(regexp.MustCompile(`\? PodSecurityPolicy::100-default: no schema found, cannot validate`))
	s.assertOutputLineMatch(regexp.MustCompile(`✘ ConfigMap:bar-system:svc2-cm is invalid`))
	s.assertOutputLineMatch(regexp.MustCompile(`- bad config map`))
}

func TestValidateSilent(t *testing.T) {
	s := newScaffold(t)
	defer s.reset()
	s.client.validatorFunc = factory
	err := s.executeCommand("validate", "dev", "--silent")
	require.NotNil(t, err)
	s.assertOutputLineNoMatch(regexp.MustCompile(`✔ ClusterRole::allow-root-psp-policy is valid`))
	s.assertOutputLineNoMatch(regexp.MustCompile(`\? PodSecurityPolicy::100-default: no schema found, cannot validate`))
	s.assertOutputLineMatch(regexp.MustCompile(`✘ ConfigMap:bar-system:svc2-cm is invalid`))
	s.assertOutputLineMatch(regexp.MustCompile(`- bad config map`))
}

func TestValidateNegative(t *testing.T) {
	tests := []struct {
		name     string
		args     []string
		init     func(s *scaffold)
		asserter func(s *scaffold, err error)
		dir      string
	}{
		{
			name: "no env",
			args: []string{"validate"},
			asserter: func(s *scaffold, err error) {
				a := assert.New(s.t)
				a.True(cmd.IsUsageError(err))
				a.Equal("exactly one environment required, but provided: []", err.Error())
			},
		},
		{
			name: "2 envs",
			args: []string{"validate", "dev", "prod"},
			asserter: func(s *scaffold, err error) {
				a := assert.New(s.t)
				a.True(cmd.IsUsageError(err))
				a.Equal("exactly one environment required, but provided: [\"dev\" \"prod\"]", err.Error())
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
			name: "bad env",
			args: []string{"validate", "foo"},
			asserter: func(s *scaffold, err error) {
				a := assert.New(s.t)
				a.False(cmd.IsUsageError(err))
				a.Equal(`invalid environment "foo"`, err.Error())
			},
		},
		{
			name: "bad component",
			args: []string{"validate", "dev", "-c", "foo"},
			asserter: func(s *scaffold, err error) {
				a := assert.New(s.t)
				a.False(cmd.IsUsageError(err))
				a.Equal(`specified components: bad component reference(s): foo`, err.Error())
			},
		},
		{
			name: "bad filters",
			args: []string{"validate", "dev", "-c", "svc1-cm", "-C", "svc2-cm"},
			asserter: func(s *scaffold, err error) {
				a := assert.New(s.t)
				a.True(cmd.IsUsageError(err))
				a.Equal(`cannot include as well as exclude components, specify one or the other`, err.Error())
			},
		},
		{
			name: "duplicate objects",
			dir:  "testdata/dups",
			args: []string{"validate", "dev"},
			asserter: func(s *scaffold, err error) {
				a := assert.New(s.t)
				a.True(cmd.IsRuntimeError(err))
				a.Equal(`duplicate objects ConfigMap cm1 (component: x) and ConfigMap cm1 (component: y)`, err.Error())
			},
		},
		{
			name: "duplicate objects even with filters",
			dir:  "testdata/dups",
			args: []string{"validate", "dev", "-K", "configmap"},
			asserter: func(s *scaffold, err error) {
				a := assert.New(s.t)
				a.True(cmd.IsRuntimeError(err))
				a.Equal(`duplicate objects ConfigMap cm1 (component: x) and ConfigMap cm1 (component: y)`, err.Error())
			},
		},
		{
			name: "baseline",
			args: []string{"validate", "_"},
			asserter: func(s *scaffold, err error) {
				a := assert.New(s.t)
				a.True(cmd.IsUsageError(err))
				a.Equal(`cannot validate baseline environment, use a real environment`, err.Error())
			},
		},
		{
			name: "errors",
			args: []string{"validate", "dev"},
			init: func(s *scaffold) {
				s.client.validatorFunc = func(ctx context.Context, gvk schema.GroupVersionKind) (k8smeta.Validator, error) {
					return nil, fmt.Errorf("no validator for you")
				}
			},
			asserter: func(s *scaffold, err error) {
				a := assert.New(s.t)
				a.False(cmd.IsUsageError(err))
				a.Contains(err.Error(), "no validator for you")
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			s := newCustomScaffold(t, test.dir)
			defer s.reset()
			s.client.validatorFunc = factory
			if test.init != nil {
				test.init(s)
			}
			err := s.executeCommand(test.args...)
			require.NotNil(t, err)
			test.asserter(s, err)
		})
	}
}
