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
	"os"
	"strings"

	"github.com/chzyer/readline"
	"github.com/pkg/errors"
	"github.com/splunk/qbec/internal/eval"
	"github.com/splunk/qbec/internal/model"
	"github.com/splunk/qbec/internal/objsort"
	"github.com/splunk/qbec/internal/remote"
	"github.com/splunk/qbec/internal/sio"
	"github.com/splunk/qbec/internal/vm"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

// clientProvider returns a client for the supplied environment.
type clientProvider func(env string) (Client, error)

// stdClientProvider provides clients based on the supplied Kubernetes config
type stdClientProvider struct {
	app       *model.App
	config    *remote.Config
	verbosity int
}

// Client returns a client for the supplied environment.
func (s stdClientProvider) Client(env string) (Client, error) {
	server, err := s.app.ServerURL(env)
	if err != nil {
		return nil, errors.Wrap(err, "get client")
	}
	ns := s.app.DefaultNamespace(env)
	rem, err := s.config.Client(remote.ConnectOpts{
		EnvName:   env,
		ServerURL: server,
		Namespace: ns,
		Verbosity: s.verbosity,
	})
	if err != nil {
		return nil, err
	}
	return rem, nil
}

// ConfigFactory provides a config.
type ConfigFactory struct {
	Stdout          io.Writer //standard output for command
	Stderr          io.Writer // standard error for command
	SkipConfirm     bool      // do not prompt for confirmation
	Colors          bool      // show colorized output
	EvalConcurrency int       // concurrency of eval operations
	Verbosity       int       // verbosity level
	StrictVars      bool      // strict mode for variable evaluation
}

func (cp ConfigFactory) internalConfig(app *model.App, vmConfig vm.Config, clp clientProvider) (*Config, error) {
	var stdout io.Writer = os.Stdout
	var stderr io.Writer = os.Stderr

	if cp.Stdout != nil {
		stdout = cp.Stdout
	}
	if cp.Stderr != nil {
		stderr = cp.Stderr
	}

	cfg := &Config{
		app:             app,
		vmc:             vmConfig,
		clp:             clp,
		colors:          cp.Colors,
		yes:             cp.SkipConfirm,
		evalConcurrency: cp.EvalConcurrency,
		verbose:         cp.Verbosity,
		stdin:           os.Stdin,
		stdout:          stdout,
		stderr:          stderr,
	}
	if err := cfg.init(cp.StrictVars); err != nil {
		return nil, err
	}
	return cfg, nil
}

// Config returns the command configuration.
func (cp ConfigFactory) Config(app *model.App, vmConfig vm.Config, remoteConfig *remote.Config) (*Config, error) {
	scp := &stdClientProvider{
		app:       app,
		config:    remoteConfig,
		verbosity: cp.Verbosity,
	}
	return cp.internalConfig(app, vmConfig, scp.Client)
}

// Config is the command configuration.
type Config struct {
	app             *model.App        // app loaded from file
	vmc             vm.Config         // jsonnet VM config
	tlaVars         map[string]string // all top level string vars specified for the command
	tlaCodeVars     map[string]string // all top level code vars specified for the command
	clp             clientProvider    // the client provider
	colors          bool              // colorize output
	yes             bool              // auto-confirm
	evalConcurrency int               // concurrency of component eval
	verbose         int               // verbosity level
	stdin           io.Reader         // standard input
	stdout          io.Writer         // standard output
	stderr          io.Writer         // standard error
}

// init checks variables and sets up defaults. In strict mode, it requires all variables
// to be specified and does not allow undeclared variables to be passed in.
// It also sets the base VM config to include the library paths from the app definition
// and exclude all TLA variables. Require TLA variables are set per component later.
func (c *Config) init(strict bool) error {
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
func (c Config) App() *model.App { return c.app }

// EvalContext returns the evaluation context for the supplied environment.
func (c Config) EvalContext(env string) eval.Context {
	return eval.Context{
		App:         c.App().Name(),
		Tag:         c.App().Tag(),
		Env:         env,
		DefaultNs:   c.app.DefaultNamespace(env),
		VMConfig:    c.vmConfig,
		Verbose:     c.Verbosity() > 1,
		Concurrency: c.EvalConcurrency(),
	}
}

// vmConfig returns the VM configuration that only has the supplied top-level arguments.
func (c Config) vmConfig(tlaVars []string) vm.Config {
	cfg := c.vmc.WithoutTopLevel()

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
func (c Config) Client(env string) (Client, error) {
	return c.clp(env)
}

// Colorize returns true if output needs to be colorized.
func (c Config) Colorize() bool { return c.colors }

// Verbosity returns the log verbosity level
func (c Config) Verbosity() int { return c.verbose }

// EvalConcurrency returns the concurrency to be used for evaluating components.
func (c Config) EvalConcurrency() int { return c.evalConcurrency }

// Stdout returns the standard output configured for the command.
func (c Config) Stdout() io.Writer {
	return c.stdout
}

// Confirm prompts for confirmation if needed.
func (c Config) Confirm(context string) error {
	fmt.Fprintln(c.stderr)
	fmt.Fprintln(c.stderr, context)
	fmt.Fprintln(c.stderr)
	if c.yes {
		return nil
	}
	inst, err := readline.NewEx(&readline.Config{
		Prompt: "Do you want to continue [y/n]: ",
		Stdin:  c.stdin,
		Stdout: c.stdout,
		Stderr: c.stderr,
	})
	if err != nil {
		return err
	}
	for {
		s, err := inst.Readline()
		if err != nil {
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

// SortConfig returns the sort configuration.
func sortConfig(provider objsort.Namespaced) objsort.Config {
	return objsort.Config{
		NamespacedIndicator: func(gvk schema.GroupVersionKind) (bool, error) {
			ret, err := provider(gvk)
			if err != nil {
				return false, err
			}
			return ret, nil
		},
	}
}
