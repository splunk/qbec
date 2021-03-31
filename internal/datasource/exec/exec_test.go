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

package exec

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"runtime"
	"testing"

	"github.com/splunk/qbec/internal/datasource/api"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestExecBasic(t *testing.T) {
	exe, err := exec.LookPath("qbec-replay-exec")
	if err != nil {
		t.SkipNow()
	}
	a := assert.New(t)
	pwd, err := os.Getwd()
	require.NoError(t, err)
	var tests = []struct {
		inherit bool
	}{
		{true}, {false},
	}
	for _, test := range tests {
		t.Run(fmt.Sprintf("inhert_%t", test.inherit), func(t *testing.T) {
			ds := New("replay", "var1")
			os.Setenv("_akey_", "aval")
			defer os.Unsetenv("_akey_")
			err = ds.Init(func(name string) (string, error) {
				if name != "var1" {
					return "", fmt.Errorf("invalid call to config provider, want %q got %q", "var1", name)
				}
				return fmt.Sprintf(`
{
	"command": "qbec-replay-exec",
	"args": [ "one", "two" ],
	"env": {
		"foo": "bar"
	},
	"stdin": "input",
	"inheritEnv": %t
}
`, test.inherit), nil
			})
			require.NoError(t, err)
			defer ds.Close()
			a.Equal("replay", ds.Name())
			str, err := ds.Resolve("/foo/bar")
			require.NoError(t, err)
			var data struct {
				DSName  string   `json:"dsName"`
				Command string   `json:"command"`
				Args    []string `json:"args"`
				Dir     string   `json:"dir"`
				Env     []string `json:"env"`
				Input   string   `json:"stdin"`
			}
			err = json.Unmarshal([]byte(str), &data)
			require.NoError(t, err)
			a.Equal("replay", data.DSName)
			a.Equal(exe, data.Command)
			a.EqualValues([]string{"one", "two"}, data.Args)
			a.Equal(pwd, data.Dir)
			a.Contains(data.Env, "foo=bar")
			if test.inherit {
				a.Contains(data.Env, "_akey_=aval")
			} else {
				a.NotContains(data.Env, "_akey_=aval")
			}
			a.Contains(data.Env, "__DS_NAME__=replay")
			a.Contains(data.Env, "__DS_PATH__=/foo/bar")
			a.Equal("input", data.Input)
		})
	}

}

func TestExecRelativeFilePath(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("not running exec bit tests on windows")
	}
	ds := New("replay", "var1")
	err := ds.Init(func(name string) (string, error) {
		c := Config{Command: "testdata/exec-bit-set.sh"}
		b, _ := json.Marshal(c)
		return string(b), nil
	})
	require.NoError(t, err)
	defer ds.Close()
	s, err := ds.Resolve("/")
	require.NoError(t, err)
	assert.Equal(t, "{}\n", s)
}

func TestExecNegative(t *testing.T) {
	if _, err := exec.LookPath("qbec-replay-exec"); err != nil {
		t.SkipNow()
	}
	tests := []struct {
		name         string
		config       Config
		path         string
		cp           api.ConfigProvider
		skipWindows  bool
		initAsserter func(t *testing.T, err error)
		asserter     func(t *testing.T, resolved string, err error)
	}{
		{
			name:   "bad-exe",
			config: Config{Command: "non-existent"},
			initAsserter: func(t *testing.T, err error) {
				require.Error(t, err)
				assert.Contains(t, err.Error(), `init data source replay: invalid command 'non-existent'`)
			},
		},
		{
			name:        "bad-exec-bit",
			config:      Config{Command: "./testdata/exec-bit-not-set.sh"},
			skipWindows: true,
			initAsserter: func(t *testing.T, err error) {
				require.Error(t, err)
				assert.Contains(t, err.Error(), `init data source replay: invalid command './testdata/exec-bit-not-set.sh'`)
			},
		},
		{
			name:   "exe-err",
			path:   "/fail",
			config: Config{Command: "qbec-replay-exec"},
			asserter: func(t *testing.T, resolved string, err error) {
				require.Error(t, err)
				assert.Contains(t, err.Error(), `exit status 1`)
			},
		},
		{
			name:   "exe-slow",
			path:   "/slow",
			config: Config{Command: "qbec-replay-exec", Timeout: "500ms"},
			asserter: func(t *testing.T, resolved string, err error) {
				require.Error(t, err)
				if runtime.GOOS == "windows" {
					assert.Contains(t, err.Error(), `exit status 1`)
				} else {
					assert.Contains(t, err.Error(), `signal: killed`)
				}
			},
		},
		{
			name:   "no-command",
			config: Config{},
			initAsserter: func(t *testing.T, err error) {
				require.Error(t, err)
				assert.Contains(t, err.Error(), `command not specified`)
			},
		},
		{
			name:   "bad-timeout",
			config: Config{Command: "qbec-exec-replay", Timeout: "abc"},
			initAsserter: func(t *testing.T, err error) {
				require.Error(t, err)
				assert.Contains(t, err.Error(), `init data source replay: invalid timeout 'abc'`)
			},
		},
		{
			name:   "cp-not-found",
			config: Config{Command: "qbec-exec-replay"},
			cp:     func(name string) (string, error) { return "", fmt.Errorf("NOT FOUND") },
			initAsserter: func(t *testing.T, err error) {
				require.Error(t, err)
				assert.Contains(t, err.Error(), `init data source replay: NOT FOUND`)
			},
		},
		{
			name:   "cp-bad-json",
			config: Config{Command: "qbec-exec-replay"},
			cp:     func(name string) (string, error) { return "{", nil },
			initAsserter: func(t *testing.T, err error) {
				require.Error(t, err)
				assert.Contains(t, err.Error(), `init data source replay: unexpected end of JSON input`)
			},
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if test.skipWindows && runtime.GOOS == "windows" {
				t.SkipNow()
			}
			ds := New("replay", "c")
			cp := func(name string) (string, error) {
				b, _ := json.Marshal(test.config)
				return string(b), nil
			}
			if test.cp != nil {
				cp = test.cp
			}
			err := ds.Init(cp)
			if test.initAsserter == nil {
				require.NoError(t, err)
			} else {
				test.initAsserter(t, err)
			}
			if err != nil {
				return
			}
			defer ds.Close()
			p := test.path
			if p == "" {
				p = "/"
			}
			ret, err := ds.Resolve(p)
			if test.asserter != nil {
				test.asserter(t, ret, err)
			}
		})
	}
}
