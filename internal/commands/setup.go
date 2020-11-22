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
	"log"
	"os"
	"path/filepath"
	"strings"

	"github.com/mattn/go-isatty"
	"github.com/pkg/errors"
	"github.com/spf13/cobra"
	"github.com/splunk/qbec/internal/model"
	"github.com/splunk/qbec/internal/remote"
	"github.com/splunk/qbec/internal/sio"
	"github.com/splunk/qbec/internal/vm"
)

const (
	currentMarker = "__current__"
)

var (
	version         = "dev"
	commit          = "dev"
	goVersion       = "unknown"
	jsonnetVersion  = "v0.16.0"            // update this when library dependency is upgraded
	clientGoVersion = "kubernetes-1.17.13" // ditto when client go dep is upgraded
)

// Executable is the name of the qbec executable.
var Executable = "qbec"

func newVersionCommand() *cobra.Command {
	var jsonOutput bool

	c := &cobra.Command{
		Use:   "version",
		Short: "print program version",
		Run: func(c *cobra.Command, args []string) {
			if jsonOutput {
				out := struct {
					Qbec     string `json:"qbec"`
					Jsonnet  string `json:"jsonnet"`
					ClientGo string `json:"client-go"`
					Go       string `json:"go"`
					Commit   string `json:"commit"`
				}{
					Qbec:     version,
					Jsonnet:  jsonnetVersion,
					ClientGo: clientGoVersion,
					Go:       goVersion,
					Commit:   commit,
				}
				enc := json.NewEncoder(c.OutOrStdout())
				enc.SetIndent("", "  ")
				if err := enc.Encode(out); err != nil {
					log.Fatalln(err)
				}
				return
			}
			fmt.Fprintf(c.OutOrStdout(), "%s version: %s\njsonnet version: %s\nclient-go version: %s\ngo version: %s\ncommit: %s\n",
				Executable,
				version,
				jsonnetVersion,
				clientGoVersion,
				goVersion,
				commit,
			)
		},
	}
	c.Flags().BoolVar(&jsonOutput, "json", false, "print versions in JSON format")
	return c
}

func newOptionsCommand(root *cobra.Command) *cobra.Command {
	leader := fmt.Sprintf("All %s commands accept the following options (some may not use them unless relevant):\n", root.CommandPath())
	trailer := "Note: using options that begin with 'force:' will cause qbec to drop its safety checks. Use with care."
	cmd := &cobra.Command{
		Use:   "options",
		Short: "print global options for program",
		Long:  strings.Join([]string{"", leader, root.LocalFlags().FlagUsages(), "", trailer, ""}, "\n"),
	}
	return cmd
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

func doSetup(root *cobra.Command, cf configFactory, overrideCP clientProvider) {
	var rootDir string
	var appTag string
	var envFile string

	vmConfigFn := vm.ConfigFromCommandParams(root, "vm:", true)
	remoteConfig := remote.NewConfig(root, "k8s:")
	forceOptsFn := addForceOptions(root, "force:")

	root.SetUsageTemplate(usageTemplate(root.CommandPath()))
	root.PersistentFlags().StringVar(&rootDir, "root", defaultRoot(), "root directory of repo (from QBEC_ROOT or auto-detect)")
	root.PersistentFlags().IntVarP(&cf.verbosity, "verbose", "v", cf.verbosity, "verbosity level")
	root.PersistentFlags().BoolVar(&cf.colors, "colors", cf.colors, "colorize output (set automatically if not specified)")
	root.PersistentFlags().BoolVar(&cf.skipConfirm, "yes", cf.skipConfirm, "do not prompt for confirmation. The default value can be overridden by setting QBEC_YES=true")
	root.PersistentFlags().BoolVar(&cf.strictVars, "strict-vars", cf.strictVars, "require declared variables to be specified, do not allow undeclared variables")
	root.PersistentFlags().IntVar(&cf.evalConcurrency, "eval-concurrency", cf.evalConcurrency, "concurrency with which to evaluate components")
	root.PersistentFlags().StringVar(&appTag, "app-tag", "", "build tag to create suffixed objects, indicates GC scope")
	root.PersistentFlags().StringVarP(&envFile, "env-file", "E", defaultEnvironmentFile(), "use additional environment file not declared in qbec.yaml")
	root.AddCommand(newOptionsCommand(root))
	root.AddCommand(newVersionCommand())

	var cmdCfg *config
	root.PersistentPreRunE = func(cmd *cobra.Command, args []string) error {
		if cmd.Name() == "version" || cmd.Name() == "init" || cmd.Name() == "completion" { // don't make these commands dependent on work dir
			return nil
		}
		if !cmd.Flags().Changed("colors") {
			cf.colors = isatty.IsTerminal(os.Stdout.Fd())
		}
		sio.EnableColors(cf.colors)

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
		app, err := model.NewApp("qbec.yaml", envFiles, appTag)
		if err != nil {
			return err
		}

		var cc *remote.ContextInfo
		forceOpts := forceOptsFn()
		if forceOpts.k8sContext == currentMarker {
			cc, err = remote.CurrentContextInfo()
			if err != nil {
				return err
			}
			forceOpts.k8sContext = cc.ContextName
		}
		if forceOpts.k8sNamespace == currentMarker {
			if cc == nil {
				return newUsageError(fmt.Sprintf("current namespace can only be forced when the context is also forced to current"))
			}
			forceOpts.k8sNamespace = cc.Namespace
		}
		app.SetOverrideNamespace(forceOpts.k8sNamespace)
		vmConfig, err := vmConfigFn()
		if err != nil {
			return newRuntimeError(err)
		}

		cmdCfg, err = cf.getConfig(app, vmConfig, remoteConfig, forceOpts, overrideCP)
		return err
	}
	setupCommands(root, func() *config {
		return cmdCfg
	})
}

// Setup sets up all sub-commands for the supplied root command and adds facilities for commands
// to access common options.
func Setup(root *cobra.Command) {
	doSetup(root, configFactory{
		skipConfirm:     skipPrompts(),
		evalConcurrency: 5,
	}, nil)
}
