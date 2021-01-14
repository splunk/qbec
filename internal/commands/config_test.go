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
	"bytes"
	"testing"

	"github.com/ghodss/yaml"
	"github.com/splunk/qbec/internal/model"
	"github.com/splunk/qbec/internal/remote"
	"github.com/splunk/qbec/internal/vm"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestConfigCreate(t *testing.T) {
	a := assert.New(t)
	fn := setPwd(t, "testdata")
	defer fn()
	app, err := model.NewApp("qbec.yaml", nil, "t1")
	require.NoError(t, err)
	rc := &remote.Config{}
	vs := vm.VariableSet{}.
		WithTopLevelVars(map[string]string{"tlaFoo": "xxx"}).
		WithTopLevelCodeVars(map[string]string{"tlaBar": "true"}).
		WithVars(map[string]string{"extFoo": "xxx"})
	vmc := vm.Config{VariableSet: vs}

	f := configFactory{
		skipConfirm:     true,
		colors:          true,
		evalConcurrency: 7,
		verbosity:       4,
		strictVars:      false,
		stdout:          bytes.NewBufferString(""),
		stderr:          bytes.NewBufferString(""),
	}

	cfg, err := f.getConfig(app, vmc, rc, forceOptions{}, nil)
	require.NoError(t, err)
	a.Equal(4, cfg.Verbosity())
	a.Equal(7, cfg.EvalConcurrency())
	a.Equal(app, cfg.App())
	a.True(cfg.Colorize())
	a.Equal("kube-system-t1", cfg.app.DefaultNamespace("dev"))
	a.Equal("default-t1", cfg.app.DefaultNamespace("prod"))
	a.Nil(cfg.Confirm("we will destroy you"))

	ctx := cfg.EvalContext("dev", map[string]interface{}{"foo": "bar"})
	a.Equal(cfg.EvalConcurrency(), ctx.Concurrency)
}

func TestConfigStrictVarsPass(t *testing.T) {
	fn := setPwd(t, "testdata")
	defer fn()
	app, err := model.NewApp("qbec.yaml", nil, "")
	require.NoError(t, err)
	rc := &remote.Config{}

	vs := vm.VariableSet{}.
		WithTopLevelVars(map[string]string{"tlaFoo": "xxx"}).
		WithCodeVars(map[string]string{"extFoo": "xxx", "extBar": "yyy", "noDefault": "boo"})
	vmc := vm.Config{VariableSet: vs}
	f := configFactory{
		strictVars: true,
	}
	_, err = f.getConfig(app, vmc, rc, forceOptions{}, nil)
	require.NoError(t, err)
}

func TestConfigStrictVarsFail(t *testing.T) {
	a := assert.New(t)
	fn := setPwd(t, "testdata")
	defer fn()
	app, err := model.NewApp("qbec.yaml", nil, "")
	require.NoError(t, err)
	rc := &remote.Config{}
	vs := vm.VariableSet{}.
		WithVars(map[string]string{"extSomething": "some-other-thing"}).
		WithTopLevelVars(map[string]string{"tlaGargle": "xxx"}).
		WithTopLevelCodeVars(map[string]string{"tlaBurble": "true"}).
		WithCodeVars(map[string]string{"extSomethingElse": "some-other-thing"})
	vmc := vm.Config{VariableSet: vs}
	f := configFactory{
		strictVars: true,
	}
	_, err = f.getConfig(app, vmc, rc, forceOptions{}, nil)
	require.NotNil(t, err)
	msg := err.Error()
	a.Contains(msg, "specified external variable 'extSomething' not declared for app")
	a.Contains(msg, "specified external variable 'extSomethingElse' not declared for app")
	a.Contains(msg, "declared external variable 'extFoo' not specfied for command")
	a.Contains(msg, "declared external variable 'extBar' not specfied for command")
	a.Contains(msg, "specified top level variable 'tlaGargle' not declared for app")
	a.Contains(msg, "specified top level variable 'tlaBurble' not declared for app")
	a.Contains(msg, "declared top level variable 'tlaFoo' not specfied for command")
}

func TestConfigConfirm(t *testing.T) {
	a := assert.New(t)
	fn := setPwd(t, "testdata")
	defer fn()
	app, err := model.NewApp("qbec.yaml", nil, "")
	require.NoError(t, err)
	rc := &remote.Config{}
	vmc := vm.Config{}

	var stdout, stderr bytes.Buffer
	stdin := bytes.NewReader([]byte("abcd\ny\n"))
	f := configFactory{
		skipConfirm: false,
		stdout:      &stdout,
		stderr:      &stderr,
	}
	cfg, err := f.getConfig(app, vmc, rc, forceOptions{}, nil)
	require.NoError(t, err)
	cfg.stdin = stdin
	err = cfg.Confirm("we will destroy you")
	require.NoError(t, err)

	cfg.stdin = bytes.NewReader([]byte(""))
	err = cfg.Confirm("we will destroy you")
	require.NotNil(t, err)
	a.Equal("failed to get user confirmation", err.Error())

	cfg.stdin = bytes.NewReader([]byte("n\n"))
	err = cfg.Confirm("we will destroy you")
	require.NotNil(t, err)
	a.Equal("canceled", err.Error())
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

func TestConfigConnectOpts(t *testing.T) {
	reset := setPwd(t, "testdata")
	defer reset()
	app, err := model.NewApp("qbec.yaml", nil, "")
	require.NoError(t, err)

	scp := stdClientProvider{
		app:       app,
		verbosity: 1,
	}
	co, err := scp.connectOpts("dev")
	require.NoError(t, err)
	assert.EqualValues(t, remote.ConnectOpts{
		EnvName:      "dev",
		ServerURL:    "https://dev-server",
		Namespace:    "kube-system",
		Verbosity:    1,
		ForceContext: "",
	}, co)

	scp = stdClientProvider{
		app:       app,
		verbosity: 1,
	}
	co, err = scp.connectOpts("minikube")
	require.NoError(t, err)
	assert.EqualValues(t, remote.ConnectOpts{
		EnvName:      "minikube",
		ServerURL:    "",
		Namespace:    "kube-public",
		Verbosity:    1,
		ForceContext: "minikube",
	}, co)

	scp = stdClientProvider{
		app:          app,
		verbosity:    2,
		forceContext: "kind",
	}
	co, err = scp.connectOpts("dev")
	require.NoError(t, err)
	assert.EqualValues(t, remote.ConnectOpts{
		EnvName:      "dev",
		ServerURL:    "https://dev-server",
		Namespace:    "kube-system",
		Verbosity:    2,
		ForceContext: "kind",
	}, co)

	scp = stdClientProvider{
		app:          app,
		verbosity:    2,
		forceContext: "kind",
	}
	co, err = scp.connectOpts("minikube")
	require.NoError(t, err)
	assert.EqualValues(t, remote.ConnectOpts{
		EnvName:      "minikube",
		ServerURL:    "",
		Namespace:    "kube-public",
		Verbosity:    2,
		ForceContext: "kind",
	}, co)
}
