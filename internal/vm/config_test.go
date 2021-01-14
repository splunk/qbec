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
package vm

import (
	"encoding/json"
	"os"
	"testing"

	"github.com/spf13/cobra"
	"github.com/splunk/qbec/internal/testutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type code struct {
	Foo string `json:"foo"`
	Bar string `json:"bar"`
}

type result struct {
	TLAStr     string `json:"tlaStr"`
	TLACode    bool   `json:"tlaCode"`
	ExtStr     string `json:"extStr"`
	ExtCode    code   `json:"extCode"`
	LibPath1   code   `json:"libpath1"`
	LibPath2   code   `json:"libpath2"`
	InlineStr  string `json:"inlineStr"`
	InlineCode bool   `json:"inlineCode"`
	ListVar1   string `json:"listVar1"`
	ListVar2   string `json:"listVar2"`
}

var evalCode = `
function (tlaStr,tlaCode) {
	tlaStr: tlaStr,
	tlaCode: tlaCode,
	extStr: std.extVar('extStr'),
	extCode: std.extVar('extCode'),
	libPath1: import 'libcode1.libsonnet',
	libPath2: import 'libcode2.libsonnet',
	inlineStr: std.extVar('inlineStr'),
	inlineCode: std.extVar('inlineCode'),
	listVar1: std.extVar('listVar1'),
	listVar2: std.extVar('listVar2'),
}
`

func TestConfigBasic(t *testing.T) {
	var fn func() (Config, error)
	var cfg Config
	var output string
	cmd := &cobra.Command{
		Use: "show",
		RunE: func(c *cobra.Command, args []string) error {
			var err error
			cfg, err = fn()
			if err != nil {
				return err
			}
			cfg := cfg.WithLibPaths([]string{"testdata/lib2"})
			vars := cfg.VariableSet.WithVars(map[string]string{"inlineStr": "ifoo"}).
				WithCodeVars(map[string]string{"inlineCode": "true"})
			jvm := newJsonnetVM(cfg.LibPaths)
			vars.register(jvm)
			output, err = jvm.EvaluateSnippet("test.jsonnet", evalCode)
			return err
		},
	}
	cmd.SilenceUsage = true
	cmd.SilenceErrors = true
	fn = ConfigFromCommandParams(cmd, "vm:", false)
	cmd.SetArgs([]string{
		"show",
		"--vm:ext-str=extStr",
		"--vm:ext-code-file=extCode=testdata/extCode.libsonnet",
		"--vm:tla-str=tlaStr=tlafoo",
		"--vm:tla-code=tlaCode=true",
		"--vm:jpath=testdata/lib1",
		"--vm:ext-str-list=testdata/vars.txt",
	})
	os.Setenv("extStr", "envFoo")
	defer os.Unsetenv("extStr")
	os.Setenv("listVar2", "l2")
	defer os.Unsetenv("listVar2")
	err := cmd.Execute()
	require.Nil(t, err)
	var r result
	err = json.Unmarshal([]byte(output), &r)
	require.Nil(t, err)
	assert.EqualValues(t, result{
		TLAStr:     "tlafoo",
		TLACode:    true,
		ExtStr:     "envFoo",
		ExtCode:    code{Foo: "ec1foo", Bar: "ec1bar"},
		LibPath1:   code{Foo: "lc1foo", Bar: "lc1bar"},
		LibPath2:   code{Foo: "lc2foo", Bar: "lc2bar"},
		InlineStr:  "ifoo",
		InlineCode: true,
		ListVar1:   "l1",
		ListVar2:   "l2",
	}, r)
}

func TestConfigShorthands(t *testing.T) {
	var fn func() (Config, error)
	var cfg Config
	var output string
	cmd := &cobra.Command{
		Use: "show",
		RunE: func(c *cobra.Command, args []string) error {
			var err error
			cfg, err = fn()
			if err != nil {
				return err
			}
			cfg = cfg.WithLibPaths([]string{"testdata/lib2"})
			vars := cfg.VariableSet.
				WithVars(map[string]string{"inlineStr": "ifoo"}).
				WithCodeVars(map[string]string{"inlineCode": "true"})
			jvm := newJsonnetVM(cfg.LibPaths)
			vars.register(jvm)
			output, err = jvm.EvaluateSnippet("test.jsonnet", evalCode)
			return err
		},
	}
	cmd.SilenceUsage = true
	cmd.SilenceErrors = true
	fn = ConfigFromCommandParams(cmd, "vm:", true)
	cmd.SetArgs([]string{
		"show",
		"-V",
		"extStr",
		"--vm:ext-code-file=extCode=testdata/extCode.libsonnet",
		"-A",
		"tlaStr=tlafoo",
		"--vm:tla-code=tlaCode=true",
		"--vm:jpath=testdata/lib1",
		"--vm:ext-str-list=testdata/vars.txt",
	})
	os.Setenv("extStr", "envFoo")
	defer os.Unsetenv("extStr")
	os.Setenv("listVar2", "l2")
	defer os.Unsetenv("listVar2")
	err := cmd.Execute()
	require.Nil(t, err)
	var r result
	err = json.Unmarshal([]byte(output), &r)
	require.Nil(t, err)
	assert.EqualValues(t, result{
		TLAStr:     "tlafoo",
		TLACode:    true,
		ExtStr:     "envFoo",
		ExtCode:    code{Foo: "ec1foo", Bar: "ec1bar"},
		LibPath1:   code{Foo: "lc1foo", Bar: "lc1bar"},
		LibPath2:   code{Foo: "lc2foo", Bar: "lc2bar"},
		InlineStr:  "ifoo",
		InlineCode: true,
		ListVar1:   "l1",
		ListVar2:   "l2",
	}, r)
}

func TestConfigNegative(t *testing.T) {
	execInVM := func(code string, args []string) error {
		var fn func() (Config, error)
		cmd := &cobra.Command{
			Use: "show",
			RunE: func(c *cobra.Command, args []string) error {
				var err error
				cfg, err := fn()
				if err != nil {
					return err
				}
				jvm := newJsonnetVM(cfg.LibPaths)
				cfg.VariableSet.register(jvm)
				if code == "" {
					code = "{}"
				}
				_, err = jvm.EvaluateSnippet("test.jsonnet", code)
				return err
			},
		}
		fn = ConfigFromCommandParams(cmd, "vm:", false)
		cmd.SetArgs(args)
		cmd.SilenceUsage = true
		cmd.SilenceErrors = true
		return cmd.Execute()
	}
	tests := []struct {
		name     string
		code     string
		args     []string
		asserter func(a *assert.Assertions, err error)
	}{
		{
			name: "ext-str-undef",
			args: []string{"show", "--vm:ext-str=undef_foo"},
			asserter: func(a *assert.Assertions, err error) {
				require.NotNil(t, err)
				a.Contains(err.Error(), "no value found from environment for undef_foo")
			},
		},
		{
			name: "ext-code-undef",
			args: []string{"show", "--vm:ext-code=undef_foo"},
			asserter: func(a *assert.Assertions, err error) {
				require.NotNil(t, err)
				a.Contains(err.Error(), "no value found from environment for undef_foo")
			},
		},
		{
			name: "tla-str-undef",
			args: []string{"show", "--vm:tla-str=undef_foo"},
			asserter: func(a *assert.Assertions, err error) {
				require.NotNil(t, err)
				a.Contains(err.Error(), "no value found from environment for undef_foo")
			},
		},
		{
			name: "tla-code-undef",
			args: []string{"show", "--vm:tla-code=undef_foo"},
			asserter: func(a *assert.Assertions, err error) {
				require.NotNil(t, err)
				a.Contains(err.Error(), "no value found from environment for undef_foo")
			},
		},
		{
			name: "ext-file-undef",
			args: []string{"show", "--vm:ext-str-file=foo"},
			asserter: func(a *assert.Assertions, err error) {
				require.NotNil(t, err)
				a.Contains(err.Error(), "ext-str-file no filename specified for foo")
			},
		},
		{
			name: "tla-file-undef",
			args: []string{"show", "--vm:tla-str-file=foo=bar"},
			asserter: func(a *assert.Assertions, err error) {
				require.NotNil(t, err)
				a.Contains(err.Error(), "open bar: "+testutil.FileNotFoundMessage)
			},
		},
		{
			name: "shorthand-not-enabled",
			args: []string{"show", "-A extStr"},
			asserter: func(a *assert.Assertions, err error) {
				require.NotNil(t, err)
				a.Contains(err.Error(), "unknown shorthand flag: 'A'")
			},
		},
		{
			name: "ext-list-bad-file",
			args: []string{"show", "--vm:ext-str-list=no-such-file"},
			asserter: func(a *assert.Assertions, err error) {
				require.NotNil(t, err)
				a.Contains(err.Error(), testutil.FileNotFoundMessage)
			},
		},
		{
			name: "ext-list-bad-file",
			args: []string{"show", "--vm:ext-str-list=testdata/vars.txt"},
			asserter: func(a *assert.Assertions, err error) {
				require.NotNil(t, err)
				a.Contains(err.Error(), "process list testdata/vars.txt, line 3: no value found from environment for listVar2")
			},
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			a := assert.New(t)
			err := execInVM(test.code, test.args)
			test.asserter(a, err)
		})
	}
}

func TestConfigFromScratch(t *testing.T) {
	vars := VariableSet{}.
		WithVars(map[string]string{"foo": "bar"}).
		WithCodeVars(map[string]string{"bar": "true"})
	jvm := newJsonnetVM(nil)
	vars.register(jvm)
	out, err := jvm.EvaluateSnippet("test.jsonnet", `std.extVar('foo') + std.toString(std.extVar('bar'))`)
	require.Nil(t, err)
	assert.Equal(t, `"bartrue"`+"\n", out)
}
