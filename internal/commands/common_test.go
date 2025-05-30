// Copyright 2025 Splunk Inc.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package commands

import (
	"bytes"
	"context"
	"fmt"
	"sync"
	"testing"

	"github.com/ghodss/yaml"
	"github.com/pkg/errors"
	"github.com/splunk/qbec/internal/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRunInParallelNoObjects(t *testing.T) {
	err := runInParallel(context.TODO(), []model.K8sLocalObject{}, func(ctx context.Context, o model.K8sLocalObject) error { return nil }, 5)
	require.NoError(t, err)
}

type input struct {
	component string
	env       string
	namespace string
	name      string
}

func (i input) makeObject() model.K8sLocalObject {
	data := map[string]interface{}{
		"apiVersion": "v1",
		"kind":       "ConfigMap",
		"metadata": map[string]interface{}{
			"namespace": i.namespace,
			"name":      i.name,
		},
		"data": map[string]interface{}{
			"foo": "bar",
		},
	}
	return model.NewK8sLocalObject(data, model.LocalAttrs{App: "app1", Tag: "t1", Component: i.component, Env: i.env})
}

func (i input) String() string {
	return fmt.Sprintf("%s:%s:%s:%s", i.component, i.env, i.namespace, i.name)
}

func TestRunInParallel(t *testing.T) {
	var l sync.Mutex
	seen := map[string]bool{}
	setSeen := func(s string) {
		l.Lock()
		defer l.Unlock()
		seen[s] = true
	}
	worker := func(ctx context.Context, o model.K8sLocalObject) error {
		str := fmt.Sprintf("%s:%s:%s:%s", o.Component(), o.Environment(), o.GetNamespace(), o.GetName())
		setSeen(str)
		return nil
	}
	inputs := []input{
		{component: "c1", env: "dev", namespace: "default", name: "c1"},
		{component: "c1", env: "dev", namespace: "default", name: "c2"},
		{component: "c1", env: "dev", namespace: "default", name: "c3"},
		{component: "c1", env: "dev", namespace: "default", name: "c4"},
		{component: "c2", env: "dev", namespace: "kube-system", name: "k1"},
		{component: "c2", env: "dev", namespace: "kube-system", name: "k2"},
		{component: "c2", env: "dev", namespace: "kube-system", name: "k3"},
		{component: "c3", env: "dev", namespace: "kube-public", name: "p1"},
		{component: "c3", env: "dev", namespace: "kube-public", name: "p2"},
		{component: "c3", env: "dev", namespace: "kube-public", name: "p3"},
	}
	var objs []model.K8sLocalObject
	for _, in := range inputs {
		objs = append(objs, in.makeObject())
	}
	ctx := context.TODO()
	err := runInParallel(ctx, objs, worker, 5)
	require.NoError(t, err)
	a := assert.New(t)
	for _, in := range inputs {
		a.Contains(seen, in.String())
	}

	seen = map[string]bool{}
	worker = func(ctx context.Context, o model.K8sLocalObject) error {
		str := fmt.Sprintf("%s:%s:%s:%s", o.Component(), o.Environment(), o.GetNamespace(), o.GetName())
		setSeen(str)
		if o.GetNamespace() == "kube-system" {
			return errors.New("kserr")
		}
		return nil
	}

	err = runInParallel(ctx, objs, worker, 0)
	require.NotNil(t, err)
	a.True(len(seen) < len(inputs))
	a.Contains(err.Error(), "/v1, Kind=ConfigMap:kube-system:k1: kserr")
}

func TestStats(t *testing.T) {
	data := map[string]int{
		"processed": 10,
		"success":   9,
		"failure":   1,
	}
	var buf bytes.Buffer
	printStats(&buf, data)
	expected := `---
stats:
  failure: 1
  processed: 10
  success: 9

`
	assert.Equal(t, expected, buf.String())
}

func TestOrdering(t *testing.T) {
	simple := `
apiVersion: v1
kind: ConfigMap
metadata:
  name: foo
data:
  foo: bar
`
	good := `
apiVersion: v1
kind: ConfigMap
metadata:
  name: foo
  annotations:
    directives.qbec.io/apply-order: "1000"
data:
  foo: bar
`
	bad := `
apiVersion: v1
kind: ConfigMap
metadata:
  name: foo
  annotations:
    directives.qbec.io/apply-order: "foo"
data:
  foo: bar
`
	unmarshal := func(s string) map[string]interface{} {
		ret := map[string]interface{}{}
		err := yaml.Unmarshal([]byte(s), &ret)
		if err != nil {
			panic(err)
		}
		return ret
	}
	tests := []struct {
		name     string
		data     map[string]interface{}
		expected int
	}{
		{"no annotations", unmarshal(simple), 0},
		{"bad", unmarshal(bad), 0},
		{"good", unmarshal(good), 1000},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			ret := ordering(model.NewK8sLocalObject(test.data, model.LocalAttrs{
				App:       "app",
				Tag:       "tag",
				Component: "component",
				Env:       "env",
			}))
			assert.Equal(t, test.expected, ret)
		})
	}
}
