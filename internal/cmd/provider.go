// Copyright 2025 Splunk Inc.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package cmd

import (
	"fmt"

	"github.com/pkg/errors"
	"github.com/spf13/cobra"
	"github.com/splunk/qbec/internal/model"
	"github.com/splunk/qbec/internal/remote"
)

const (
	currentMarker = "__current__"
)

// ForceOptions are options that override qbec safety features and disregard
// configuration in qbec.yaml.
type ForceOptions struct {
	K8sContext   string // override kubernetes context
	K8sNamespace string // override kubernetes default namespace
}

// addForceOptions adds flags to the supplied root command and returns forced options.
func addForceOptions(cmd *cobra.Command, cfg *remote.Config, prefix string) func() (ForceOptions, error) {
	var forceOpts ForceOptions
	ctxUsage := fmt.Sprintf("force K8s context with supplied value. Special values are %s and %s for in-cluster and current contexts respectively. Defaulted from QBEC_FORCE_K8S_CONTEXT",
		remote.ForceInClusterContext, currentMarker)
	pf := cmd.PersistentFlags()
	pf.StringVar(&forceOpts.K8sContext, prefix+"k8s-context", envOrDefault("QBEC_FORCE_K8S_CONTEXT", ""), ctxUsage)
	nsUsage := fmt.Sprintf("override default namespace for environment with supplied value. The special value %s can be used to extract the value in the kube config. Defaulted from QBEC_FORCE_K8S_NAMESPACE", currentMarker)
	pf.StringVar(&forceOpts.K8sNamespace, prefix+"k8s-namespace", envOrDefault("QBEC_FORCE_K8S_NAMESPACE", ""), nsUsage)
	return func() (ForceOptions, error) {
		var cc *remote.ContextInfo
		var err error
		if forceOpts.K8sContext == currentMarker {
			cc, err = cfg.CurrentContextInfo()
			if err != nil {
				return forceOpts, err
			}
			forceOpts.K8sContext = cc.ContextName
		}
		if forceOpts.K8sNamespace == currentMarker {
			if cc == nil {
				return forceOpts, NewUsageError("current namespace can only be forced when the context is also forced to current")
			}
			forceOpts.K8sNamespace = cc.Namespace
		}
		return forceOpts, nil
	}
}

// stdClientProvider provides clients based on the supplied Kubernetes config
type stdClientProvider struct {
	app          *model.App
	config       *remote.Config
	verbosity    int
	forceContext string
}

func (s stdClientProvider) connectOpts(env string) (ret remote.ConnectOpts, _ error) {
	server, err := s.app.ServerURL(env)
	if err != nil {
		return ret, err
	}
	fc, err := s.app.Context(env)
	if err != nil {
		return ret, err
	}
	// override with command-line forcing if supplied
	if s.forceContext != "" {
		fc = s.forceContext
	}
	ns := s.app.DefaultNamespace(env)
	return remote.ConnectOpts{
		EnvName:      env,
		ServerURL:    server,
		Namespace:    ns,
		ForceContext: fc,
		Verbosity:    s.verbosity,
	}, nil
}

// Client returns a client for the supplied environment.
func (s stdClientProvider) Client(env string) (KubeClient, error) {
	opts, err := s.connectOpts(env)
	if err != nil {
		return nil, errors.Wrap(err, "get client")
	}
	rem, err := s.config.Client(opts)
	if err != nil {
		return nil, err
	}
	return rem, nil
}

func (s stdClientProvider) Attrs(env string) (*remote.KubeAttributes, error) {
	opts, err := s.connectOpts(env)
	if err != nil {
		return nil, errors.Wrap(err, "get kubernetes attrs")
	}
	rem, err := s.config.KubeAttributes(opts)
	if err != nil {
		return nil, err
	}
	return rem, nil
}
