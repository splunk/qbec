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
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"sync"

	"github.com/chzyer/readline"
	"github.com/mattn/go-isatty"
	"github.com/spf13/cobra"
	"github.com/splunk/qbec/internal/eval"
	"github.com/splunk/qbec/internal/model"
	"github.com/splunk/qbec/internal/remote"
	"github.com/splunk/qbec/internal/remote/k8smeta"
	"github.com/splunk/qbec/internal/vm"
	"github.com/splunk/qbec/internal/vm/externals"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"
)

// KubeClient encapsulates all remote operations needed for the superset of all commands.
type KubeClient interface {
	DisplayName(o model.K8sMeta) string
	IsNamespaced(kind schema.GroupVersionKind) (bool, error)
	Get(obj model.K8sMeta) (*unstructured.Unstructured, error)
	Sync(obj model.K8sLocalObject, opts remote.SyncOptions) (*remote.SyncResult, error)
	ValidatorFor(gvk schema.GroupVersionKind) (k8smeta.Validator, error)
	ListObjects(scope remote.ListQueryConfig) (remote.Collection, error)
	Delete(model.K8sMeta, remote.DeleteOptions) (*remote.SyncResult, error)
	ObjectKey(obj model.K8sMeta) string
	ResourceInterface(obj schema.GroupVersionKind, namespace string) (dynamic.ResourceInterface, error)
}

// ClientProvider returns a kubernetes client for the specific environment
type ClientProvider func(env string) (KubeClient, error)

// KubeAttrsProvider provides k8s attributes of the supplied environment
type KubeAttrsProvider func(env string) (*remote.KubeAttributes, error)

// Options are optional attributes to create a context, mostly used for testing.
type Options struct {
	Stdout            io.Writer
	Stderr            io.Writer
	SkipConfirm       bool
	ClientProvider    ClientProvider
	KubeAttrsProvider KubeAttrsProvider
}

// Context is the global context of the qbec command that handles all global options supported by
// the tool.
type Context struct {
	root            string                       // qbec root directory
	appTag          string                       // tag for GC scope
	envFile         string                       // additional environment file
	remote          *remote.Config               // remote config
	forceOptsFn     func() (ForceOptions, error) // options to force cluster/ namespace
	ext             externals.Externals          // external config
	clp             ClientProvider               // the client provider
	attrsp          KubeAttrsProvider            // the kubernetes attribute provider
	colors          bool                         // colorize output
	yes             bool                         // auto-confirm
	evalConcurrency int                          // concurrency of component eval
	verbose         int                          // verbosity level
	stdin           io.Reader                    // standard input
	stdout          io.Writer                    // standard output
	stderr          io.Writer                    // standard error
	strictVars      bool                         // strict vars
	app             *model.App                   // app loaded from file
}

func envOrDefault(name, def string) string {
	v := os.Getenv(name)
	if v != "" {
		return v
	}
	return def
}

func defaultRoot() string {
	return envOrDefault("QBEC_ROOT", "")
}

func defaultEnvironmentFile() string {
	return envOrDefault("QBEC_ENV_FILE", "")
}

func skipPrompts() bool {
	return os.Getenv("QBEC_YES") == "true"
}

// memoizeForceFn caches the output of the first call to supplied function and returns the same outputs always.
func memoizeForceFn(fn func() (ForceOptions, error)) func() (ForceOptions, error) {
	var fOpts ForceOptions
	var fErr error
	var once sync.Once
	return func() (ForceOptions, error) {
		once.Do(func() {
			fOpts, fErr = fn()
		})
		return fOpts, fErr
	}
}

// New sets up the supplied root command with common options and returns a function to
// get the context after arguments have been parsed.
func New(root *cobra.Command, opts Options) func() (Context, error) {
	extConfigFn := externals.FromCommandParams(root, "vm:", true)
	remoteConfig := remote.NewConfig(root, "k8s:")
	forceOptsFn := addForceOptions(root, remoteConfig, "force:")

	// the reason that we don't immediately evaluate the force function has to do with kubeconfig set to a relative
	// path that changes if qbec changes directory to the qbec root. Historically any override kubeconfigs with relative
	// path evaluated w.r.t to the qbec root and the lazy eval of the force function preserves this behavior./
	cf := Context{
		remote:      remoteConfig,
		clp:         opts.ClientProvider,
		forceOptsFn: memoizeForceFn(forceOptsFn),
		attrsp:      opts.KubeAttrsProvider,
		stdout:      opts.Stdout,
		stderr:      opts.Stderr,
		yes:         opts.SkipConfirm || skipPrompts(),
	}
	cf.stdin = os.Stdin
	if cf.stdout == nil {
		cf.stdout = os.Stdout
	}
	if cf.stderr == nil {
		cf.stderr = os.Stderr
	}

	root.PersistentFlags().StringVar(&cf.root, "root", defaultRoot(), "root directory of repo (from QBEC_ROOT or auto-detect)")
	root.PersistentFlags().IntVarP(&cf.verbose, "verbose", "v", cf.verbose, "verbosity level")
	root.PersistentFlags().BoolVar(&cf.colors, "colors", cf.colors, "colorize output (set automatically if not specified)")
	root.PersistentFlags().BoolVar(&cf.yes, "yes", cf.yes, "do not prompt for confirmation. The default value can be overridden by setting QBEC_YES=true")
	root.PersistentFlags().BoolVar(&cf.strictVars, "strict-vars", cf.strictVars, "require declared variables to be specified, do not allow undeclared variables")
	root.PersistentFlags().IntVar(&cf.evalConcurrency, "eval-concurrency", cf.evalConcurrency, "concurrency with which to evaluate components")
	root.PersistentFlags().StringVar(&cf.appTag, "app-tag", "", "build tag to create suffixed objects, indicates GC scope")
	root.PersistentFlags().StringVarP(&cf.envFile, "env-file", "E", defaultEnvironmentFile(), "use additional environment file not declared in qbec.yaml")

	return func() (ret Context, err error) {
		if !root.Flags().Changed("colors") {
			cf.colors = isatty.IsTerminal(os.Stdout.Fd())
		}
		cf.ext, err = extConfigFn()
		if err != nil {
			return ret, err
		}
		return cf, nil
	}
}

// RootDir returns an overridden root dir or blank
func (c Context) RootDir() string { return c.root }

// AppTag returns the app tag specified
func (c Context) AppTag() string { return c.appTag }

// EnvFiles returns additional environment files and URLs
func (c Context) EnvFiles() []string {
	if c.envFile == "" {
		return nil
	}
	return []string{c.envFile}
}

// Colorize returns true if output needs to be colorized.
func (c Context) Colorize() bool { return c.colors }

// Verbosity returns the log verbosity level
func (c Context) Verbosity() int { return c.verbose }

// EvalConcurrency returns the concurrency to be used for evaluating components.
func (c Context) EvalConcurrency() int { return c.evalConcurrency }

// Stdout returns the standard output configured for the command.
func (c Context) Stdout() io.Writer { return c.stdout }

// Stderr returns the standard error configured for the command.
func (c Context) Stderr() io.Writer { return c.stderr }

// ForceOptions returns the forced context and/ or namespace if any. The caller will never
// see the value __current__ since that is already resolved by the option processor.
func (c Context) ForceOptions() (ForceOptions, error) {
	return c.forceOptsFn()
}

// KubeContextInfo returns kube context information.
func (c Context) KubeContextInfo() (*remote.ContextInfo, error) {
	return c.remote.CurrentContextInfo()
}

// Confirm prompts for confirmation if needed.
func (c Context) Confirm(action string) error {
	_, _ = fmt.Fprintln(c.stderr)
	_, _ = fmt.Fprintln(c.stderr, action)
	_, _ = fmt.Fprintln(c.stderr)
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

// AppContext returns an application context for the supplied app. It is valid for the app to be nil
// for commands that do not need it.
func (c Context) AppContext(app *model.App) (AppContext, error) {
	ret := AppContext{Context: c, app: app}
	var err error
	if app != nil {
		err = ret.init()
	}
	return ret, err
}

// BasicEvalContext returns a basic evaluation context without any app-level machinery.
func (c Context) BasicEvalContext() eval.BaseContext {
	return eval.BaseContext{
		LibPaths: c.ext.LibPaths,
		Vars:     vm.VariablesFromConfig(c.ext),
		Verbose:  c.verbose > 1,
	}
}
