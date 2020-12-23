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
	"bytes"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"text/template"
	"time"

	"github.com/ghodss/yaml"
	"github.com/splunk/qbec/internal/sio"
	"github.com/splunk/qbec/internal/testutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func setPwd(t *testing.T, dir string) func() {
	wd, err := os.Getwd()
	require.Nil(t, err)
	p, err := filepath.Abs(dir)
	require.Nil(t, err)
	err = os.Chdir(p)
	require.Nil(t, err)
	return func() {
		err = os.Chdir(wd)
		require.Nil(t, err)
	}
}

func TestDeepMerge(t *testing.T) {
	base := `
foo1: bar
foo2: [ 1, 2 ]
foo3: extra
inner:
  foo4: bar3
  foo5: 10
`
	override := `
foo1: bar-prime
foo2: [3,2,1]
foo10: extra
inner:
  foo0: zero
  foo4: bar4
`
	expected := `
foo1: bar-prime
foo2:
- 3
- 2
- 1
foo3: extra
foo10: extra
inner:
  foo0: zero
  foo4: bar4
  foo5: 10
`
	var baseData, overrideData map[string]interface{}
	err := yaml.Unmarshal([]byte(base), &baseData)
	require.NoError(t, err)
	err = yaml.Unmarshal([]byte(override), &overrideData)
	require.NoError(t, err)
	ret := deepMerge(baseData, overrideData)
	b, err := yaml.Marshal(ret)
	require.NoError(t, err)
	assert.Equal(t, strings.Trim(expected, "\n"), strings.Trim(string(b), "\n"))
}

func TestAppSimple(t *testing.T) {
	reset := setPwd(t, "../../examples/test-app")
	defer reset()
	app, err := NewApp("qbec.yaml", nil, "")
	require.Nil(t, err)
	a := assert.New(t)
	a.Equal("example1", app.Name())
	a.Equal(4, len(app.inner.Spec.Environments))
	a.Contains(app.inner.Spec.Environments, "dev")
	a.Contains(app.inner.Spec.Environments, "prod")
	a.Contains(app.inner.Spec.Environments, "stage")
	a.Equal(4, len(app.allComponents))
	a.Equal(3, len(app.defaultComponents))
	a.Contains(app.allComponents, "service2")
	a.NotContains(app.defaultComponents, "service2")
	a.Equal(false, app.AddComponentLabel())

	comps, err := app.ComponentsForEnvironment("_", nil, nil)
	require.Nil(t, err)
	require.Equal(t, 3, len(comps))
	a.Equal("cluster-objects", comps[0].Name)
	a.Equal("service1", comps[1].Name)
	a.Equal("test-job", comps[2].Name)

	comps, err = app.ComponentsForEnvironment("dev", nil, nil)
	require.Nil(t, err)
	require.Equal(t, 3, len(comps))
	a.Equal("cluster-objects", comps[0].Name)
	a.Equal("service2", comps[1].Name)
	a.Equal("test-job", comps[2].Name)

	comps, err = app.ComponentsForEnvironment("prod", nil, nil)
	require.Nil(t, err)
	require.Equal(t, 4, len(comps))
	a.Equal("cluster-objects", comps[0].Name)
	a.Equal("service1", comps[1].Name)
	a.Equal("service2", comps[2].Name)
	a.Equal("test-job", comps[3].Name)

	comps, err = app.ComponentsForEnvironment("dev", nil, []string{"service2"})
	require.Nil(t, err)
	require.Equal(t, 2, len(comps))
	a.Equal("cluster-objects", comps[0].Name)
	a.Equal("test-job", comps[1].Name)

	comps, err = app.ComponentsForEnvironment("dev", []string{"service2"}, nil)
	require.Nil(t, err)
	require.Equal(t, 1, len(comps))
	a.Equal("service2", comps[0].Name)

	comps, err = app.ComponentsForEnvironment("dev", []string{"service1"}, nil)
	require.Nil(t, err)
	require.Equal(t, 0, len(comps))

	a.EqualValues(map[string]interface{}{
		"externalFoo": "bar",
	}, app.DeclaredVars())

	a.EqualValues(map[string]interface{}{
		"tlaFoo": true,
	}, app.DeclaredTopLevelVars())

	u, err := app.ServerURL("dev")
	require.Nil(t, err)
	a.Equal("https://dev-server", u)

	u, err = app.ServerURL("local")
	require.Nil(t, err)
	a.Equal("", u)

	u, err = app.Context("dev")
	require.Nil(t, err)
	a.Equal("", u)

	u, err = app.Context("local")
	require.Nil(t, err)
	a.Equal("minikube", u)

	a.Equal("default", app.DefaultNamespace("dev"))
	a.Equal("", app.Tag())

	app.SetOverrideNamespace("")
	a.Equal("default", app.DefaultNamespace("dev"))
	app.SetOverrideNamespace("foobar")
	a.Equal("foobar", app.DefaultNamespace("dev"))

	_, err = app.ServerURL("devx")
	require.NotNil(t, err)
	a.Equal(`invalid environment "devx"`, err.Error())

	_, err = app.Context("devx")
	require.NotNil(t, err)
	a.Equal(`invalid environment "devx"`, err.Error())

	checkBase := func(props map[string]interface{}) {
		require.NotNil(t, props)
		a.Equal(3, len(props))
		a.Equal("no-override", props["core"])
		a.Equal("unknown", props["envType"])
		extra := props["extra"].(map[string]interface{})
		a.Equal(2, len(extra))
		a.Equal("bar", extra["foo"])
		a.Equal("baz", extra["bar"])
	}

	props := app.BaseProperties()
	checkBase(props)

	props, err = app.Properties("dev")
	require.NoError(t, err)
	require.NotNil(t, props)
	a.Equal(3, len(props))
	a.Equal("no-override", props["core"])
	a.Equal("development", props["envType"])
	extra := props["extra"].(map[string]interface{})
	a.Equal(2, len(extra))
	a.Equal("baz", extra["foo"])
	a.Equal(nil, extra["bar"])

	props, err = app.Properties("prod")
	require.NoError(t, err)
	require.NotNil(t, props)
	t.Log(props)
	a.Equal(3, len(props))
	a.Equal("no-override", props["core"])
	a.Equal("prod", props["envType"])
	extra = props["extra"].(map[string]interface{})
	a.Equal(2, len(extra))
	a.Equal("bar", extra["foo"])
	a.Equal("baz", extra["bar"])

	props, err = app.Properties("_")
	require.NoError(t, err)
	checkBase(props)

	props, err = app.Properties("stage")
	require.NoError(t, err)
	checkBase(props)

	_, err = app.Properties("foo")
	require.Error(t, err)

	a.Equal("params.libsonnet", app.ParamsFile())
	a.Equal("pp.jsonnet", app.PostProcessor())
	a.EqualValues([]string{"lib"}, app.LibPaths())

	envs := app.Environments()
	a.Equal(4, len(envs))
}

func TestAppWarnings(t *testing.T) {
	o, c := sio.Output, sio.ColorsEnabled()
	defer func() {
		sio.Output = o
		sio.EnableColors(c)
	}()
	sio.EnableColors(false)
	reset := setPwd(t, "./testdata/bad-app")
	defer reset()
	a := assert.New(t)

	buf := bytes.NewBuffer(nil)
	sio.Output = buf
	app, err := NewApp("app-warn.yaml", []string{"envs/override-dev.yaml"}, "foobar")
	require.Nil(t, err)
	a.Contains(buf.String(), "[warn] override env definition 'dev' from file dev2.yaml (previous: inline)")
	a.Contains(buf.String(), "[warn] override env definition 'dev' from file envs/override-dev.yaml (previous: dev2.yaml)")

	buf = bytes.NewBuffer(nil)
	sio.Output = buf
	comps, err := app.ComponentsForEnvironment("dev", nil, nil)
	require.Nil(t, err)
	a.Equal(2, len(comps))
	a.Contains(buf.String(), "component b included from dev is already included by default")

	buf = bytes.NewBuffer(nil)
	sio.Output = buf
	_, err = app.ComponentsForEnvironment("prod", nil, nil)
	require.Nil(t, err)
	a.Contains(buf.String(), "[warn] component a excluded from prod is already excluded by default")

	a.Equal("foobar", app.Tag())
	a.Equal("default-foobar", app.DefaultNamespace("dev"))

	a.EqualValues(map[string]interface{}{}, app.BaseProperties())
}

func TestAppComponentLoadSubdirs(t *testing.T) {
	reset := setPwd(t, "testdata/subdir-app")
	defer reset()
	app, err := NewApp("qbec.yaml", nil, "")
	require.Nil(t, err)
	comps, err := app.ComponentsForEnvironment("dev", nil, nil)
	require.Nil(t, err)
	a := assert.New(t)
	a.Equal(2, len(comps))
	comp := comps[0]
	a.Equal("comp1", comp.Name)
	a.Equal(1, len(comp.Files))
	a.Contains(comp.Files, filepath.Join("components", "comp1", "index.jsonnet"))

	comp = comps[1]
	a.Equal("comp2", comp.Name)
	a.Equal(3, len(comp.Files))
	a.Contains(comp.Files, filepath.Join("components", "comp2", "cm1.yaml"))
	a.Contains(comp.Files, filepath.Join("components", "comp2", "cm2.json"))
	a.Contains(comp.Files, filepath.Join("components", "comp2", "index.yaml"))
}

func TestAppComponentLoadNegative(t *testing.T) {
	reset := setPwd(t, "../../examples/test-app")
	defer reset()
	app, err := NewApp("qbec.yaml", nil, "")
	require.Nil(t, err)
	a := assert.New(t)

	_, err = app.ComponentsForEnvironment("boo", nil, nil)
	require.NotNil(t, err)
	a.Equal(`invalid environment "boo"`, err.Error())

	_, err = app.ComponentsForEnvironment("dev", []string{"d"}, nil)
	require.NotNil(t, err)
	a.Equal(`specified components: bad component reference(s): d`, err.Error())

	_, err = app.ComponentsForEnvironment("dev", nil, []string{"d"})
	require.NotNil(t, err)
	a.Equal(`specified components: bad component reference(s): d`, err.Error())

	_, err = app.ComponentsForEnvironment("dev", []string{"a"}, []string{"b"})
	require.NotNil(t, err)
	a.Equal(`cannot include as well as exclude components, specify one or the other`, err.Error())
}

func TestHttpEnvFiles(t *testing.T) {
	reset := setPwd(t, "testdata/http-app")
	defer reset()
	s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		b, err := ioutil.ReadFile("envs.yaml")
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		_, _ = w.Write(b)
	}))
	defer s.Close()

	b, err := ioutil.ReadFile("qbec-template.yaml")
	tmpl, err := template.New("qbec").Parse(string(b))
	require.NoError(t, err)
	var buf bytes.Buffer
	err = tmpl.Execute(&buf, map[string]interface{}{"URL": s.URL})
	require.NoError(t, err)
	err = ioutil.WriteFile("qbec.yaml", buf.Bytes(), 0640)
	require.NoError(t, err)
	app, err := NewApp("qbec.yaml", nil, "")
	require.NoError(t, err)
	envs := app.Environments()
	a := assert.New(t)
	a.Equal(3, len(envs))
	a.Contains(envs, "stage")
	a.Contains(envs, "prod")
	require.Contains(t, envs, "dev")
	a.EqualValues("https://new-dev-server", envs["dev"].Server)
}

func TestAppNegative(t *testing.T) {
	reset := setPwd(t, "./testdata/bad-app")
	defer reset()

	tests := []struct {
		tag      string
		file     string
		envFiles []string
		asserter func(t *testing.T, err error)
	}{
		{
			file: "non-existent.yaml",
			asserter: func(t *testing.T, err error) {
				assert.Contains(t, err.Error(), testutil.FileNotFoundMessage)
			},
		},
		{
			file: "bad-yaml.yaml",
			asserter: func(t *testing.T, err error) {
				assert.Contains(t, err.Error(), "converting YAML to JSON")
			},
		},
		{
			file: "bad-comp-exclude.yaml",
			asserter: func(t *testing.T, err error) {
				assert.Contains(t, err.Error(), "default exclusions: bad component reference(s): d")
			},
		},
		{
			file: "bad-env-exclude.yaml",
			asserter: func(t *testing.T, err error) {
				assert.Contains(t, err.Error(), "dev exclusions: bad component reference(s): d")
			},
		},
		{
			file: "bad-env-include.yaml",
			asserter: func(t *testing.T, err error) {
				assert.Contains(t, err.Error(), "dev inclusions: bad component reference(s): d")
			},
		},
		{
			file: "bad-env-include-exclude.yaml",
			asserter: func(t *testing.T, err error) {
				assert.Contains(t, err.Error(), "env dev: component c present in both include and exclude sections")
			},
		},
		{
			file: "bad-baseline-env.yaml",
			asserter: func(t *testing.T, err error) {
				assert.Contains(t, err.Error(), "cannot use _ as an environment name since it has a special meaning")
			},
		},
		{
			file: "bad-comps.yaml",
			asserter: func(t *testing.T, err error) {
				assert.Contains(t, err.Error(), fmt.Sprintf("duplicate component a, found %s and %s", filepath.FromSlash("bad-comps/a.json"), filepath.FromSlash("bad-comps/a.yaml")))
			},
		},
		{
			file: "bad-app-name.yaml",
			asserter: func(t *testing.T, err error) {
				assert.Contains(t, err.Error(), "metadata.name in body should match")
			},
		},
		{
			file: "bad-env-name.yaml",
			asserter: func(t *testing.T, err error) {
				assert.Contains(t, err.Error(), "invalid environment foo/bar, must match")
			},
		},
		{
			file: "bad-dup-tla.yaml",
			asserter: func(t *testing.T, err error) {
				assert.Contains(t, err.Error(), "duplicate top-level variable foo")
			},
		},
		{
			file: "bad-dup-ext.yaml",
			asserter: func(t *testing.T, err error) {
				assert.Contains(t, err.Error(), "duplicate external variable foo")
			},
		},
		{
			file: "app-warn.yaml",
			tag:  "-foobar",
			asserter: func(t *testing.T, err error) {
				assert.Contains(t, err.Error(), "invalid tag name '-foobar', must match")
			},
		},
		{
			file: "bad-no-envs.yaml",
			asserter: func(t *testing.T, err error) {
				assert.Contains(t, err.Error(), "no environments defined for app")
			},
		},
		{
			file: "bad-missing-env-file.yaml",
			asserter: func(t *testing.T, err error) {
				assert.Contains(t, err.Error(), "missing-env.yaml: "+testutil.FileNotFoundMessage)
			},
		},
		{
			file: "bad-malformed-env-file.yaml",
			asserter: func(t *testing.T, err error) {
				assert.Contains(t, err.Error(), "malformed-env.yaml: unmarshal YAML")
			},
		},
		{
			file: "bad-invalid-env-file.yaml",
			asserter: func(t *testing.T, err error) {
				assert.Contains(t, err.Error(), "invalid-env.yaml, 1 schema validation error(s): spec.foo in body is a forbidden property")
			},
		},
		{
			file:     "app-warn.yaml",
			envFiles: []string{"foobar.yaml"},
			asserter: func(t *testing.T, err error) {
				assert.Contains(t, err.Error(), "open foobar.yaml")
			},
		},
		{
			file: "bad-env-config.yaml",
			asserter: func(t *testing.T, err error) {
				assert.Contains(t, err.Error(), "verify environment foo: neither server nor context was set")
			},
		},
		{
			file: "bad-env-config2.yaml",
			asserter: func(t *testing.T, err error) {
				assert.Contains(t, err.Error(), "verify environment foo: only one of server or context may be set")
			},
		},
		{
			file: "bad-env-config3.yaml",
			asserter: func(t *testing.T, err error) {
				assert.Contains(t, err.Error(), "verify environment foo: context for environment ('__current__') may not start with __")
			},
		},
	}

	for _, test := range tests {
		t.Run(test.file, func(t *testing.T) {
			_, err := NewApp(test.file, test.envFiles, test.tag)
			require.NotNil(t, err)
			test.asserter(t, err)
		})
	}
}

func TestNegativeDownload(t *testing.T) {
	t.Run("no-endpoint", func(t *testing.T) {
		_, err := readEnvFile("http://nonexistent.server")
		require.Error(t, err)
		assert.Contains(t, err.Error(), "download environments from http://nonexistent.server")
	})
	t.Run("bad-status", func(t *testing.T) {
		s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusBadRequest)
		}))
		defer s.Close()
		_, err := readEnvFile(s.URL)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "download environments from "+s.URL)
		assert.Contains(t, err.Error(), "status : 400 Bad Request")
	})
	t.Run("slow-server", func(t *testing.T) {
		s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			time.Sleep(time.Second)
			w.WriteHeader(http.StatusBadRequest)
		}))
		defer s.Close()
		o := httpClient
		defer func() { httpClient = o }()
		httpClient = &http.Client{Timeout: 100 * time.Millisecond}
		_, err := readEnvFile(s.URL)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "download environments from "+s.URL)
		assert.Contains(t, err.Error(), "context deadline exceeded")
	})
}

func TestAppLabel(t *testing.T) {
	reset := setPwd(t, "testdata/label-app")
	defer reset()
	app, err := NewApp("qbec.yaml", nil, "")
	require.Nil(t, err)
	a := assert.New(t)
	a.Equal(true, app.AddComponentLabel())
}
