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
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"testing"

	"github.com/splunk/qbec/internal/model"
	"github.com/splunk/qbec/vm"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func producer(component string, data map[string]interface{}) model.K8sLocalObject {
	return model.NewK8sLocalObject(data, model.LocalAttrs{App: "foo", Tag: "", Component: component, Env: "dev"})
}

func decorate(ctx Context) Context {
	ctx.Vars = ctx.Vars.WithVars(
		vm.NewVar("qbec.io/tag", "t1"),
		vm.NewVar("qbec.io/env", "dev"),
		vm.NewVar("qbec.io/cleanMode", "off"),
		vm.NewVar("qbec.io/defaultNs", "foobar"),
		vm.NewCodeVar("qbec.io/envProperties", `{ foo: "bar"}`),
	)
	return ctx
}

func TestEvalFile(t *testing.T) {
	data, err := File("testdata/params.libsonnet", BaseContext{
		Vars: vm.VariableSet{}.WithVars(
			vm.NewVar("qbec.io/tag", "t1"),
			vm.NewVar("qbec.io/env", "dev"),
			vm.NewVar("qbec.io/cleanMode", "off"),
			vm.NewVar("qbec.io/defaultNs", "foobar"),
			vm.NewCodeVar("qbec.io/envProperties", `{ foo: "bar"}`),
		),
	})
	require.Nil(t, err)
	a := assert.New(t)
	var paramsMap map[string]interface{}
	err = json.Unmarshal([]byte(data), &paramsMap)
	require.NoError(t, err)
	comps, ok := paramsMap["components"].(map[string]interface{})
	require.True(t, ok)
	base, ok := comps["base"].(map[string]interface{})
	require.True(t, ok)
	a.EqualValues("dev", base["env"])
	a.EqualValues("foobar", base["ns"])
	a.EqualValues("t1", base["tag"])
	a.EqualValues("bar", base["foo"])
}

func TestEvalCode(t *testing.T) {
	data, err := Code("params", vm.MakeCode(`import 'testdata/params.libsonnet'`), BaseContext{
		LibPaths: []string{"."},
		Vars: vm.VariableSet{}.WithVars(
			vm.NewVar("qbec.io/tag", "t1"),
			vm.NewVar("qbec.io/env", "dev"),
			vm.NewVar("qbec.io/cleanMode", "off"),
			vm.NewVar("qbec.io/defaultNs", "foobar"),
			vm.NewCodeVar("qbec.io/envProperties", `{ foo: "bar"}`),
		),
	})
	require.Nil(t, err)
	a := assert.New(t)
	var paramsMap map[string]interface{}
	err = json.Unmarshal([]byte(data), &paramsMap)
	require.NoError(t, err)
	comps, ok := paramsMap["components"].(map[string]interface{})
	require.True(t, ok)
	base, ok := comps["base"].(map[string]interface{})
	require.True(t, ok)
	a.EqualValues("dev", base["env"])
	a.EqualValues("foobar", base["ns"])
	a.EqualValues("t1", base["tag"])
	a.EqualValues("bar", base["foo"])
}

func TestEvalParams(t *testing.T) {
	paramsMap, err := Params("testdata/params.libsonnet", decorate(Context{
		BaseContext: BaseContext{Verbose: true},
	}))
	require.Nil(t, err)
	a := assert.New(t)
	comps, ok := paramsMap["components"].(map[string]interface{})
	require.True(t, ok)
	base, ok := comps["base"].(map[string]interface{})
	require.True(t, ok)
	a.EqualValues("dev", base["env"])
	a.EqualValues("foobar", base["ns"])
	a.EqualValues("t1", base["tag"])
	a.EqualValues("bar", base["foo"])
}

func TestEvalParamsNegative(t *testing.T) {
	_, err := Params("testdata/params.invalid.libsonnet", decorate(Context{}))
	require.NotNil(t, err)
	require.Contains(t, err.Error(), "end of file")

	_, err = Params("testdata/params.non-object.libsonnet", decorate(Context{}))
	require.NotNil(t, err)
	require.Contains(t, err.Error(), "cannot unmarshal array")
}

func TestEvalComponents(t *testing.T) {
	objs, err := Components([]model.Component{
		{
			Name:  "b",
			Files: []string{"testdata/components/b.yaml"},
		},
		{
			Name:  "c",
			Files: []string{"testdata/components/c.jsonnet"},
		},
		{
			Name:  "a",
			Files: []string{"testdata/components/a.json"},
		},
		{
			Name: "d",
			Files: []string{
				"testdata/components/d/index.yaml",
				"testdata/components/d/subdir-cm.yaml",
				"testdata/components/d/subdir-cm2.json",
			},
		},
		{
			Name:         "tla",
			Files:        []string{"testdata/components/tla.jsonnet"},
			TopLevelVars: []string{"foo", "bar"},
		},
	},
		decorate(Context{
			BaseContext: BaseContext{
				Vars:    vm.VariableSet{}.WithTopLevelVars(vm.NewVar("foo", "foo"), vm.NewCodeVar("bar", "true")),
				Verbose: true,
			},
			PostProcessFiles: []string{"testdata/components/pp/pp.jsonnet", "testdata/components/pp/pp2.jsonnet"},
		}),
		producer,
	)
	require.Nil(t, err)
	require.Equal(t, 6, len(objs))
	a := assert.New(t)

	// ensure postprocessor is called everywhere
	for _, obj := range objs {
		a.Equal("dev", obj.Environment())
		a.Equal("service2", obj.ToUnstructured().GetAnnotations()["team"])
		a.Equal("#svc2", obj.ToUnstructured().GetAnnotations()["slack"])
		a.Equal("bar", obj.ToUnstructured().GetLabels()["foo"])
	}
	obj := objs[0]
	a.Equal("a", obj.Component())
	a.Equal("", obj.GroupVersionKind().Group)
	a.Equal("v1", obj.GroupVersionKind().Version)
	a.Equal("ConfigMap", obj.GroupVersionKind().Kind)
	a.Equal("", obj.GetNamespace())
	a.Equal("json-config-map", obj.GetName())

	obj = objs[1]
	a.Equal("b", obj.Component())
	a.Equal("yaml-config-map", obj.GetName())

	obj = objs[2]
	a.Equal("c", obj.Component())
	a.Equal("foobar", obj.GetName())
	t.Log(obj)

	obj = objs[3]
	a.Equal("d", obj.Component())
	a.Equal("subdir-config-map1", obj.GetName())

	obj = objs[4]
	a.Equal("d", obj.Component())
	a.Equal("subdir-config-map2", obj.GetName())

	obj = objs[5]
	a.Equal("tla", obj.Component())
	a.Equal("tla-config-map", obj.GetName())
}

func TestEvalComponentsEdges(t *testing.T) {
	goodComponents := []model.Component{
		{Name: "g1", Files: []string{"testdata/good-components/g1.jsonnet"}},
		{Name: "g2", Files: []string{"testdata/good-components/g2.jsonnet"}},
		{Name: "g3", Files: []string{"testdata/good-components/g3.jsonnet"}},
		{Name: "g4", Files: []string{"testdata/good-components/g4.jsonnet"}},
		{Name: "g5", Files: []string{"testdata/good-components/g5.jsonnet"}},
	}
	goodAssert := func(t *testing.T, ret []model.K8sLocalObject, err error) {
		require.NotNil(t, err)
	}
	tests := []struct {
		name        string
		components  []model.Component
		asserter    func(*testing.T, []model.K8sLocalObject, error)
		concurrency int
	}{
		{
			name: "no components",
			asserter: func(t *testing.T, ret []model.K8sLocalObject, err error) {
				require.Nil(t, err)
				assert.Equal(t, 0, len(ret))
			},
		},
		{
			name:       "single bad",
			components: []model.Component{{Name: "e1", Files: []string{"testdata/bad-components/e1.jsonnet"}}},
			asserter: func(t *testing.T, ret []model.K8sLocalObject, err error) {
				require.NotNil(t, err)
				assert.Contains(t, err.Error(), "evaluate 'e1'")
			},
		},
		{
			name: "two bad",
			components: []model.Component{
				{Name: "e1", Files: []string{"testdata/bad-components/e1.jsonnet"}},
				{Name: "e2", Files: []string{"testdata/bad-components/e2.jsonnet"}},
			},
			asserter: func(t *testing.T, ret []model.K8sLocalObject, err error) {
				require.NotNil(t, err)
				assert.Contains(t, err.Error(), "evaluate 'e1'")
				assert.Contains(t, err.Error(), "evaluate 'e2'")
			},
		},
		{
			name: "many bad",
			components: []model.Component{
				{Name: "e1", Files: []string{"testdata/bad-components/e1.jsonnet"}},
				{Name: "e2", Files: []string{"testdata/bad-components/e2.jsonnet"}},
				{Name: "e3", Files: []string{"testdata/bad-components/e3.jsonnet"}},
				{Name: "e4", Files: []string{"testdata/bad-components/e4.jsonnet"}},
				{Name: "e5", Files: []string{"testdata/bad-components/e5.jsonnet"}},
			},
			asserter: func(t *testing.T, ret []model.K8sLocalObject, err error) {
				require.NotNil(t, err)
				assert.Contains(t, err.Error(), "... and 2 more errors")
			},
		},
		{
			name: "bad file",
			components: []model.Component{
				{Name: "e1", Files: []string{"testdata/bad-components/XXX.jsonnet"}},
			},
			asserter: func(t *testing.T, ret []model.K8sLocalObject, err error) {
				require.NotNil(t, err)
				assert.Contains(t, err.Error(), "file not found")
			},
		},
		{
			name: "bad YAML file",
			components: []model.Component{
				{Name: "e1", Files: []string{"testdata/bad-components/XXX.yaml"}},
			},
			asserter: func(t *testing.T, ret []model.K8sLocalObject, err error) {
				require.NotNil(t, err)
				assert.Contains(t, err.Error(), "file not found")
			},
		},
		{
			name: "bad JSON file",
			components: []model.Component{
				{Name: "e1", Files: []string{"testdata/bad-components/XXX.json"}},
			},
			asserter: func(t *testing.T, ret []model.K8sLocalObject, err error) {
				require.NotNil(t, err)
				assert.Contains(t, err.Error(), "file not found")
			},
		},
		{
			name:        "negative concurrency",
			components:  goodComponents,
			asserter:    goodAssert,
			concurrency: -10,
		},
		{
			name:        "zero concurrency",
			components:  goodComponents,
			asserter:    goodAssert,
			concurrency: 0,
		},
		{
			name:        "4 concurrency",
			components:  goodComponents,
			asserter:    goodAssert,
			concurrency: 4,
		},
		{
			name:        "one concurrency",
			components:  goodComponents,
			asserter:    goodAssert,
			concurrency: 1,
		},
		{
			name:        "million concurrency",
			components:  goodComponents,
			asserter:    goodAssert,
			concurrency: 1000000,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			ret, err := evalComponents(test.components, decorate(Context{
				Concurrency: test.concurrency,
			}), nil, producer)
			test.asserter(t, ret, err)
		})
	}
}

func TestEvalComponentsBadJson(t *testing.T) {
	_, err := Components([]model.Component{
		{
			Name:  "bad",
			Files: []string{"testdata/components/bad.json"},
		},
	}, decorate(Context{}), producer)
	require.NotNil(t, err)
	require.Contains(t, err.Error(), "invalid character")
}

func TestEvalComponentsBadPostProcessor(t *testing.T) {
	_, err := Components([]model.Component{
		{
			Name:  "bad",
			Files: []string{"testdata/components/a.json"},
		},
	}, decorate(Context{PostProcessFiles: []string{"foo/bar.jsonnet"}}), producer)
	require.NotNil(t, err)
	require.Contains(t, err.Error(), "run post-processor foo/bar.jsonnet:")
}

func TestEvalComponentsBadYaml(t *testing.T) {
	_, err := Components([]model.Component{
		{
			Name:  "bad",
			Files: []string{"testdata/components/bad.yaml"},
		},
	}, decorate(Context{}), producer)
	require.NotNil(t, err)
	require.Contains(t, err.Error(), "did not find expected node content")
}

func TestEvalComponentsBadObjects(t *testing.T) {
	_, err := Components([]model.Component{
		{
			Name:  "bad",
			Files: []string{"testdata/components/bad-objects.yaml"},
		},
	}, decorate(Context{}), producer)
	require.NotNil(t, err)
	require.Contains(t, err.Error(), `non-kubernetes object found while evaluating path "$[0].foo" (found "string"`)
}

func TestEvalComponentsBadMetadata(t *testing.T) {
	_, err := Components([]model.Component{
		{
			Name:  "bad-metadata",
			Files: []string{"testdata/components/bad-metadata.yaml"},
		},
	}, decorate(Context{}), producer)
	require.NotNil(t, err)
	require.Contains(t, err.Error(), `/v1, Kind=ConfigMap, Name=subdir-config-map1: .metadata.annotations accessor error`)
}

func TestEvalComponentsBadPostProc(t *testing.T) {
	_, err := Components([]model.Component{
		{
			Name:  "bad-postproc",
			Files: []string{"testdata/components/b.yaml"},
		},
	}, decorate(Context{PostProcessFiles: []string{"testdata/components/bad-pp.libsonnet"}}), producer)
	require.NotNil(t, err)
	require.Contains(t, err.Error(), `post-eval did not return an object`)
}

func TestEvalPostProcessor(t *testing.T) {
	obj := map[string]interface{}{
		"apiVersion": "v1",
		"kind":       "ConfigMap",
		"metadata": map[string]interface{}{
			"name": "cm",
		},
		"data": map[string]interface{}{
			"foo": "bar",
		},
	}
	tests := []struct {
		name     string
		code     string
		asserter func(t *testing.T, ret map[string]interface{}, err error)
	}{
		{
			name: "add annotation",
			code: `function (object) object + { metadata +: { annotations +:{ slack: '#crash' }}}`,
			asserter: func(t *testing.T, ret map[string]interface{}, err error) {
				require.Nil(t, err)
				ann := ret["metadata"].(map[string]interface{})["annotations"].(map[string]interface{})["slack"]
				assert.Equal(t, "#crash", ann)
			},
		},
		{
			name: "return scalar",
			code: `function (object) "boo"`,
			asserter: func(t *testing.T, ret map[string]interface{}, err error) {
				require.NotNil(t, err)
				assert.Equal(t, `post-eval did not return an object, "boo"`+"\n", err.Error())
			},
		},
		{
			name: "return array",
			code: `function (object) [ object ]`,
			asserter: func(t *testing.T, ret map[string]interface{}, err error) {
				require.NotNil(t, err)
				assert.Contains(t, err.Error(), `post-eval did not return an object, [`)
			},
		},
		{
			name: "return k8s list",
			code: `function (object) { apiVersion: "v1", kind: "List", items: [ object ] }`,
			asserter: func(t *testing.T, ret map[string]interface{}, err error) {
				require.NotNil(t, err)
				assert.Contains(t, err.Error(), `post-eval did not return a K8s object,`)
			},
		},
		{
			name: "bad code",
			code: `function (object) object2`,
			asserter: func(t *testing.T, ret map[string]interface{}, err error) {
				require.NotNil(t, err)
				assert.Contains(t, err.Error(), `post-eval object:`)
			},
		},
		{
			name: "bad tla",
			code: `function (o) o`,
			asserter: func(t *testing.T, ret map[string]interface{}, err error) {
				require.NotNil(t, err)
				assert.Contains(t, err.Error(), `post-eval object: RUNTIME ERROR: function has no parameter object`)
			},
		},
	}

	tmpDir, err := ioutil.TempDir("", "pp*")
	require.NoError(t, err)
	defer os.RemoveAll(tmpDir)
	count := 0
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			count++
			ctx := decorate(Context{})
			file := filepath.Join(tmpDir, fmt.Sprintf("f%03d.libsonnet", count))
			err := ioutil.WriteFile(file, []byte(test.code), 0644)
			require.NoError(t, err)
			pp := postProc{ctx: ctx, file: file}
			ret, err := pp.run(obj)
			test.asserter(t, ret, err)
		})
	}
}
