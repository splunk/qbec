/*
   Copyright 2021 Splunk Inc.

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

package cmd

import (
	"testing"

	"github.com/splunk/qbec/internal/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestConfigNoStrictVarsPass(t *testing.T) {
	fn := setPwd(t, "testdata")
	defer fn()
	app, err := model.NewApp("qbec.yaml", nil, "")
	require.NoError(t, err)

	ctx := getContext(t, Options{}, []string{})
	ac, err := ctx.AppContext(app)
	require.NoError(t, err)
	a := assert.New(t)
	a.True(ac.vars.HasVar("extFoo"))
	a.True(ac.vars.HasVar("extBar"))
	a.False(ac.vars.HasVar("noDefault"))
}

func TestConfigStrictVarsPass(t *testing.T) {
	fn := setPwd(t, "testdata")
	defer fn()
	app, err := model.NewApp("qbec.yaml", nil, "")
	require.NoError(t, err)

	ctx := getContext(t, Options{}, []string{
		"--strict-vars",
		"--vm:ext-str=extFoo=xxx",
		"--vm:ext-str=extBar=yyy",
		"--vm:ext-str=noDefault=boo",
		"--vm:tla-str=tlaFoo=xxx",
	})
	_, err = ctx.AppContext(app)
	require.NoError(t, err)
}

func TestConfigStrictVarsFail(t *testing.T) {
	fn := setPwd(t, "testdata")
	defer fn()
	ctx := getContext(t, Options{}, []string{
		"--strict-vars",
		"--vm:ext-str=extSomething=some-other-thing",
		"--vm:ext-code=extSomethingElse=true",
		"--vm:tla-str=tlaGargle=xxx",
		"--vm:tla-code=tlaBurble=true",
	})
	app, err := model.NewApp("qbec.yaml", nil, "")
	require.NoError(t, err)

	_, err = ctx.AppContext(app)
	require.Error(t, err)
	msg := err.Error()
	a := assert.New(t)
	a.Contains(msg, "specified external variable 'extSomething' not declared for app")
	a.Contains(msg, "specified external variable 'extSomethingElse' not declared for app")
	a.Contains(msg, "declared external variable 'extFoo' not specfied for command")
	a.Contains(msg, "declared external variable 'extBar' not specfied for command")
	a.Contains(msg, "specified top level variable 'tlaGargle' not declared for app")
	a.Contains(msg, "specified top level variable 'tlaBurble' not declared for app")
	a.Contains(msg, "declared top level variable 'tlaFoo' not specfied for command")
}
