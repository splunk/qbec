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

package eval

import (
	"testing"

	"github.com/splunk/qbec/internal/model"
	"github.com/splunk/qbec/internal/vm"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestEvalParams(t *testing.T) {
	paramsMap, err := Params("testdata/params.libsonnet", Context{
		Env:     "dev",
		Verbose: true,
	})
	require.Nil(t, err)
	a := assert.New(t)
	comps, ok := paramsMap["components"].(map[string]interface{})
	require.True(t, ok)
	base, ok := comps["base"].(map[string]interface{})
	require.True(t, ok)
	a.EqualValues("dev", base["env"])
}

func TestEvalParamsNegative(t *testing.T) {
	_, err := Params("testdata/params.invalid.libsonnet", Context{Env: "dev"})
	require.NotNil(t, err)
	require.Contains(t, err.Error(), "end of file")

	_, err = Params("testdata/params.non-object.libsonnet", Context{Env: "dev"})
	require.NotNil(t, err)
	require.Contains(t, err.Error(), "cannot unmarshal array")
}

func TestEvalComponents(t *testing.T) {
	objs, err := Components([]model.Component{
		{
			Name: "b",
			File: "testdata/components/b.yaml",
		},
		{
			Name: "c",
			File: "testdata/components/c.jsonnet",
		},
		{
			Name: "a",
			File: "testdata/components/a.json",
		},
	}, Context{Env: "dev", Verbose: true})
	require.Nil(t, err)
	require.Equal(t, 3, len(objs))
	a := assert.New(t)

	obj := objs[0]
	a.Equal("a", obj.Component())
	a.Equal("dev", obj.Environment())
	a.Equal("", obj.GetObjectKind().GroupVersionKind().Group)
	a.Equal("v1", obj.GetObjectKind().GroupVersionKind().Version)
	a.Equal("ConfigMap", obj.GetObjectKind().GroupVersionKind().Kind)
	a.Equal("", obj.GetNamespace())
	a.Equal("json-config-map", obj.GetName())

	obj = objs[1]
	a.Equal("b", obj.Component())
	a.Equal("dev", obj.Environment())
	a.Equal("yaml-config-map", obj.GetName())

	obj = objs[2]
	a.Equal("c", obj.Component())
	a.Equal("dev", obj.Environment())
	a.Equal("jsonnet-config-map", obj.GetName())
}

func TestEvalComponentsBadJson(t *testing.T) {
	_, err := Components([]model.Component{
		{
			Name: "bad",
			File: "testdata/components/bad.json",
		},
	}, Context{Env: "dev", VM: vm.New(vm.Config{})})
	require.NotNil(t, err)
	require.Contains(t, err.Error(), "invalid character")
}

func TestEvalComponentsBadYaml(t *testing.T) {
	_, err := Components([]model.Component{
		{
			Name: "bad",
			File: "testdata/components/bad.yaml",
		},
	}, Context{Env: "dev", VM: vm.New(vm.Config{})})
	require.NotNil(t, err)
	require.Contains(t, err.Error(), "did not find expected node content")
}

func TestEvalComponentsBadObjects(t *testing.T) {
	_, err := Components([]model.Component{
		{
			Name: "bad",
			File: "testdata/components/bad-objects.yaml",
		},
	}, Context{Env: "dev", VM: vm.New(vm.Config{})})
	require.NotNil(t, err)
	require.Contains(t, err.Error(), `unexpected type for object (string) at path "$.bad[0].foo"`)
}
