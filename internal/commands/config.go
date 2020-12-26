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

package commands

import (
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"strconv"
	"strings"

	"github.com/chzyer/readline"
	"github.com/pkg/errors"
	"github.com/spf13/cobra"
	"github.com/splunk/qbec/internal/eval"
	"github.com/splunk/qbec/internal/model"
	"github.com/splunk/qbec/internal/objsort"
	"github.com/splunk/qbec/internal/remote"
	"github.com/splunk/qbec/internal/sio"
	"github.com/splunk/qbec/internal/vm"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

// clientProvider returns a client for the supplied environment.
type clientProvider func(env string) (kubeClient, error)

type kubeAttrsProvider func(env string) (*remote.KubeAttributes, error)

// forceOptions are options that override qbec safety features and disregard
// configuration in qbec.yaml.
type forceOptions struct {
	k8sContext   string // override kubernetes context
	k8sNamespace string // override kubernetes default namespace
}

// addForceOptions adds flags to the supplied root command and returns forced options.
func addForceOptions(cmd *cobra.Command, prefix string) func() forceOptions {
	var f forceOptions
	ctxUsage := fmt.Sprintf("force K8s context with supplied value. Special values are %s and %s for in-cluster and current contexts respectively. Defaulted from QBEC_FORCE_K8S_CONTEXT",
		remote.ForceInClusterContext, currentMarker)
	pf := cmd.PersistentFlags()
	pf.StringVar(&f.k8sContext, prefix+"k8s-context", envOrDefault("QBEC_FORCE_K8S_CONTEXT", ""), ctxUsage)
	nsUsage := fmt.Sprintf("override default namespace for environment with supplied value. The special value %s can be used to extract the value in the kube config. Defaulted from QBEC_FORCE_K8S_NAMESPACE", currentMarker)
	pf.StringVar(&f.k8sNamespace, prefix+"k8s-namespace", envOrDefault("QBEC_FORCE_K8S_NAMESPACE", ""), nsUsage)
	return func() forceOptions { return f }
}

// stdClientProvider provides clients based on the supplied Kubernetes config
type stdClientProvider struct {
	app                    *model.App
	config                 *remote.Config
	verbosity              int
	forceContext           string
	overrideClientProvider func(env string) (kubeClient, error)
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
func (s stdClientProvider) Client(env string) (kubeClient, error) {
	if s.overrideClientProvider != nil {
		return s.overrideClientProvider(env)
	}
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

// configFactory provides a config.
type configFactory struct {
	stdout          io.Writer //standard output for command
	stderr          io.Writer // standard error for command
	skipConfirm     bool      // do not prompt for confirmation
	colors          bool      // show colorized output
	evalConcurrency int       // concurrency of eval operations
	verbosity       int       // verbosity level
	strictVars      bool      // strict mode for variable evaluation
}

func (cp configFactory) internalConfig(app *model.App, vmConfig vm.Config, clp clientProvider, kp kubeAttrsProvider) (*config, error) {
	var stdout io.Writer = os.Stdout
	var stderr io.Writer = os.Stderr

	if cp.stdout != nil {
		stdout = cp.stdout
	}
	if cp.stderr != nil {
		stderr = cp.stderr
	}
	cfg := &config{
		app:             app,
		vmc:             vmConfig,
		clp:             clp,
		attrsp:          kp,
		colors:          cp.colors,
		yes:             cp.skipConfirm,
		evalConcurrency: cp.evalConcurrency,
		verbose:         cp.verbosity,
		stdin:           os.Stdin,
		stdout:          stdout,
		stderr:          stderr,
	}
	if err := cfg.init(cp.strictVars); err != nil {
		return nil, err
	}
	return cfg, nil
}

// getConfig returns the command configuration.
func (cp configFactory) getConfig(app *model.App, vmConfig vm.Config, remoteConfig *remote.Config, forceOpts forceOptions,
	overrideCP func(env string) (kubeClient, error)) (*config, error) {
	scp := &stdClientProvider{
		app:                    app,
		config:                 remoteConfig,
		verbosity:              cp.verbosity,
		forceContext:           forceOpts.k8sContext,
		overrideClientProvider: overrideCP,
	}
	return cp.internalConfig(app, vmConfig, scp.Client, scp.Attrs)
}

// config is the command configuration.
type config struct {
	app             *model.App        // app loaded from file
	vmc             vm.Config         // jsonnet VM config
	tlaVars         map[string]string // all top level string vars specified for the command
	tlaCodeVars     map[string]string // all top level code vars specified for the command
	clp             clientProvider    // the client provider
	attrsp          kubeAttrsProvider // the kubernetes attribute provider
	colors          bool              // colorize output
	yes             bool              // auto-confirm
	evalConcurrency int               // concurrency of component eval
	verbose         int               // verbosity level
	stdin           io.Reader         // standard input
	stdout          io.Writer         // standard output
	stderr          io.Writer         // standard error
	cleanEvalMode   bool              // clean mode for eval
}

// init checks variables and sets up defaults. In strict mode, it requires all variables
// to be specified and does not allow undeclared variables to be passed in.
// It also sets the base VM config to include the library paths from the app definition
// and exclude all TLA variables. Required TLA variables are set per component later.
func (c *config) init(strict bool) error {
	var msgs []string
	c.tlaVars = c.vmc.TopLevelVars()
	c.tlaCodeVars = c.vmc.TopLevelCodeVars()
	c.vmc = c.vmc.WithLibPaths(c.app.LibPaths())

	vars := c.vmc.Vars()
	codeVars := c.vmc.CodeVars()

	declaredExternals := c.app.DeclaredVars()
	declaredTLAs := c.app.DeclaredTopLevelVars()

	checkStrict := func(tla bool, declared map[string]interface{}, varSources ...map[string]string) {
		kind := "external"
		if tla {
			kind = "top level"
		}
		// check that all specified variables have been declared
		for _, src := range varSources {
			for k := range src {
				_, ok := declared[k]
				if !ok {
					msgs = append(msgs, fmt.Sprintf("specified %s variable '%s' not declared for app", kind, k))
				}
			}
		}
		// check that all declared variables have been specified
		var fn func(string) bool
		if tla {
			fn = c.vmc.HasTopLevelVar
		} else {
			fn = c.vmc.HasVar
		}
		for k := range declared {
			ok := fn(k)
			if !ok {
				msgs = append(msgs, fmt.Sprintf("declared %s variable '%s' not specfied for command", kind, k))
			}
		}
	}

	if strict {
		checkStrict(false, declaredExternals, vars, codeVars)
		checkStrict(true, declaredTLAs, c.tlaVars, c.tlaCodeVars)
		if len(msgs) > 0 {
			return fmt.Errorf("strict vars check failures\n\t%s", strings.Join(msgs, "\n\t"))
		}
	}

	// apply default values for external vars
	addStrings, addCodes := map[string]string{}, map[string]string{}

	for k, v := range declaredExternals {
		if c.vmc.HasVar(k) {
			continue
		}
		if v == nil {
			sio.Warnf("no/ nil default specified for variable %q\n", k)
			continue
		}
		switch t := v.(type) {
		case string:
			addStrings[k] = t
		default:
			b, err := json.Marshal(v)
			if err != nil {
				return fmt.Errorf("json marshal: unexpected error marshaling default for variable %s, %v", k, err)
			}
			addCodes[k] = string(b)
		}
	}
	c.vmc = c.vmc.WithoutTopLevel().WithVars(addStrings).WithCodeVars(addCodes)
	return nil
}

// App returns the application object loaded for this run.
func (c config) App() *model.App { return c.app }

// EvalContext returns the evaluation context for the supplied environment.
func (c config) EvalContext(env string, props map[string]interface{}) eval.Context {
	p, err := json.Marshal(props)
	if err != nil {
		sio.Warnln("unable to serialize env properties to JSON:", err)
	}
	cm := "off"
	if c.cleanEvalMode {
		cm = "on"
	}
	baseConfig := c.vmc.WithVars(map[string]string{
		model.QbecNames.EnvVarName:       env,
		model.QbecNames.TagVarName:       c.app.Tag(),
		model.QbecNames.DefaultNsVarName: c.app.DefaultNamespace(env),
		model.QbecNames.CleanModeVarName: cm,
	}).WithCodeVars(map[string]string{
		model.QbecNames.EnvPropsVarName: string(p),
	})
	return eval.Context{
		VMConfig:        func(tlaVars []string) vm.Config { return c.vmConfig(baseConfig, tlaVars) },
		Verbose:         c.Verbosity() > 1,
		Concurrency:     c.EvalConcurrency(),
		PreProcessFile:  c.App().Preprocessor(),
		PostProcessFile: c.App().PostProcessor(),
	}
}

func (c config) ObjectProducer(env string) eval.LocalObjectProducer {
	return func(component string, data map[string]interface{}) model.K8sLocalObject {
		app := c.app
		return model.NewK8sLocalObject(data, model.LocalAttrs{
			App:               app.Name(),
			Tag:               app.Tag(),
			Component:         component,
			Env:               env,
			SetComponentLabel: app.AddComponentLabel(),
		})
	}
}

// vmConfig returns the VM configuration that only has the supplied top-level arguments.
func (c config) vmConfig(baseConfig vm.Config, tlaVars []string) vm.Config {
	cfg := baseConfig.WithoutTopLevel()

	// common case to avoid useless object creation. If no required vars
	// needed or none present, just return the config with empty TLAs
	if len(tlaVars) == 0 || (len(c.tlaVars) == 0 && len(c.tlaCodeVars) == 0) {
		return cfg
	}

	// else create a subset that match requirements
	check := map[string]bool{}
	for _, v := range tlaVars {
		check[v] = true
	}

	addStrs := map[string]string{}
	for k, v := range c.tlaVars {
		if check[k] {
			addStrs[k] = v
		}
	}
	addCodes := map[string]string{}
	for k, v := range c.tlaCodeVars {
		if check[k] {
			addCodes[k] = v
		}
	}
	return cfg.WithTopLevelVars(addStrs).WithTopLevelCodeVars(addCodes)
}

// Client returns a client for the supplied environment
func (c config) Client(env string) (kubeClient, error) {
	return c.clp(env)
}

// KubeAttributes returns the kubernetes attributes for the supplied environment
func (c config) KubeAttributes(env string) (*remote.KubeAttributes, error) {
	return c.attrsp(env)
}

// Colorize returns true if output needs to be colorized.
func (c config) Colorize() bool { return c.colors }

// Verbosity returns the log verbosity level
func (c config) Verbosity() int { return c.verbose }

// EvalConcurrency returns the concurrency to be used for evaluating components.
func (c config) EvalConcurrency() int { return c.evalConcurrency }

// Stdout returns the standard output configured for the command.
func (c config) Stdout() io.Writer {
	return c.stdout
}

// Stderr returns the standard error configured for the command.
func (c config) Stderr() io.Writer {
	return c.stderr
}

// Confirm prompts for confirmation if needed.
func (c config) Confirm(context string) error {
	fmt.Fprintln(c.stderr)
	fmt.Fprintln(c.stderr, context)
	fmt.Fprintln(c.stderr)
	if c.yes {
		return nil
	}
	inst, err := readline.NewEx(&readline.Config{
		Prompt:              "Do you want to continue [y/n]: ",
		Stdin:               ioutil.NopCloser(c.stdin),
		Stdout:              c.stdout,
		Stderr:              c.stderr,
		ForceUseInteractive: true,
	})
	if err != nil {
		return err
	}
	for {
		s, err := inst.Readline()
		if err != nil {
			if err == io.EOF {
				return errors.New("failed to get user confirmation")
			}
			return err
		}
		if s == "y" {
			return nil
		}
		if s == "n" {
			return errors.New("canceled")
		}
	}
}

func ordering(item model.K8sQbecMeta) int {
	a := item.GetAnnotations()
	if a == nil {
		return 0
	}
	v := a[model.QbecNames.Directives.ApplyOrder]
	if v == "" {
		return 0
	}
	val, err := strconv.Atoi(v)
	if err != nil {
		sio.Warnf("invalid apply order directive '%s' for %s, ignored\n", v, model.NameForDisplay(item))
		return 0
	}
	return val
}

// sortConfig returns the sort configuration.
func sortConfig(provider objsort.Namespaced) objsort.Config {
	return objsort.Config{
		NamespacedIndicator: func(gvk schema.GroupVersionKind) (bool, error) {
			ret, err := provider(gvk)
			if err != nil {
				return false, err
			}
			return ret, nil
		},
		OrderingProvider: ordering,
	}
}
