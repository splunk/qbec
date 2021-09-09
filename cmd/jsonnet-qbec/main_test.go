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

package main

import (
	"encoding/json"
	"os"
	"os/exec"
	"testing"

	"github.com/splunk/qbec/internal/vmexternals"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func runConfig() string {
	return `
{
	command: 'qbec-replay-exec',
	args: [
		'arg1',
		'arg2',
	],
	env: {
		foo: 'bar',
	},
	stdin: '{ "bar": "baz" }',
	timeout: '3s',
}
`
}

func shouldSkip() bool {
	_, err := exec.LookPath("qbec-replay-exec")
	return err != nil
}

func TestExecDataSource(t *testing.T) {
	if shouldSkip() {
		t.Skipf("program 'qbec-replay-exec' not found on path, skipping test")
	}
	p, _ := exec.LookPath("qbec-replay-exec")
	wd, err := os.Getwd()
	require.NoError(t, err)
	ext := vmexternals.Externals{
		Variables: vmexternals.UserVariables{
			Vars: map[string]vmexternals.UserVal{
				"runConfig": {
					Value: runConfig(),
					Code:  true,
				},
			},
		},
		LibPaths: []string{},
		DataSources: []string{
			"exec://replay?configVar=runConfig",
		},
	}
	s, err := run("testdata/basic.jsonnet", ext)
	require.NoError(t, err)
	t.Logf("output was:\n%s\n", s)

	var output map[string]interface{}
	err = json.Unmarshal([]byte(s), &output)
	require.NoError(t, err)

	a := assert.New(t)
	a.Equal(p, output["command"])
	a.Equal(wd, output["dir"])
	a.EqualValues([]interface{}{"arg1", "arg2"}, output["args"])
	a.Equal("replay", output["dsName"])
	a.Contains(output["env"], "__DS_NAME__=replay")
	a.Contains(output["env"], "__DS_PATH__=/test/path")
	a.Contains(output["env"], "foo=bar")
}

func TestExecDataSourceFail(t *testing.T) {
	if shouldSkip() {
		t.Skipf("program 'qbec-replay-exec' not found on path, skipping test")
	}
	ext := vmexternals.Externals{
		Variables: vmexternals.UserVariables{
			Vars: map[string]vmexternals.UserVal{
				"runConfig": {
					Value: runConfig(),
					Code:  true,
				},
			},
		},
		DataSources: []string{
			"exec://replay?configVar=runConfig",
		},
	}
	_, err := run("testdata/fail.jsonnet", ext)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "RUNTIME ERROR: data source replay, target=/fail:")
}

func TestExecDataSourceTimeout(t *testing.T) {
	if shouldSkip() {
		t.Skipf("program 'qbec-replay-exec' not found on path, skipping test")
	}
	ext := vmexternals.Externals{
		Variables: vmexternals.UserVariables{
			Vars: map[string]vmexternals.UserVal{
				"runConfig": {
					Value: runConfig(),
					Code:  true,
				},
			},
		},
		DataSources: []string{
			"exec://replay?configVar=runConfig",
		},
	}
	_, err := run("testdata/slow.jsonnet", ext)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "RUNTIME ERROR: data source replay, target=/slow")
}
