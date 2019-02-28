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

// Package remote has the client implementation for interrogating and updating K8s objects and their metadata.
package remote

import (
	"fmt"
	"sync"

	"github.com/pkg/errors"
	"github.com/spf13/cobra"
	"github.com/splunk/qbec/internal/sio"
	"k8s.io/client-go/discovery"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
)

// inspired by the config code in ksonnet but implemented differently.

// ConnectOpts are the connection options required for the config.
type ConnectOpts struct {
	EnvName   string // environment name, display purposes only
	ServerURL string // the server URL to connect to, must be configured in the kubeconfig
	Namespace string // the default namespace to set for the context
	Verbosity int    // verbosity of client interactions
}

// Config provides clients for specific contexts out of a kubeconfig file, with overrides for auth.
type Config struct {
	loadingRules *clientcmd.ClientConfigLoadingRules
	overrides    *clientcmd.ConfigOverrides
	l            sync.Mutex
	kubeconfig   clientcmd.ClientConfig
}

// NewConfig returns a new configuration, adding flags to the supplied command to set k8s access overrides, prefixed by
// the supplied string.
func NewConfig(cmd *cobra.Command, prefix string) *Config {
	loadingRules := clientcmd.NewDefaultClientConfigLoadingRules()
	overrides := &clientcmd.ConfigOverrides{}
	cmd.PersistentFlags().StringVar(&loadingRules.ExplicitPath, prefix+"kubeconfig", "", "Path to a kubeconfig file. Alternative to env var $KUBECONFIG.")
	clientcmd.BindOverrideFlags(overrides, cmd.PersistentFlags(), clientcmd.ConfigOverrideFlags{
		AuthOverrideFlags: clientcmd.RecommendedAuthOverrideFlags(prefix),
		Timeout: clientcmd.FlagInfo{
			LongName:    prefix + clientcmd.FlagTimeout,
			Default:     "0",
			Description: "The length of time to wait before giving up on a single server request. Non-zero values should contain a corresponding time unit (e.g. 1s, 2m, 3h). A value of zero means don't timeout requests."},
	})
	return &Config{
		loadingRules: loadingRules,
		overrides:    overrides,
	}
}

func (c *Config) getRESTConfig(opts ConnectOpts) (*rest.Config, error) {
	if c.kubeconfig == nil {
		c.kubeconfig = clientcmd.NewNonInteractiveDeferredLoadingClientConfig(c.loadingRules, c.overrides)
	}
	if err := c.overrideCluster(c.kubeconfig, opts); err != nil {
		return nil, err
	}
	restConfig, err := c.kubeconfig.ClientConfig()
	if err != nil {
		return nil, err
	}
	return restConfig, nil
}

func (c *Config) overrideCluster(kc clientcmd.ClientConfig, opts ConnectOpts) error {
	rc, err := kc.RawConfig()
	if err != nil {
		return errors.Wrap(err, "raw Config from kubeconfig")
	}
	for name, cluster := range rc.Clusters {
		if cluster.Server == opts.ServerURL {
			sio.Noticeln("setting cluster to", name)
			c.overrides.Context.Cluster = name
			c.overrides.Context.Namespace = opts.Namespace
			for contextName, ctx := range rc.Contexts {
				if ctx.Cluster == name {
					sio.Noticeln("setting context to", contextName)
					c.overrides.CurrentContext = contextName
				}
			}
			return nil
		}
	}
	return fmt.Errorf("unable to find any cluster with URL %q  (for env %s) in the kube config", opts.ServerURL, opts.EnvName)
}

// Client returns a client that correctly points to the server as specified in the connection options.
// For this to work correctly, the kubernetes config that is used *must* have a cluster that has the supplied
// server URL as an endpoint, so that correct TLS certs are used for authenticating the server.
func (c *Config) Client(opts ConnectOpts) (*Client, error) {
	c.l.Lock()
	defer c.l.Unlock()
	conf, err := c.getRESTConfig(opts)
	if err != nil {
		return nil, err
	}

	disco, err := discovery.NewDiscoveryClientForConfig(conf)
	if err != nil {
		return nil, err
	}

	discoCache := newCachedDiscoveryClient(disco)
	mapper := discovery.NewDeferredDiscoveryRESTMapper(discoCache, dynamic.VersionInterfaces)
	pathResolver := dynamic.LegacyAPIPathResolverFunc
	pool := dynamic.NewClientPool(conf, mapper, pathResolver)
	return newClient(pool, disco, opts.Namespace, opts.Verbosity)
}

// ContextInfo has information we care about a K8s context
type ContextInfo struct {
	ServerURL string // the server URL defined for the cluster
	Namespace string // the namespace if set for the context, else "default"
}

// CurrentContextInfo returns information for the current context found in kubeconfig.
func CurrentContextInfo() (*ContextInfo, error) {
	loadingRules := clientcmd.NewDefaultClientConfigLoadingRules()
	overrides := &clientcmd.ConfigOverrides{}
	cc := clientcmd.NewNonInteractiveDeferredLoadingClientConfig(loadingRules, overrides)
	kc, err := cc.RawConfig()
	if err != nil {
		return nil, err
	}
	if kc.CurrentContext == "" {
		return nil, fmt.Errorf("no current context set")
	}
	var cluster string
	ns := "default"
	for name, ctx := range kc.Contexts {
		if name == kc.CurrentContext {
			if ctx.Namespace != "" {
				ns = ctx.Namespace
			}
			cluster = ctx.Cluster
		}
	}
	if cluster == "" {
		return nil, fmt.Errorf("no cluster found for context %s", kc.CurrentContext)
	}
	var serverURL string
	for cname, clusterInfo := range kc.Clusters {
		if cluster == cname {
			serverURL = clusterInfo.Server
		}
	}
	if serverURL == "" {
		return nil, fmt.Errorf("unable to find server URL for cluster %s", cluster)
	}
	return &ContextInfo{
		ServerURL: serverURL,
		Namespace: ns,
	}, nil
}
