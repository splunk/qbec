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

	vmConfigFn := vm.ConfigFromCommandParams(root, "vm:", true)
	cfg := remote.NewConfig(root, "k8s:")
	root.SetUsageTemplate(usageTemplate(root.CommandPath()))
	root.PersistentFlags().StringVar(&rootDir, "root", defaultRoot(), "root directory of repo (from QBEC_ROOT or auto-detect)")
	root.PersistentFlags().IntVarP(&cp.Verbosity, "verbose", "v", 0, "verbosity level")
	root.PersistentFlags().BoolVar(&cp.Colors, "colors", false, "colorize output (set automatically if not specified)")
	root.PersistentFlags().BoolVar(&cp.SkipConfirm, "yes", false, "do not prompt for confirmation")
	root.PersistentFlags().BoolVar(&cp.StrictVars, "strict-vars", false, "require declared variables to be specified, do not allow undeclared variables")
	root.PersistentFlags().IntVar(&cp.EvalConcurrency, "eval-concurrency", 5, "concurrency with which to evaluate components")

	root.AddCommand(newOptionsCommand(root))
	root.AddCommand(newVersionCommand())

	var cmdCfg *commands.Config
	root.PersistentPreRunE = func(cmd *cobra.Command, args []string) error {
		if cmd.Name() == "version" || cmd.Name() == "init" { // don't make these commands dependent on work dir
			return nil
		}
		if !cmd.Flags().Changed("colors") {
			cp.Colors = isatty.IsTerminal(os.Stdout.Fd())
		}
		sio.EnableColors = cp.Colors
		if err := setWorkDir(rootDir); err != nil {
			return err
		}
		app, err := model.NewApp("qbec.yaml")
		if err != nil {
			return err
		}
		conf, err := vmConfigFn()
		if err != nil {
			return err
		}
		cmdCfg, err = cp.Config(app, conf, cfg)
		return err
	}
	commands.Setup(root, func() *commands.Config {
		return cmdCfg
	})
}
