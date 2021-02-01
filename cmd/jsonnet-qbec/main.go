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
	"github.com/splunk/qbec/internal/vm/externals"
	"github.com/splunk/qbec/internal/vm/importers"
)

func run(args []string, out io.Writer) error {
	var configInit func() (externals.Externals, error)
	var sources []string
	exe := filepath.Base(args[0])
	root := &cobra.Command{
		Use:   exe + " <sub-command>",
		Short: "jsonnet with yaml support",
		Run: func(c *cobra.Command, args []string) {
			run := func() error {
				if len(args) != 1 {
					return fmt.Errorf("exactly one file argument is required")
				}
				ext, err := configInit()
				if err != nil {
					return errors.Wrap(err, "create VM ext")
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
					err = ds.Start(ext.ToVarMap())
					if err != nil {
						return errors.Wrapf(err, "start data source %s", ds.Name())
					}
					dataSources = append(dataSources, ds)
					closers = append(closers, ds)
				}
				jvm := vm.New(vm.Config{
					LibPaths:    ext.LibPaths,
					DataSources: dataSources,
				})
				file := args[0]
				str, err := jvm.EvalFile(file, vm.VariablesFromConfig(ext))
				if err != nil {
					return err
				}
				_, _ = fmt.Fprintln(out, str)
				return nil
			}
			if err := run(); err != nil {
				log.Fatalln(err)
			}
		},
	}
	configInit = externals.FromCommandParams(root, "", true)
	root.Flags().StringArrayVar(&sources, "data-source", sources, "data source URL")
	root.SetArgs(args[1:])
	if err := root.Execute(); err != nil {
		return err
	}
	return nil
}

func main() {
	if err := run(os.Args, os.Stdout); err != nil {
		log.Fatalln(err)
	}
}
