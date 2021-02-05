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
	"github.com/splunk/qbec/internal/remote"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

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
