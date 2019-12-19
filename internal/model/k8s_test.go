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
	"encoding/json"
	"fmt"
	"testing"

	"github.com/ghodss/yaml"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var cm = `
---
apiVersion:  v1
kind: ConfigMap
metadata:
  namespace: ns1
  name: cm
data:
  foo: bar
`

func toData(s string) map[string]interface{} {
	var data map[string]interface{}
	err := yaml.Unmarshal([]byte(s), &data)
	if err != nil {
		panic(err)
	}
	return data
}

func TestK8sObject(t *testing.T) {
	obj := NewK8sObject(toData(cm))
	a := assert.New(t)
	a.Equal("cm", obj.GetName())
	a.Equal("ns1", obj.GetNamespace())
	a.Equal("ConfigMap", obj.GetKind())
	a.Equal("", obj.GroupVersionKind().Group)
	a.Equal("v1", obj.GroupVersionKind().Version)
	a.Equal("ConfigMap", obj.GroupVersionKind().Kind)
	a.NotNil(obj.ToUnstructured())
	a.Equal("/v1, Kind=ConfigMap:ns1:cm", fmt.Sprint(obj))
	b, err := json.Marshal(obj)
	require.Nil(t, err)
	s := string(b)
	a.Contains(s, `"foo":"bar"`)
}

func TestK8sLocalObject(t *testing.T) {
	obj := NewK8sLocalObject(toData(cm), "app1", "", "c1", "e1")
	a := assert.New(t)
	a.Equal("app1", obj.Application())
	a.Equal("c1", obj.Component())
	a.Equal("e1", obj.Environment())
	a.Equal("", obj.Tag())
	labels := obj.ToUnstructured().GetLabels()
	a.Equal("app1", labels[QbecNames.ApplicationLabel])
	a.Equal("e1", labels[QbecNames.EnvironmentLabel])
	_, ok := labels[QbecNames.TagLabel]
	a.False(ok)
}

func TestK8sLocalObjectWithTag(t *testing.T) {
	obj := NewK8sLocalObject(toData(cm), "app1", "t1", "c1", "e1")
	a := assert.New(t)
	a.Equal("app1", obj.Application())
	a.Equal("c1", obj.Component())
	a.Equal("e1", obj.Environment())
	a.Equal("t1", obj.Tag())
	labels := obj.ToUnstructured().GetLabels()
	a.Equal("app1", labels[QbecNames.ApplicationLabel])
	a.Equal("e1", labels[QbecNames.EnvironmentLabel])
	a.Equal("t1", labels[QbecNames.TagLabel])
}

func TestAssertMetadata(t *testing.T) {
	good := `
apiVersion: v1
kind: ConfigMap
metadata:
  name: foo
  labels:
    foo: bar
data:
  foo: bar
`
	nilAnnotations := `
apiVersion: v1
kind: ConfigMap
metadata:
  name: foo
  annotations:
data:
  foo: bar
`
	badLabels := `
apiVersion: v1
kind: ConfigMap
metadata:
  name: foo
  labels:
    foo: 10
  annotations:
    x: "foo"
data:
  foo: bar
`
	badAnnotations := `
apiVersion: v1
kind: ConfigMap
metadata:
  name: foo
  labels:
    x: "foo"
  annotations:
    foo: true
data:
  foo: bar
`

	tests := []struct {
		name     string
		data     map[string]interface{}
		assertFn func(t *testing.T, err error)
	}{
		{
			"good",
			toData(good),
			func(t *testing.T, err error) {
				assert.Nil(t, err)
			},
		},
		{
			"nil-annotations",
			toData(nilAnnotations),
			func(t *testing.T, err error) {
				assert.Nil(t, err)
			},
		},
		{
			"bad-labels",
			toData(badLabels),
			func(t *testing.T, err error) {
				require.NotNil(t, err)
				assert.Contains(t, err.Error(), "/v1, Kind=ConfigMap, Name=foo: .metadata.labels accessor error")
			},
		},
		{
			"bad-annotations",
			toData(badAnnotations),
			func(t *testing.T, err error) {
				require.NotNil(t, err)
				assert.Contains(t, err.Error(), "/v1, Kind=ConfigMap, Name=foo: .metadata.annotations accessor error")
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			err := AssertMetadataValid(test.data)
			test.assertFn(t, err)
		})
	}
}
