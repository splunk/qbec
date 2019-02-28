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
	"strings"
	"time"

	"github.com/spf13/cobra"
	"github.com/splunk/qbec/internal/commands"
	"github.com/splunk/qbec/internal/sio"
)

const (
	vMajor         = 0
	vMinor         = 6
	jsonnetVersion = "v0.11.2" // update this when library dependency is upgraded
)

// versions set from command line
var (
	PatchVersion       = "0"   // update from LDFLAGS
	PatchVersionSuffix = "dev" // ditto
)

func newVersionCommand(root *cobra.Command) *cobra.Command {
	return &cobra.Command{
		Use:   "version",
		Short: "print program version",
		Run: func(c *cobra.Command, args []string) {
			fmt.Printf("%s version: %d.%d.%s-%s\njsonnet version: %s\n",
				root.CommandPath(),
				vMajor,
				vMinor,
				PatchVersion,
				PatchVersionSuffix,
				jsonnetVersion)
		},
	}
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
	exe := "qbec"
	longdesc := "\n" + strings.Trim(fmt.Sprintf(`

%s provides a set of commands to manage kubernetes objects on multiple clusters.

`, exe), "\n")
	root := &cobra.Command{
		Use:   exe + " <sub-command>",
		Short: "Kubernetes cluster config tool",
		Long:  longdesc,
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
		cmd.Usage()
		sio.Println()
	}
	sio.Errorln(err)
	exit(1)
}
