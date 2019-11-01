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
	app, err := model.NewApp("qbec.yaml", "t1")
	require.Nil(t, err)
	rc := &remote.Config{}
	vmc := vm.Config{}

	vmc = vmc.WithTopLevelVars(map[string]string{"tlaFoo": "xxx"})
	vmc = vmc.WithTopLevelCodeVars(map[string]string{"tlaBar": "true"})
	vmc = vmc.WithVars(map[string]string{"extFoo": "xxx"})

	f := ConfigFactory{
		SkipConfirm:     true,
		Colors:          true,
		EvalConcurrency: 7,
		Verbosity:       4,
		StrictVars:      false,
		Stdout:          bytes.NewBufferString(""),
		Stderr:          bytes.NewBufferString(""),
	}

	cfg, err := f.Config(app, vmc, rc)
	require.Nil(t, err)
	a.Equal(4, cfg.Verbosity())
	a.Equal(7, cfg.EvalConcurrency())
	a.Equal(app, cfg.App())
	a.True(cfg.Colorize())
	a.Equal("kube-system-t1", cfg.app.DefaultNamespace("dev"))
	a.Equal("default-t1", cfg.app.DefaultNamespace("prod"))
	a.Nil(cfg.Confirm("we will destroy you"))

	ctx := cfg.EvalContext("dev")
	a.Equal("app1", ctx.App)
	a.Equal("dev", ctx.Env)
	a.Equal("t1", ctx.Tag)
	a.Equal("kube-system-t1", ctx.DefaultNs)
	a.Equal(cfg.EvalConcurrency(), ctx.Concurrency)

	testVMC := ctx.VMConfig([]string{"tlaFoo", "tlaBar"})
	a.EqualValues(map[string]string{"tlaFoo": "xxx"}, testVMC.TopLevelVars())
	a.EqualValues(map[string]string{"tlaBar": "true"}, testVMC.TopLevelCodeVars())
	a.EqualValues(map[string]string{"extFoo": "xxx"}, testVMC.Vars())
	a.EqualValues(map[string]string{"extBar": `{"bar":"quux"}`}, testVMC.CodeVars())
}

func TestConfigStrictVarsPass(t *testing.T) {
	fn := setPwd(t, "testdata")
	defer fn()
	app, err := model.NewApp("qbec.yaml", "")
	require.Nil(t, err)
	rc := &remote.Config{}
	vmc := vm.Config{}

	vmc = vmc.WithTopLevelVars(map[string]string{"tlaFoo": "xxx"})
	vmc = vmc.WithCodeVars(map[string]string{"extFoo": "xxx", "extBar": "yyy", "noDefault": "boo"})

	f := ConfigFactory{
		StrictVars: true,
	}

	_, err = f.Config(app, vmc, rc)
	require.Nil(t, err)
}

func TestConfigStrictVarsFail(t *testing.T) {
	a := assert.New(t)
	fn := setPwd(t, "testdata")
	defer fn()
	app, err := model.NewApp("qbec.yaml", "")
	require.Nil(t, err)
	rc := &remote.Config{}
	vmc := vm.Config{}

	vmc = vmc.WithTopLevelVars(map[string]string{"tlaGargle": "xxx"})
	vmc = vmc.WithVars(map[string]string{"extSomething": "some-other-thing"})
	vmc = vmc.WithTopLevelCodeVars(map[string]string{"tlaBurble": "true"})
	vmc = vmc.WithCodeVars(map[string]string{"extSomethingElse": "some-other-thing"})

	f := ConfigFactory{
		StrictVars: true,
	}

	_, err = f.Config(app, vmc, rc)
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
	app, err := model.NewApp("qbec.yaml", "")
	require.Nil(t, err)
	rc := &remote.Config{}
	vmc := vm.Config{}

	var stdout, stderr bytes.Buffer
	stdin := bytes.NewReader([]byte("abcd\ny\n"))
	f := ConfigFactory{
		SkipConfirm: false,
		Stdout:      &stdout,
		Stderr:      &stderr,
	}
	cfg, err := f.Config(app, vmc, rc)
	require.Nil(t, err)
	cfg.stdin = stdin
	err = cfg.Confirm("we will destroy you")
	require.Nil(t, err)

	cfg.stdin = bytes.NewReader([]byte(""))
	err = cfg.Confirm("we will destroy you")
	require.NotNil(t, err)
	a.Equal("failed to get user confirmation", err.Error())

	cfg.stdin = bytes.NewReader([]byte("n\n"))
	err = cfg.Confirm("we will destroy you")
	require.NotNil(t, err)
	a.Equal("canceled", err.Error())
}
