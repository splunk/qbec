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
	"io/ioutil"
	"log"
	"os"
	"path/filepath"

	"github.com/pkg/errors"
	"github.com/spf13/cobra"
	"github.com/splunk/qbec/internal/vm"
)

func main() {
	var configInit func() (vm.Config, error)
	exe := filepath.Base(os.Args[0])
	root := &cobra.Command{
		Use:   exe + " <sub-command>",
		Short: "jsonnet with yaml support",
		Run: func(c *cobra.Command, args []string) {
			run := func() error {
				if len(args) != 1 {
					return fmt.Errorf("exactly one file argument is required")
				}
				config, err := configInit()
				if err != nil {
					return errors.Wrap(err, "create VM config")
				}
				jvm := vm.New(config)
				file := args[0]
				b, err := ioutil.ReadFile(file)
				if err != nil {
					return err
				}
				str, err := jvm.EvaluateSnippet(file, string(b))
				if err != nil {
					return err
				}
				fmt.Println(str)
				return nil
			}
			if err := run(); err != nil {
				log.Fatalln(err)
			}
		},
	}
	configInit = vm.ConfigFromCommandParams(root, "", true)
	if err := root.Execute(); err != nil {
		log.Fatalln(err)
	}
}
