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
	"context"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/spf13/cobra"
	"github.com/splunk/qbec/internal/cmd"
	"github.com/splunk/qbec/internal/commands"
	"github.com/splunk/qbec/internal/sio"
)

var start = time.Now()

func main() {
	longdesc := "\n" + strings.Trim(fmt.Sprintf(`

%s provides a set of commands to manage kubernetes objects on multiple clusters.

`, commands.Executable), "\n")
	root := &cobra.Command{
		Use:                    commands.Executable,
		Short:                  "Kubernetes cluster config tool",
		Long:                   longdesc,
		BashCompletionFunction: commands.BashCompletionFunc,
	}
	root.SilenceUsage = true
	root.SilenceErrors = true
	commands.Setup(root)
	c, err := root.ExecuteContextC(context.TODO())

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
	case cmd.IsRuntimeError(err):
	default:
		sio.Println()
		c.Example = "" // do not print examples when there is a usage error
		_ = c.Usage()
		sio.Println()
	}
	sio.Errorln(err)
	exit(1)
}
