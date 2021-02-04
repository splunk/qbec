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
	"path/filepath"
	"strings"
	"sync"

	"github.com/pkg/errors"
	"github.com/spf13/cobra"
	"github.com/splunk/qbec/internal/sio"
	"k8s.io/client-go/discovery"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
)

// Constants for special context values.
const (
	ForceInClusterContext = "__incluster__"
)

// inspired by the config code in ksonnet but implemented differently.

// ConnectOpts are the connection options required for the config.
type ConnectOpts struct {
	EnvName      string // environment name, display purposes only
	ServerURL    string // the server URL to connect to, must be configured in the kubeconfig
	Namespace    string // the default namespace to set for the context
	Verbosity    int    // verbosity of client interactions
	ForceContext string // __incluster__ or __current or named context
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

func (c *Config) setupOverrides(opts ConnectOpts) error {
	if c.kubeconfig == nil {
		c.kubeconfig = clientcmd.NewNonInteractiveDeferredLoadingClientConfig(c.loadingRules, c.overrides)
	}
	rc, err := c.kubeconfig.RawConfig()
	if err != nil {
		return errors.Wrap(err, "raw Config from kubeconfig")
	}
	c.overrides.Context.Namespace = opts.Namespace

	overrideClusterForEnv := func() error {
		for name, cluster := range rc.Clusters {
			if cluster.Server == opts.ServerURL {
				sio.Noticeln("setting cluster to", name)
				c.overrides.Context.Cluster = name
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

	overrideCtx := func(wantCtx string) {
		c.overrides.CurrentContext = wantCtx
		c.overrides.Context.Cluster = rc.Contexts[wantCtx].Cluster
	}

	switch opts.ForceContext {
	case ForceInClusterContext:
		return fmt.Errorf("cannot set up overrides for in-cluster context")
	case "":
		if err := overrideClusterForEnv(); err != nil {
			return err
		}
	default: // assume named context
		wantCtx := opts.ForceContext
		if _, ok := rc.Contexts[wantCtx]; !ok {
			return fmt.Errorf("attempt to use context %s, but no such context was found", wantCtx)
		}
		sio.Warnf("force context %s\n", wantCtx)
		overrideCtx(wantCtx)
	}
	return nil
}

func (c *Config) getRESTConfig(opts ConnectOpts) (*rest.Config, error) {
	if opts.ForceContext == ForceInClusterContext {
		sio.Warnln("force in-cluster config")
		return rest.InClusterConfig()
	}
	if err := c.setupOverrides(opts); err != nil {
		return nil, err
	}
	restConfig, err := c.kubeconfig.ClientConfig()
	if err != nil {
		return nil, err
	}
	return restConfig, nil
}

// KubeAttributes is a collection k8s attributes pertaining to an connection.
type KubeAttributes struct {
	ConfigFile string `json:"configFile"` // the kubeconfig file or a list of such file separated by the list path separator
	Context    string `json:"context"`    // the context to use, if known
	Cluster    string `json:"cluster"`    // the cluster to use, always set
	Namespace  string `json:"namespace"`  // the nanespace to use
}

// KubeAttributes returns client attributes for the supplied connection options.
func (c *Config) KubeAttributes(opts ConnectOpts) (*KubeAttributes, error) {
	if err := c.setupOverrides(opts); err != nil {
		return nil, err
	}
	configFile := strings.Join(c.loadingRules.Precedence, string(filepath.ListSeparator))
	if c.loadingRules.ExplicitPath != "" {
		configFile = c.loadingRules.ExplicitPath
	}
	return &KubeAttributes{
		ConfigFile: configFile,
		Cluster:    c.overrides.Context.Cluster,
		Context:    c.overrides.CurrentContext,
		Namespace:  opts.Namespace,
	}, nil
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
	return newClient(newResourceClient(conf), disco, opts.Namespace, opts.Verbosity)
}

// ContextInfo has information we care about a K8s context
type ContextInfo struct {
	ContextName string // the name of the context
	ServerURL   string // the server URL defined for the cluster
	Namespace   string // the namespace if set for the context, else "default"
}

// CurrentContextInfo returns information for the current context found in kubeconfig.
func (c *Config) CurrentContextInfo() (*ContextInfo, error) {
	cc := clientcmd.NewNonInteractiveDeferredLoadingClientConfig(c.loadingRules, c.overrides)
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
		ContextName: kc.CurrentContext,
		ServerURL:   serverURL,
		Namespace:   ns,
	}, nil
}
