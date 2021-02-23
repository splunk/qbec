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
	"log"
	"os"
	"path/filepath"

	"github.com/pkg/errors"
	"github.com/spf13/cobra"
	"github.com/splunk/qbec/internal/cmd"
	"github.com/splunk/qbec/internal/datasource"
	"github.com/splunk/qbec/internal/datasource/api"
	"github.com/splunk/qbec/internal/vm"
	"github.com/splunk/qbec/internal/vm/externals"
	"github.com/splunk/qbec/internal/vm/importers"
)

func getProvider(cfg vm.Config, vs vm.VariableSet) func(name string) (string, error) {
	return func(name string) (string, error) {
		jvm := vm.New(cfg)
		return jvm.EvalCode(
			fmt.Sprintf("<%s>", name),
			vm.MakeCode(fmt.Sprintf(`std.extVar('%s')`, name)),
			vs,
		)
	}
}

func run(file string, ext externals.Externals) (string, error) {
	var dataSources []importers.DataSource
	for _, s := range ext.DataSources {
		ds, err := datasource.Create(s)
		if err != nil {
			return "", errors.Wrapf(err, "create data source %s", s)
		}
		dataSources = append(dataSources, ds)
	}
	cfg := vm.Config{
		LibPaths:    ext.LibPaths,
		DataSources: dataSources,
	}
	vs := vm.VariablesFromConfig(ext)
	provider := getProvider(cfg, vs)
	for _, ds := range dataSources {
		apiDs := ds.(api.DataSource)
		if err := apiDs.Init(provider); err != nil {
			return "", errors.Wrapf(err, "init data source %s", apiDs.Name())
		}
		cmd.RegisterCleanupTask(apiDs)
	}
	jvm := vm.New(cfg)
	return jvm.EvalFile(file, vs)
}

func main() {
	var configInit func() (externals.Externals, error)
	exe := filepath.Base(os.Args[0])
	root := &cobra.Command{
		Use:   exe + " <sub-command>",
		Short: "jsonnet with yaml support",
		RunE: func(c *cobra.Command, args []string) error {
			if len(args) != 1 {
				return fmt.Errorf("exactly one file argument is required")
			}
			ext, err := configInit()
			if err != nil {
				return errors.Wrap(err, "create VM ext")
			}
			s, err := run(args[0], ext)
			if err != nil {
				return err
			}
			fmt.Println(s)
			return nil
		},
	}
	cmd.RegisterSignalHandlers()
	defer cmd.Close()
	configInit = externals.FromCommandParams(root, "", true)
	if err := root.Execute(); err != nil {
		log.Fatalln(err)
	}
}
