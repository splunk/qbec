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

	"github.com/pkg/errors"
	"github.com/spf13/cobra"
	"github.com/splunk/qbec/internal/cmd"
	"github.com/splunk/qbec/internal/filematcher"
	"github.com/splunk/qbec/internal/model"
	"github.com/splunk/qbec/internal/sio"
)

var (
	version         = "dev"
	commit          = "dev"
	goVersion       = "unknown"
	jsonnetVersion  = "v0.20.0"           // update this when library dependency is upgraded
	clientGoVersion = "kubernetes-1.26.8" // ditto when client go dep is upgraded
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
	c := &cobra.Command{
		Use:   "options",
		Short: "print global options for program",
		Long:  strings.Join([]string{"", leader, root.LocalFlags().FlagUsages(), "", trailer, ""}, "\n"),
	}
	return c
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

var noQbecContext = map[string]bool{
	"version":    true,
	"init":       true,
	"completion": true,
	"options":    true,
	"fmt":        true,
}

func doSetup(root *cobra.Command, opts cmd.Options) {
	root.SetUsageTemplate(usageTemplate(root.CommandPath()))
	ccFn := cmd.NewContext(root, opts)
	var appCtx cmd.AppContext

	root.AddCommand(newOptionsCommand(root))
	root.AddCommand(newVersionCommand())

	root.PersistentPreRunE = func(c *cobra.Command, args []string) (outErr error) {
		defer func() {
			outErr = cmd.WrapError(outErr)
		}()
		ctx, err := ccFn()
		if err != nil {
			return err
		}
		sio.EnableColors(ctx.Colorize())
		cmd.RegisterSignalHandlers()

		skipApp := noQbecContext[c.Name()]
		// for the eval command, require qbec machinery only if the env option is specified
		if c.Name() == "eval" {
			e, err := c.Flags().GetString("env")
			if err != nil {
				return err
			}
			if e == "" {
				skipApp = true
			}
		}

		// for the lint command require an app only when the flag is set
		if c.Name() == "lint" {
			e, err := c.Flags().GetBool("app")
			if err != nil {
				return err
			}
			skipApp = !e
		}

		// retain backwards compatibility for the alpha fmt command
		if c.Name() == "fmt" && c.Parent().Name() == "alpha" {
			skipApp = false
		}

		if skipApp { // do not change work dir, do not load app
			appCtx, err = ctx.AppContext(nil)
			return err
		}
		// if env file has been specified on the command line, ensure it is resolved w.r.t to the current working
		// directory before we change it
		var envFiles []string
		for _, envFile := range ctx.EnvFiles() {
			files, err := filematcher.Match(envFile)
			if err != nil {
				return err
			}
			envFiles = append(envFiles, files...)
		}
		if err := setWorkDir(ctx.RootDir()); err != nil {
			return err
		}
		app, err := model.NewApp("qbec.yaml", envFiles, ctx.AppTag())
		if err != nil {
			return err
		}
		forceOpts, err := ctx.ForceOptions()
		if err != nil {
			return err
		}
		app.SetOverrideNamespace(forceOpts.K8sNamespace)
		appCtx, err = ctx.AppContext(app)
		return err
	}

	root.PersistentPostRunE = func(c *cobra.Command, args []string) error {
		return cmd.Close()
	}

	setupCommands(root, func() cmd.AppContext {
		return appCtx
	})
}

// Setup sets up all sub-commands for the supplied root command and adds facilities for commands
// to access common options.
func Setup(root *cobra.Command) {
	doSetup(root, cmd.Options{})
}
