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

package main

import (
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/chzyer/readline"
	"github.com/mattn/go-isatty"
	"github.com/pkg/errors"
	"github.com/spf13/cobra"
	"github.com/splunk/qbec/internal/commands"
	"github.com/splunk/qbec/internal/model"
	"github.com/splunk/qbec/internal/objsort"
	"github.com/splunk/qbec/internal/remote"
	"github.com/splunk/qbec/internal/sio"
	"github.com/splunk/qbec/internal/vm"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

type gOpts struct {
	verbose   int            // verbosity level
	app       *model.App     // app loaded from file
	config    vm.Config      // jsonnet VM config
	k8sConfig *remote.Config // remote config for k8s, when needed
	colors    bool           // colorize output
	yes       bool           // auto-confirm
}

func (g gOpts) App() *model.App {
	return g.app
}

func (g gOpts) VM() *vm.VM {
	cfg := g.config.WithLibPaths(g.app.Spec.LibPaths)
	return vm.New(cfg)
}

func (g gOpts) Colorize() bool {
	return g.colors
}

func (g gOpts) Verbosity() int {
	return g.verbose
}

type client struct {
	*remote.Client
}

func (c *client) ValidatorFor(gvk schema.GroupVersionKind) (remote.Validator, error) {
	return c.ServerMetadata().ValidatorFor(gvk)
}

func (c *client) DisplayName(o model.K8sMeta) string {
	return c.ServerMetadata().DisplayName(o)
}

func (c *client) IsNamespaced(kind schema.GroupVersionKind) (bool, error) {
	return c.ServerMetadata().IsNamespaced(kind)
}

func (g gOpts) DefaultNamespace(env string) string {
	envObj := g.app.Spec.Environments[env]
	ns := envObj.DefaultNamespace
	if ns == "" {
		ns = "default"
	}
	return ns
}

func (g gOpts) Client(env string) (commands.Client, error) {
	envObj, ok := g.app.Spec.Environments[env]
	if !ok {
		return nil, fmt.Errorf("get client: invalid environment %q", env)
	}
	ns := envObj.DefaultNamespace
	if ns == "" {
		ns = "default"
	}
	rem, err := g.k8sConfig.Client(remote.ConnectOpts{
		EnvName:   env,
		ServerURL: envObj.Server,
		Namespace: ns,
		Verbosity: g.verbose,
	})
	if err != nil {
		return nil, err
	}
	return &client{Client: rem}, nil
}

func (g gOpts) SortConfig(provider objsort.Namespaced) objsort.Config {
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

func (g gOpts) Stdout() io.Writer {
	return os.Stdout
}

func (g gOpts) Confirm(context string) error {
	fmt.Fprintln(os.Stderr)
	fmt.Fprintln(os.Stderr, context)
	fmt.Fprintln(os.Stderr)
	if g.yes {
		return nil
	}
	inst, err := readline.New("Do you want to continue [y/n]: ")
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

func usageTemplate(rootCmd string) string {
	return fmt.Sprintf(`Usage:{{if .Runnable}}
  {{.UseLine}}{{end}}{{if .HasAvailableSubCommands}}
  {{.CommandPath}} [command]{{end}}{{if gt (len .Aliases) 0}}

Aliases:
  {{.NameAndAliases}}{{end}}{{if .HasExample}}

Examples:
{{.Example}}{{end}}{{if .HasAvailableSubCommands}}

Available Commands:{{range .Commands}}{{if (or .IsAvailableCommand (eq .Name "help"))}}
  {{rpad .Name .NamePadding }} {{.Short}}{{end}}{{end}}{{end}}{{if .HasAvailableLocalFlags}}

Flags:
{{.LocalFlags.FlagUsages | trimTrailingWhitespaces}}{{end}}{{if .HasAvailableSubCommands}}

Use "{{.CommandPath}} [command] --help" for more information about a command.{{end}}

Use "%s options" for a list of global options available to all commands.
`, rootCmd)
}

var expectedFiles = []string{"qbec.yaml"}

// setWorkDir sets the working dir of the current process as the top-level
// directory of the source tree. The current working directory of the process
// must be somewhere at or under this tree. If the specified argument is non-empty
// then it is returned provided it is a valid root.
func setWorkDir(specified string) error {
	isRootDir := func(dir string) bool {
		for _, f := range expectedFiles {
			if _, err := os.Stat(filepath.Join(dir, f)); err != nil {
				return false
			}
		}
		return true
	}
	cwd, err := os.Getwd()
	if err != nil {
		return errors.Wrap(err, "os.Getwd")
	}
	orig := cwd

	doChdir := func(dir string) error {
		if orig != dir {
			sio.Debugln(fmt.Sprintf("cd %s", dir))
			if err := os.Chdir(dir); err != nil {
				return err
			}
		}
		return nil
	}

	if specified != "" {
		abs, err := filepath.Abs(specified)
		if err != nil {
			return err
		}
		if !isRootDir(abs) {
			return fmt.Errorf("specified root %q not valid, does not contain expected files", abs)
		}
		return doChdir(abs)
	}

	for {
		if isRootDir(cwd) {
			return doChdir(cwd)
		}
		old := cwd
		cwd = filepath.Dir(cwd)
		if cwd == "" || old == cwd {
			return fmt.Errorf("unable to find source root at or above %s", orig)
		}
	}
}

// setup sets up all sub-commands for the supplied root command and adds facilities for commands
// to access common options.
func setup(root *cobra.Command) {
	var opts gOpts
	var rootDir string

	vmConfigFn := vm.ConfigFromCommandParams(root, "vm:")
	cfg := remote.NewConfig(root, "k8s:")
	root.SetUsageTemplate(usageTemplate(root.CommandPath()))
	root.PersistentFlags().StringVar(&rootDir, "root", defaultRoot(), "root directory of repo (from QBEC_ROOT or auto-detect)")
	root.PersistentFlags().IntVarP(&opts.verbose, "verbose", "v", 0, "verbosity level")
	root.PersistentFlags().BoolVar(&opts.colors, "colors", false, "colorize output (set automatically if not specified)")
	root.PersistentFlags().BoolVar(&opts.yes, "yes", false, "do not prompt for confirmation")

	root.AddCommand(newOptionsCommand(root))
	root.AddCommand(newVersionCommand())
	root.PersistentPreRunE = func(cmd *cobra.Command, args []string) error {
		if cmd.Name() == "version" || cmd.Name() == "init" { // don't make the version command dependent on work dir
			return nil
		}
		if !cmd.Flags().Changed("colors") {
			opts.colors = isatty.IsTerminal(os.Stdout.Fd())
		}
		sio.EnableColors = opts.colors
		if err := setWorkDir(rootDir); err != nil {
			return err
		}
		c, err := model.NewApp("qbec.yaml")
		if err != nil {
			return err
		}
		opts.app = c
		conf, err := vmConfigFn()
		if err != nil {
			return err
		}
		opts.config = conf
		opts.k8sConfig = cfg
		return nil
	}
	commands.Setup(root, func() commands.StdOptionsWithClient {
		return opts
	})
}
