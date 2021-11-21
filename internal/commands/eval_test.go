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

package commands

import (
	"runtime"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestEvalBasic(t *testing.T) {
	s := newScaffold(t)
	defer s.reset()
	err := s.executeCommand("eval", "misc/simple.jsonnet")
	require.NoError(t, err)
	var data map[string]interface{}
	err = s.jsonOutput(&data)
	require.NoError(t, err)

	a := assert.New(t)
	a.Equal("str", data["foo"])
	a.Equal(true, data["bar"])
}

func TestEvalWithDataSources(t *testing.T) {
	s := newScaffold(t)
	defer s.reset()
	err := s.executeCommand("eval", "misc/simple-ds.xsonnet", "--vm:data-source", "exec://simple-ds?configVar=testKey", "--vm:ext-code",
		`testKey={ "command": "echo", "args": ["-n", "bar"] }`)
	require.NoError(t, err)
	var data map[string]interface{}
	err = s.jsonOutput(&data)
	require.NoError(t, err)

	a := assert.New(t)
	if runtime.GOOS == "windows" {
		a.Equal("-n bar\r\n", data["foo"])
	} else {
		a.Equal("bar", data["foo"])
	}
}

func TestEvalVars(t *testing.T) {
	s := newScaffold(t)
	defer s.reset()
	err := s.executeCommand("eval", "misc/vars.jsonnet", "--vm:ext-str", "foo=str", "--vm:ext-code", "bar=true")
	require.NoError(t, err)
	var data map[string]interface{}
	err = s.jsonOutput(&data)
	require.NoError(t, err)

	a := assert.New(t)
	a.Equal("str", data["foo"])
	a.Equal(true, data["bar"])
}

func TestEvalTLAs(t *testing.T) {
	s := newScaffold(t)
	defer s.reset()
	err := s.executeCommand("eval", "misc/tla.jsonnet", "--vm:tla-str", "foo=str", "--vm:tla-code", "bar=true")
	require.NoError(t, err)
	var data map[string]interface{}
	err = s.jsonOutput(&data)
	require.NoError(t, err)

	a := assert.New(t)
	a.Equal("str", data["foo"])
	a.Equal(true, data["bar"])
}

func TestEvalBasicYAML(t *testing.T) {
	s := newScaffold(t)
	defer s.reset()
	err := s.executeCommand("eval", "misc/simple.jsonnet", "--format=yaml")
	require.NoError(t, err)
	d, err := s.yamlOutput()
	require.NoError(t, err)
	require.Equal(t, 1, len(d))
	data, ok := d[0].(map[string]interface{})
	require.True(t, ok)
	a := assert.New(t)
	a.Equal("str", data["foo"])
	a.Equal(true, data["bar"])
}

func TestEvalInQBECContext(t *testing.T) {
	s := newScaffold(t)
	defer s.reset()
	err := s.executeCommand("eval", "misc/qbec.jsonnet", "--env=dev")
	require.NoError(t, err)
	var data map[string]interface{}
	err = s.jsonOutput(&data)
	a := assert.New(t)
	a.Equal("dev", data["foo"])
	a.Equal("development", data["bar"])
}

func TestEvalBadArgs(t *testing.T) {
	s := newScaffold(t)
	defer s.reset()
	err := s.executeCommand("eval", "misc/qbec.jsonnet", "misc/simple.jsonnet")
	require.Error(t, err)
	assert.Equal(t, "exactly one file required", err.Error())
}

func TestEvalBadFile(t *testing.T) {
	s := newScaffold(t)
	defer s.reset()
	err := s.executeCommand("eval", "non-existent.jsonnet")
	require.Error(t, err)
	assert.Equal(t, "non-existent.jsonnet: file not found", err.Error())
}

func TestEvalBadFileQbecCtx(t *testing.T) {
	s := newScaffold(t)
	defer s.reset()
	err := s.executeCommand("eval", "--env=dev", "non-existent.jsonnet")
	require.Error(t, err)
	assert.Equal(t, "non-existent.jsonnet: file not found", err.Error())
}
