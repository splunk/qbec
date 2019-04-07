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

package model

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestValidatorBasic(t *testing.T) {
	doc := `---
apiVersion: qbec.io/v1alpha1
kind: App
metadata:
  name: foobar
spec:
  excludes:
    - a
  vars:
    external:
      - name: foo
        secret: true
        default: 10
      - name: bar
        default: { foo: { bar : { baz : 10 } } }
    topLevel:
      - name: foo
        secret: true
        components: [ 'a', 'b' ]
  environments:
    dev:
      server: "https://dev-server"
      includes:
      - a
      - b
      excludes:
      - c
      - d
`
	v, err := newValidator()
	require.Nil(t, err)
	errs := v.validateYAML([]byte(doc))
	for _, e := range errs {
		t.Log(e)
	}
	require.Nil(t, errs)
}

func TestValidatorNegative(t *testing.T) {
	tests := []struct {
		name     string
		yaml     string
		asserter func(t *testing.T, errs []error)
	}{
		{
			name: "bad yaml",
			yaml: `{ foo`,
			asserter: func(t *testing.T, errs []error) {
				require.Equal(t, 1, len(errs))
				assert.Contains(t, errs[0].Error(), "YAML unmarshal")
			},
		},
		{
			name: "no kind",
			yaml: `{ apiVersion: "qbec.io/v1alpha1", metadata: { name: "foo"}, spec: { environments: { dev: { server: "https://dev" } } } }`,
			asserter: func(t *testing.T, errs []error) {
				require.Equal(t, 1, len(errs))
				assert.Equal(t, "missing or invalid kind property", errs[0].Error())
			},
		},
		{
			name: "bad kind",
			yaml: `{ apiVersion: "qbec.io/v1alpha1", kind: "app", metadata: { name: "foo"}, spec: { environments: { dev: { server: "https://dev" } } } }`,
			asserter: func(t *testing.T, errs []error) {
				require.Equal(t, 1, len(errs))
				assert.Equal(t, "bad kind property, expected App", errs[0].Error())
			},
		},
		{
			name: "bad api version",
			yaml: `{ apiVersion: "qbec.io/v1alpha2", kind: "App", metadata: { name: "foo"}, spec: { environments: { dev: { server: "https://dev" } } } }`,
			asserter: func(t *testing.T, errs []error) {
				require.Equal(t, 1, len(errs))
				assert.Equal(t, "no schema found for qbec.io.v1alpha2.App (check for valid apiVersion and kind properties)", errs[0].Error())
			},
		},
		{
			name: "no apiVersion",
			yaml: `{ kind: "App", metadata: { name: "foo"}, spec: { environments: { dev: { server: "https://dev" } } } }`,
			asserter: func(t *testing.T, errs []error) {
				require.Equal(t, 1, len(errs))
				assert.Equal(t, "missing or invalid apiVersion property", errs[0].Error())
			},
		},
		{
			name: "no metadata",
			yaml: `{ apiVersion: "qbec.io/v1alpha1", kind: "App", spec: { environments: { dev: { server: "https://dev" } } } }`,
			asserter: func(t *testing.T, errs []error) {
				require.Equal(t, 1, len(errs))
				assert.Equal(t, ".metadata in body is required", errs[0].Error())
			},
		},
		{
			name: "no metadata name",
			yaml: `{ apiVersion: "qbec.io/v1alpha1", kind: "App", metadata: {}, spec: { environments: { dev: { server: "https://dev" } } } }`,
			asserter: func(t *testing.T, errs []error) {
				require.Equal(t, 1, len(errs))
				assert.Equal(t, "metadata.name in body is required", errs[0].Error())
			},
		},
		{
			name: "no environments key",
			yaml: `{ apiVersion: "qbec.io/v1alpha1", kind: "App", metadata: { name: "foo"}, spec: {} }`,
			asserter: func(t *testing.T, errs []error) {
				require.Equal(t, 1, len(errs))
				assert.Equal(t, "spec.environments in body is required", errs[0].Error())
			},
		},
		{
			name: "no environments",
			yaml: `{ apiVersion: "qbec.io/v1alpha1", kind: "App", metadata: { name: "foo"}, spec: { environments: {} } }`,
			asserter: func(t *testing.T, errs []error) {
				require.Equal(t, 1, len(errs))
				assert.Equal(t, "spec.environments in body should have at least 1 properties", errs[0].Error())
			},
		},
		{
			name: "bad excludes",
			yaml: `{ apiVersion: "qbec.io/v1alpha1", kind: "App", metadata: { name: "foo"}, spec: { excludes : "foo", environments: { dev: { server: "https://dev" } } } }`,
			asserter: func(t *testing.T, errs []error) {
				require.Equal(t, 1, len(errs))
				assert.Equal(t, "spec.excludes in body must be of type array: \"string\"", errs[0].Error())
			},
		},
		{
			name: "bad env excludes",
			yaml: `{ apiVersion: "qbec.io/v1alpha1", kind: "App", metadata: { name: "foo"}, spec: {  environments: { dev: { server: "https://dev", excludes: "foo" } } } }`,
			asserter: func(t *testing.T, errs []error) {
				require.Equal(t, 1, len(errs))
				assert.Equal(t, "spec.environments.dev.excludes in body must be of type array: \"string\"", errs[0].Error())
			},
		},
		{
			name: "extra props",
			yaml: `{ apiVersion: "qbec.io/v1alpha1", kind: "App", metadata: { name: "foo"}, spec: { environments: { dev: { server: "https://dev" } } }, excludes: ["bar"] }`,
			asserter: func(t *testing.T, errs []error) {
				require.Equal(t, 1, len(errs))
				assert.Equal(t, ".excludes in body is a forbidden property", errs[0].Error())
			},
		},
		{
			name: "no components for TLA",
			yaml: `{ apiVersion: "qbec.io/v1alpha1", kind: "App", metadata: { name: "foo"}, spec: { vars: { topLevel: [ { name: 'foo' } ] }, environments: { dev: { server: "https://dev" } } } } }`,
			asserter: func(t *testing.T, errs []error) {
				require.Equal(t, 1, len(errs))
				assert.Equal(t, "spec.vars.topLevel.components in body is required", errs[0].Error())
			},
		},
		{
			name: "empty components for TLA",
			yaml: `{ apiVersion: "qbec.io/v1alpha1", kind: "App", metadata: { name: "foo"}, spec: { vars: { topLevel: [ { name: 'foo', components: [] } ] }, environments: { dev: { server: "https://dev" } } } } }`,
			asserter: func(t *testing.T, errs []error) {
				require.Equal(t, 1, len(errs))
				assert.Equal(t, "spec.vars.topLevel.components in body should have at least 1 items", errs[0].Error())
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			v, err := newValidator()
			require.Nil(t, err)
			errs := v.validateYAML([]byte(test.yaml))
			require.NotNil(t, errs)
			for _, e := range errs {
				t.Log(e)
			}
			test.asserter(t, errs)
		})
	}
}
