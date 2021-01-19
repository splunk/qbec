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
	"log"
	"os"
	"path/filepath"

	"github.com/pkg/errors"
	"github.com/spf13/cobra"
	"github.com/splunk/qbec/internal/datasource"
	"github.com/splunk/qbec/internal/vm"
	"github.com/splunk/qbec/internal/vm/importers"
)

func run(args []string, out io.Writer) error {
	var configInit func() (vm.CmdlineConfig, error)
	var sources []string
	exe := filepath.Base(args[0])
	root := &cobra.Command{
		Use:   exe + " <sub-command>",
		Short: "jsonnet with qbec extensions",
		RunE: func(c *cobra.Command, args []string) error {
			if len(args) != 1 {
				return fmt.Errorf("exactly one file argument is required")
			}
			config, err := configInit()
			if err != nil {
				return errors.Wrap(err, "create VM config")
			}
			var dataSources []importers.DataSource
			var closers []io.Closer
			defer func() {
				for _, c := range closers {
					_ = c.Close()
				}
			}()
			for _, src := range sources {
				ds, err := datasource.Create(src)
				if err != nil {
					return err
				}
				err = ds.Start(map[string]interface{}{
					"version": "1.0",
				})
				if err != nil {
					return errors.Wrapf(err, "start data source %s", ds.Name())
				}
				dataSources = append(dataSources, ds)
				closers = append(closers, ds)
			}
			jvm := vm.New(vm.Config{LibPaths: config.LibPaths, DataSources: dataSources})
			file := args[0]
			str, err := jvm.EvalFile(file, config.Variables)
			if err != nil {
				return err
			}
			fmt.Fprintln(out, str)
			return nil
		},
	}
	configInit = vm.ConfigFromCommandParams(root, "", true)
	root.Flags().StringArrayVar(&sources, "data-source", sources, "data source URL")
	root.SetArgs(args[1:])
	return root.Execute()
}

func main() {
	if err := run(os.Args, os.Stdout); err != nil {
		log.Fatalln(err)
	}
}
