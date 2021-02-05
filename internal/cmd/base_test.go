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

package cmd

import (
	"bytes"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestContextCreateSimple(t *testing.T) {
	a := assert.New(t)
	fn := setPwd(t, "testdata")
	defer fn()
	ctx := getContext(t, Options{}, []string{})
	a.False(ctx.strictVars)
	a.Equal(0, ctx.Verbosity())
	a.Equal("", ctx.AppTag())
	a.Equal("", ctx.RootDir())
	a.Nil(ctx.EnvFiles())
	a.Equal(0, ctx.EvalConcurrency())
	a.Equal(os.Stdout, ctx.Stdout())
	a.Equal(os.Stdin, ctx.stdin)
	a.Equal(os.Stderr, ctx.Stderr())
}

func TestContextCreate(t *testing.T) {
	a := assert.New(t)
	fn := setPwd(t, "testdata")
	defer fn()
	os.Setenv("QBEC_ROOT", "testdata2")
	defer os.Unsetenv("QBEC_ROOT")
	ctx := getContext(t, Options{}, []string{
		"--app-tag=t1",
		"--colors",
		"--env-file=testdata/extra-env.yaml",
		"--eval-concurrency=7",
		"--k8s:kubeconfig=./kubeconfig.yaml",
		"--force:k8s-context=minikube",
		"--force:k8s-namespace=ns1",
		"--root=testdata",
		"--strict-vars",
		"--verbose=25",
		"--vm:ext-code=foo=bar",
		"--vm:jpath=lib",
		"--yes",
	})
	a.True(ctx.Colorize())
	a.True(ctx.strictVars)
	a.Equal(25, ctx.Verbosity())
	a.Equal("t1", ctx.AppTag())
	a.Equal("testdata", ctx.RootDir())
	a.Equal([]string{"testdata/extra-env.yaml"}, ctx.EnvFiles())
	a.Equal(7, ctx.EvalConcurrency())
	a.Equal(os.Stdout, ctx.Stdout())
	a.Equal(os.Stdin, ctx.stdin)
	a.Equal(os.Stderr, ctx.Stderr())

	f, err := ctx.ForceOptions()
	require.NoError(t, err)
	a.Equal("minikube", f.K8sContext)
	a.Equal("ns1", f.K8sNamespace)

	ec := ctx.BasicEvalContext()
	a.EqualValues([]string{"lib"}, ec.LibPaths)
	v := ec.Vars
	a.True(v.HasVar("foo"))

	info, err := ctx.KubeContextInfo()
	require.NoError(t, err)
	a.Equal("barbaz", info.Namespace)
	a.Equal("prod", info.ContextName)
	a.Equal("https://prod-server", info.ServerURL)

	ac, err := ctx.AppContext(nil)
	require.NoError(t, err)
	a.True(ac.App() == nil)

	err = ctx.Confirm("foo")
	require.NoError(t, err)
}

func TestContextBadExt(t *testing.T) {
	a := assert.New(t)
	fn := setPwd(t, "testdata")
	defer fn()
	err := getBadContext(t, Options{}, []string{
		"-V", "non-existent-env-var",
	})
	a.Contains(err.Error(), "no value found from environment for non-existent-env-var")
}

func TestContextBadProfile(t *testing.T) {
	a := assert.New(t)
	fn := setPwd(t, "testdata")
	defer fn()
	err := getBadContext(t, Options{}, []string{
		"--pprof:cpu", "non-existent-dir/file.pprof",
	})
	a.Contains(err.Error(), "init profiler")
}

func TestContextConfirm(t *testing.T) {
	a := assert.New(t)
	fn := setPwd(t, "testdata")
	defer fn()
	var stdout, stderr bytes.Buffer
	ctx := getContext(t, Options{
		Stdout: &stdout,
		Stderr: &stderr,
	}, []string{})

	stdin := bytes.NewReader([]byte("abcd\ny\n"))
	ctx.stdin = stdin
	err := ctx.Confirm("we will destroy you")
	require.NoError(t, err)

	ctx.stdin = bytes.NewReader([]byte(""))
	err = ctx.Confirm("we will destroy you")
	require.NotNil(t, err)
	a.Equal("failed to get user confirmation", err.Error())

	ctx.stdin = bytes.NewReader([]byte("n\n"))
	err = ctx.Confirm("we will destroy you")
	require.NotNil(t, err)
	a.Equal("canceled", err.Error())
}
