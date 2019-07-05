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
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestEvalParams(t *testing.T) {
	paramsMap, err := Params("testdata/params.libsonnet", Context{
		Env:       "dev",
		Tag:       "t1",
		DefaultNs: "foobar",
		Verbose:   true,
	})
	require.Nil(t, err)
	a := assert.New(t)
	comps, ok := paramsMap["components"].(map[string]interface{})
	require.True(t, ok)
	base, ok := comps["base"].(map[string]interface{})
	require.True(t, ok)
	a.EqualValues("dev", base["env"])
	a.EqualValues("foobar", base["ns"])
	a.EqualValues("t1", base["tag"])
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
	a.Equal("", obj.GroupVersionKind().Group)
	a.Equal("v1", obj.GroupVersionKind().Version)
	a.Equal("ConfigMap", obj.GroupVersionKind().Kind)
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

func TestEvalComponentsEdges(t *testing.T) {
	goodComponents := []model.Component{
		{Name: "g1", File: "testdata/good-components/g1.jsonnet"},
		{Name: "g2", File: "testdata/good-components/g2.jsonnet"},
		{Name: "g3", File: "testdata/good-components/g3.jsonnet"},
		{Name: "g4", File: "testdata/good-components/g4.jsonnet"},
		{Name: "g5", File: "testdata/good-components/g5.jsonnet"},
	}
	goodAssert := func(t *testing.T, ret map[string]interface{}, err error) {
		require.NotNil(t, err)
	}
	tests := []struct {
		name        string
		components  []model.Component
		asserter    func(*testing.T, map[string]interface{}, error)
		concurrency int
	}{
		{
			name: "no components",
			asserter: func(t *testing.T, ret map[string]interface{}, err error) {
				require.Nil(t, err)
				assert.Equal(t, 0, len(ret))
			},
		},
		{
			name:       "single bad",
			components: []model.Component{{Name: "e1", File: "testdata/bad-components/e1.jsonnet"}},
			asserter: func(t *testing.T, ret map[string]interface{}, err error) {
				require.NotNil(t, err)
				assert.Contains(t, err.Error(), "evaluate 'e1'")
			},
		},
		{
			name: "two bad",
			components: []model.Component{
				{Name: "e1", File: "testdata/bad-components/e1.jsonnet"},
				{Name: "e2", File: "testdata/bad-components/e2.jsonnet"},
			},
			asserter: func(t *testing.T, ret map[string]interface{}, err error) {
				require.NotNil(t, err)
				assert.Contains(t, err.Error(), "evaluate 'e1'")
				assert.Contains(t, err.Error(), "evaluate 'e2'")
			},
		},
		{
			name: "many bad",
			components: []model.Component{
				{Name: "e1", File: "testdata/bad-components/e1.jsonnet"},
				{Name: "e2", File: "testdata/bad-components/e2.jsonnet"},
				{Name: "e3", File: "testdata/bad-components/e3.jsonnet"},
				{Name: "e4", File: "testdata/bad-components/e4.jsonnet"},
				{Name: "e5", File: "testdata/bad-components/e5.jsonnet"},
			},
			asserter: func(t *testing.T, ret map[string]interface{}, err error) {
				require.NotNil(t, err)
				assert.Contains(t, err.Error(), "... and 2 more errors")
			},
		},
		{
			name: "bad file",
			components: []model.Component{
				{Name: "e1", File: "testdata/bad-components/XXX.jsonnet"},
			},
			asserter: func(t *testing.T, ret map[string]interface{}, err error) {
				require.NotNil(t, err)
				assert.Contains(t, err.Error(), "no such file")
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
			ret, err := evalComponents(test.components, Context{
				Env:         "dev",
				Concurrency: test.concurrency,
			})
			test.asserter(t, ret, err)
		})
	}
}

func TestEvalComponentsBadJson(t *testing.T) {
	_, err := Components([]model.Component{
		{
			Name: "bad",
			File: "testdata/components/bad.json",
		},
	}, Context{Env: "dev"})
	require.NotNil(t, err)
	require.Contains(t, err.Error(), "invalid character")
}

func TestEvalComponentsBadYaml(t *testing.T) {
	_, err := Components([]model.Component{
		{
			Name: "bad",
			File: "testdata/components/bad.yaml",
		},
	}, Context{Env: "dev"})
	require.NotNil(t, err)
	require.Contains(t, err.Error(), "did not find expected node content")
}

func TestEvalComponentsBadObjects(t *testing.T) {
	_, err := Components([]model.Component{
		{
			Name: "bad",
			File: "testdata/components/bad-objects.yaml",
		},
	}, Context{Env: "dev"})
	require.NotNil(t, err)
	require.Contains(t, err.Error(), `unexpected type for object (string) at path "$.bad[0].foo"`)
}
