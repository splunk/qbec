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

package remote

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"k8s.io/client-go/tools/clientcmd"
)

var (
	mainKubeConfig = filepath.Join("testdata", "kube", "config.yaml")
)

func TestConfigResolution(t *testing.T) {
	tests := []struct {
		name       string
		kubeconfig string
		opts       ConnectOpts
		assertFn   func(c *Config, err error)
	}{
		{
			name:       "basic no overrides",
			kubeconfig: mainKubeConfig,
			opts: ConnectOpts{
				EnvName:   "first",
				ServerURL: "https://dev1-server",
				Namespace: "firstns",
			},
			assertFn: func(c *Config, err error) {
				require.Nil(t, err)
				assert.Equal(t, "dev1", c.overrides.CurrentContext)
				assert.Equal(t, "dev1", c.overrides.Context.Cluster)
				assert.Equal(t, "firstns", c.overrides.Context.Namespace)
			},
		},
		{
			name:       "in-cluster context",
			kubeconfig: mainKubeConfig,
			opts: ConnectOpts{
				EnvName:      "first",
				ServerURL:    "https://dev1-server",
				ForceContext: "__incluster__",
			},
			assertFn: func(c *Config, err error) {
				require.NotNil(t, err)
				assert.Equal(t, "cannot set up overrides for in-cluster context", err.Error())
			},
		},
		{
			name:       "non-existent cluster",
			kubeconfig: mainKubeConfig,
			opts: ConnectOpts{
				EnvName:   "first",
				ServerURL: "https://xxx-server",
			},
			assertFn: func(c *Config, err error) {
				require.NotNil(t, err)
				assert.Equal(t, `unable to find any cluster with URL "https://xxx-server"  (for env first) in the kube config`, err.Error())
			},
		},
		{
			name:       "force named context",
			kubeconfig: mainKubeConfig,
			opts: ConnectOpts{
				EnvName:      "first",
				ServerURL:    "https://xxx-server",
				ForceContext: "dev2",
				Namespace:    "xxx",
			},
			assertFn: func(c *Config, err error) {
				require.Nil(t, err)
				assert.Equal(t, "dev2", c.overrides.CurrentContext)
				assert.Equal(t, "dev2", c.overrides.Context.Cluster)
				assert.Equal(t, "xxx", c.overrides.Context.Namespace)
			},
		},
		{
			name:       "bad named context",
			kubeconfig: mainKubeConfig,
			opts: ConnectOpts{
				EnvName:      "first",
				ServerURL:    "https://xxx-server",
				ForceContext: "garbage",
				Namespace:    "xxx",
			},
			assertFn: func(c *Config, err error) {
				require.NotNil(t, err)
				assert.Equal(t, `attempt to use context garbage, but no such context was found`, err.Error())
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			os.Setenv("KUBECONFIG", test.kubeconfig)
			defer os.Unsetenv("KUBECONFIG")
			c := &Config{
				loadingRules: clientcmd.NewDefaultClientConfigLoadingRules(),
				overrides:    &clientcmd.ConfigOverrides{},
			}
			err := c.setupOverrides(test.opts)
			test.assertFn(c, err)
		})
	}
}
