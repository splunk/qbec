package externals

import (
	"os"
	"runtime"
	"testing"

	"github.com/spf13/cobra"
	"github.com/splunk/qbec/internal/testutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func getCR() string {
	cr := "\n"
	if runtime.GOOS == "windows" {
		cr = "\r\n"
	}
	return cr
}

func TestExternals(t *testing.T) {
	var fn func() (Externals, error)
	var cfg Externals
	cmd := &cobra.Command{
		Use: "show",
		RunE: func(c *cobra.Command, args []string) error {
			var err error
			cfg, err = fn()
			if err != nil {
				return err
			}
			return nil
		},
	}
	cmd.SilenceUsage = true
	cmd.SilenceErrors = true
	fn = FromCommandParams(cmd, "vm:", false)
	cmd.SetArgs([]string{
		"show",
		"--vm:ext-str=extStr",
		"--vm:ext-code-file=extCode=testdata/extCode.libsonnet",
		"--vm:tla-str=tlaStr=tlafoo",
		"--vm:tla-code=tlaCode=true",
		"--vm:jpath=testdata/lib1",
		"--vm:ext-str-list=testdata/vars.txt",
		"--vm:data-source=exec://foobar?configVar=extCode",
	})
	os.Setenv("extStr", "envFoo")
	defer os.Unsetenv("extStr")
	os.Setenv("listVar2", "l2")
	defer os.Unsetenv("listVar2")
	err := cmd.Execute()
	require.Nil(t, err)
	assert.EqualValues(t, []string{"testdata/lib1"}, cfg.LibPaths)
	assert.EqualValues(t, []string{"exec://foobar?configVar=extCode"}, cfg.DataSources)
	assert.EqualValues(t, map[string]UserVal{
		"extStr":   {Value: "envFoo"},
		"extCode":  {Value: `{ foo: 'ec1foo', bar: 'ec1bar'}` + getCR(), Code: true},
		"listVar1": {Value: "l1"},
		"listVar2": {Value: "l2"},
	}, cfg.Variables.Vars)
	assert.EqualValues(t, map[string]UserVal{
		"tlaStr":  {Value: "tlafoo"},
		"tlaCode": {Value: `true`, Code: true},
	}, cfg.Variables.TopLevelVars)
}

func TestConfigShorthands(t *testing.T) {
	var fn func() (Externals, error)
	var cfg Externals
	cmd := &cobra.Command{
		Use: "show",
		RunE: func(c *cobra.Command, args []string) error {
			var err error
			cfg, err = fn()
			if err != nil {
				return err
			}
			cfg = cfg.WithLibPaths([]string{"testdata/lib2"})
			return nil
		},
	}
	cmd.SilenceUsage = true
	cmd.SilenceErrors = true
	fn = FromCommandParams(cmd, "vm:", true)
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
	assert.EqualValues(t, []string{"testdata/lib1", "testdata/lib2"}, cfg.LibPaths)
	assert.EqualValues(t, map[string]UserVal{
		"extStr":   {Value: "envFoo"},
		"extCode":  {Value: `{ foo: 'ec1foo', bar: 'ec1bar'}` + getCR(), Code: true},
		"listVar1": {Value: "l1"},
		"listVar2": {Value: "l2"},
	}, cfg.Variables.Vars)
	assert.EqualValues(t, map[string]UserVal{
		"tlaStr":  {Value: "tlafoo"},
		"tlaCode": {Value: `true`, Code: true},
	}, cfg.Variables.TopLevelVars)
}

func TestConfigNegative(t *testing.T) {
	execInVM := func(code string, args []string) error {
		var fn func() (Externals, error)
		cmd := &cobra.Command{
			Use: "show",
			RunE: func(c *cobra.Command, args []string) error {
				var err error
				_, err = fn()
				return err
			},
		}
		fn = FromCommandParams(cmd, "vm:", false)
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
