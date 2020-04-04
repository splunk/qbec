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
	"os"
	"path/filepath"

	"github.com/mattn/go-isatty"
	"github.com/pkg/errors"
	"github.com/spf13/cobra"
	"github.com/splunk/qbec/internal/commands"
	"github.com/splunk/qbec/internal/model"
	"github.com/splunk/qbec/internal/remote"
	"github.com/splunk/qbec/internal/sio"
	"github.com/splunk/qbec/internal/vm"
)

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
	var cp commands.ConfigFactory
	var rootDir string
	var appTag string
	var envFile string

	vmConfigFn := vm.ConfigFromCommandParams(root, "vm:", true)
	remoteConfig := remote.NewConfig(root, "k8s:")
	forceOptsFn := commands.ForceOptionsConfig(root, "force:")

	root.SetUsageTemplate(usageTemplate(root.CommandPath()))
	root.PersistentFlags().StringVar(&rootDir, "root", defaultRoot(), "root directory of repo (from QBEC_ROOT or auto-detect)")
	root.PersistentFlags().IntVarP(&cp.Verbosity, "verbose", "v", 0, "verbosity level")
	root.PersistentFlags().BoolVar(&cp.Colors, "colors", false, "colorize output (set automatically if not specified)")
	root.PersistentFlags().BoolVar(&cp.SkipConfirm, "yes", skipPrompts(), "do not prompt for confirmation. The default value can be overridden by setting QBEC_YES=true")
	root.PersistentFlags().BoolVar(&cp.StrictVars, "strict-vars", false, "require declared variables to be specified, do not allow undeclared variables")
	root.PersistentFlags().IntVar(&cp.EvalConcurrency, "eval-concurrency", 5, "concurrency with which to evaluate components")
	root.PersistentFlags().StringVar(&appTag, "app-tag", "", "build tag to create suffixed objects, indicates GC scope")
	root.PersistentFlags().StringVarP(&envFile, "env-file", "E", defaultEnvironmentFile(), "use additional environment file not declared in qbec.yaml")
	root.AddCommand(newOptionsCommand(root))
	root.AddCommand(newVersionCommand())

	var cmdCfg *commands.Config
	root.PersistentPreRunE = func(cmd *cobra.Command, args []string) error {
		if cmd.Name() == "version" || cmd.Name() == "init" || cmd.Name() == "completion" { // don't make these commands dependent on work dir
			return nil
		}
		if !cmd.Flags().Changed("colors") {
			cp.Colors = isatty.IsTerminal(os.Stdout.Fd())
		}
		sio.EnableColors = cp.Colors

		// if env file has been specified on the command line, ensure it is resolved w.r.t to the current working
		// directory before we change it
		var envFiles []string
		if envFile != "" {
			abs, err := filepath.Abs(envFile)
			if err != nil {
				return err
			}
			envFiles = append(envFiles, abs)
		}
		if err := setWorkDir(rootDir); err != nil {
			return err
		}
		forceOpts := forceOptsFn()
		app, err := model.NewApp("qbec.yaml", envFiles, appTag)
		if err != nil {
			return err
		}
		app.SetOverrideNamespace(forceOpts.K8sNamespace)
		vmConfig, err := vmConfigFn()
		if err != nil {
			return commands.NewRuntimeError(err)
		}
		cmdCfg, err = cp.Config(app, vmConfig, remoteConfig, forceOpts)
		return err
	}
	commands.Setup(root, func() *commands.Config {
		return cmdCfg
	})
}
