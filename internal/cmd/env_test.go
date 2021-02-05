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

func TestEnvContextBasic(t *testing.T) {
	a := assert.New(t)
	fn := setPwd(t, "testdata")
	defer fn()
	app, err := model.NewApp("qbec.yaml", nil, "")
	require.NoError(t, err)
	ctx := getContext(t, Options{}, []string{
		"--k8s:kubeconfig=kubeconfig.yaml",
	})
	ac, err := ctx.AppContext(app)
	require.NoError(t, err)
	ec, err := ac.EnvContext("dev")
	require.NoError(t, err)
	a.Equal("dev", ec.Env())
	ect := ec.EvalContext(true)

	a.True(ect.Vars.HasVar("qbec.io/env"))
	a.True(ect.Vars.HasVar("qbec.io/envProperties"))
	a.True(ect.Vars.HasVar("qbec.io/defaultNs"))
	a.True(ect.Vars.HasVar("qbec.io/tag"))

	attrs, err := ec.KubeAttributes()
	require.NoError(t, err)
	a.Equal("kube-system", attrs.Namespace)
	a.Equal("dev", attrs.Context)
	a.Equal("dev", attrs.Cluster)

	prod := ec.ObjectProducer()
	obj := prod("foo", map[string]interface{}{
		"apiVersion": "v1",
		"kind":       "ConfigMap",
		"metadata": map[string]interface{}{
			"name": "cm",
		},
		"data": map[string]interface{}{
			"foo": "bar",
		},
	})
	t.Log(obj)
}

func TestEnvContextForceContext(t *testing.T) {
	a := assert.New(t)
	fn := setPwd(t, "testdata")
	defer fn()
	app, err := model.NewApp("qbec.yaml", nil, "")
	require.NoError(t, err)
	ctx := getContext(t, Options{}, []string{
		"--k8s:kubeconfig=kubeconfig.yaml",
		"--force:k8s-context=__current__",
		"--force:k8s-namespace=__current__",
	})
	ac, err := ctx.AppContext(app)
	require.NoError(t, err)
	ec, err := ac.EnvContext("dev")
	require.NoError(t, err)
	f, err := ec.ForceOptions()
	require.NoError(t, err)
	a.Equal("barbaz", f.K8sNamespace)
	a.Equal("prod", f.K8sContext)
}

func TestEnvContextBadForceContext(t *testing.T) {
	a := assert.New(t)
	fn := setPwd(t, "testdata")
	defer fn()
	app, err := model.NewApp("qbec.yaml", nil, "")
	require.NoError(t, err)
	ctx := getContext(t, Options{}, []string{
		"--k8s:kubeconfig=kubeconfig_no_current.yaml",
		"--force:k8s-context=__current__",
	})
	ac, err := ctx.AppContext(app)
	require.NoError(t, err)
	_, err = ac.EnvContext("dev")
	require.Error(t, err)
	a.Equal("no current context set", err.Error())
}

func TestEnvContextBadForceNamespace(t *testing.T) {
	a := assert.New(t)
	fn := setPwd(t, "testdata")
	defer fn()
	app, err := model.NewApp("qbec.yaml", nil, "")
	require.NoError(t, err)
	ctx := getContext(t, Options{}, []string{
		"--k8s:kubeconfig=kubeconfig.yaml",
		"--force:k8s-namespace=__current__",
	})
	ac, err := ctx.AppContext(app)
	require.NoError(t, err)
	_, err = ac.EnvContext("dev")
	require.Error(t, err)
	a.Equal("current namespace can only be forced when the context is also forced to current", err.Error())
}
