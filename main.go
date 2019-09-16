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
	"encoding/json"
	"fmt"
	"log"
	"os"
	"strings"
	"time"

	"github.com/spf13/cobra"
	"github.com/splunk/qbec/internal/commands"
	"github.com/splunk/qbec/internal/sio"
)

var (
	version        = "dev"
	commit         = "dev"
	goVersion      = "unknown"
	jsonnetVersion = "v0.14.0" // update this when library dependency is upgraded
)

var exe = "qbec"

func newVersionCommand() *cobra.Command {
	var jsonOutput bool

	c := &cobra.Command{
		Use:   "version",
		Short: "print program version",
		Run: func(c *cobra.Command, args []string) {
			if jsonOutput {
				out := struct {
					Qbec    string `json:"qbec"`
					Jsonnet string `json:"jsonnet"`
					Go      string `json:"go"`
					Commit  string `json:"commit"`
				}{
					version,
					jsonnetVersion,
					goVersion,
					commit,
				}
				enc := json.NewEncoder(os.Stdout)
				enc.SetIndent("", "  ")
				if err := enc.Encode(out); err != nil {
					log.Fatalln(err)
				}
				return
			}
			fmt.Printf("%s version: %s\njsonnet version: %s\ngo version: %s\ncommit: %s\n",
				exe,
				version,
				jsonnetVersion,
				goVersion,
				commit,
			)
		},
	}
	c.Flags().BoolVar(&jsonOutput, "json", false, "print versions in JSON format")
	return c
}

func newOptionsCommand(root *cobra.Command) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "options",
		Short: "print global options for program",
		Long:  fmt.Sprintf("\nAll %s commands accept the following options (some may not use them unless relevant):\n", root.CommandPath()) + root.LocalFlags().FlagUsages(),
	}
	return cmd
}

var start = time.Now()

func main() {
	longdesc := "\n" + strings.Trim(fmt.Sprintf(`

%s provides a set of commands to manage kubernetes objects on multiple clusters.

`, exe), "\n")
	root := &cobra.Command{
		Use:                    exe,
		Short:                  "Kubernetes cluster config tool",
		Long:                   longdesc,
		BashCompletionFunction: commands.BashCompletionFunc,
	}
	root.SilenceUsage = true
	root.SilenceErrors = true
	setup(root)
	cmd, err := root.ExecuteC()

	exit := func(code int) {
		duration := time.Since(start).Round(time.Second / 100)
		if duration > 100*time.Millisecond {
			sio.Debugln("command took", duration)
		}
		os.Exit(code)
	}

	switch {
	case err == nil:
		exit(0)
	case commands.IsRuntimeError(err):
	default:
		sio.Println()
		cmd.Example = "" // do not print examples when there is a usage error
		_ = cmd.Usage()
		sio.Println()
	}
	sio.Errorln(err)
	exit(1)
}
