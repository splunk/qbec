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

// Package vm allows flexible creation of a Jsonnet VM.
package vm

import (
	"fmt"
	"io/ioutil"
	"os"
	"strings"

	"github.com/google/go-jsonnet"
	"github.com/spf13/cobra"
)

// Config is the desired configuration of the Jsonnet VM.
type Config struct {
	Vars             map[string]string // variables keyed by name
	CodeVars         map[string]string // code variables keyed by name
	TopLevelVars     map[string]string // TLA vars keyed by name
	TopLevelCodeVars map[string]string // TLA code vars keyed by name
	Importer         jsonnet.Importer  // optional custom importer - default is the filesystem importer
	LibPaths         []string          // library paths in filesystem, ignored when a custom importer is specified
}

// WithCodeVars creates a new config that is the clone of this one with the additional code variables in its
// environment.
func (c Config) WithCodeVars(add map[string]string) Config {
	clone := c
	if clone.CodeVars == nil {
		clone.CodeVars = map[string]string{}
	}
	for k, v := range add {
		clone.CodeVars[k] = v
	}
	return clone
}

// WithVars creates a new config that is the clone of this one with the additional variables in its
// environment.
func (c Config) WithVars(add map[string]string) Config {
	clone := c
	if clone.Vars == nil {
		clone.Vars = map[string]string{}
	}
	for k, v := range add {
		clone.Vars[k] = v
	}
	return clone
}

// WithLibPaths create a new config that is the clone of this one with additional library paths.
func (c Config) WithLibPaths(paths []string) Config {
	clone := c
	clone.LibPaths = append(clone.LibPaths, paths...)
	return clone
}

type strFiles struct {
	strings []string
	files   []string
}

func getValues(name string, s strFiles) (map[string]string, error) {
	ret := map[string]string{}

	processStr := func(s string) error {
		parts := strings.SplitN(s, "=", 2)
		if len(parts) == 2 {
			ret[parts[0]] = parts[1]
			return nil
		}
		v := os.Getenv(s)
		if v == "" {
			return fmt.Errorf("%s no value found from environment for %s", name, s)
		}
		ret[s] = v
		return nil
	}
	processFile := func(s string) error {
		parts := strings.SplitN(s, "=", 2)
		if len(parts) == 1 {
			return fmt.Errorf("%s-file no filename specified for %s", name, s)
		}
		b, err := ioutil.ReadFile(parts[1])
		if err != nil {
			return err
		}
		ret[parts[0]] = string(b)
		return nil
	}
	for _, s := range s.strings {
		if err := processStr(s); err != nil {
			return nil, err
		}
	}
	for _, s := range s.files {
		if err := processFile(s); err != nil {
			return nil, err
		}
	}
	return ret, nil
}

// ConfigFromCommandParams attaches VM related flags to the specified command and returns
// a function that provides the config based on command line flags.
func ConfigFromCommandParams(cmd *cobra.Command, prefix string) func() (Config, error) {
	var (
		extStrings strFiles
		extCodes   strFiles
		tlaStrings strFiles
		tlaCodes   strFiles
		paths      []string
	)
	fs := cmd.PersistentFlags()
	fs.StringArrayVar(&extStrings.strings, prefix+"ext-str", nil, "external string: <var>=[val], if <val> is omitted, get from environment var <var>")
	fs.StringArrayVar(&extStrings.files, prefix+"ext-str-file", nil, "external string from file: <var>=<filename>")
	fs.StringArrayVar(&extCodes.strings, prefix+"ext-code", nil, "external code: <var>=[val], if <val> is omitted, get from environment var <var>")
	fs.StringArrayVar(&extCodes.files, prefix+"ext-code-file", nil, "external code from file: <var>=<filename>")
	fs.StringArrayVar(&tlaStrings.strings, prefix+"tla-str", nil, "top-level string: <var>=[val], if <val> is omitted, get from environment var <var>")
	fs.StringArrayVar(&tlaStrings.files, prefix+"tla-str-file", nil, "top-level string from file: <var>=<filename>")
	fs.StringArrayVar(&tlaCodes.strings, prefix+"tla-code", nil, "top-level code: <var>=[val], if <val> is omitted, get from environment var <var>")
	fs.StringArrayVar(&tlaCodes.files, prefix+"tla-code-file", nil, "top-level code from file: <var>=<filename>")
	fs.StringArrayVar(&paths, prefix+"jpath", nil, "additional jsonnet library path")

	return func() (c Config, err error) {
		if c.Vars, err = getValues("ext-str", extStrings); err != nil {
			return
		}
		if c.CodeVars, err = getValues("ext-code", extCodes); err != nil {
			return
		}
		if c.TopLevelVars, err = getValues("tla-str", tlaStrings); err != nil {
			return
		}
		if c.TopLevelCodeVars, err = getValues("tla-code", tlaCodes); err != nil {
			return
		}
		c.LibPaths = paths
		return
	}
}

// VM wraps a jsonnet VM and provides some additional methods to create new
// VMs using the same base configuration and additional tweaks.
type VM struct {
	*jsonnet.VM
	config Config
}

// New constructs a new VM based on the supplied config.
func New(config Config) *VM {
	vm := jsonnet.MakeVM()
	registerNativeFuncs(vm)
	registerVars := func(m map[string]string, registrar func(k, v string)) {
		if m != nil {
			for k, v := range m {
				registrar(k, v)
			}
		}
	}
	registerVars(config.Vars, vm.ExtVar)
	registerVars(config.CodeVars, vm.ExtCode)
	registerVars(config.TopLevelVars, vm.TLAVar)
	registerVars(config.TopLevelCodeVars, vm.TLACode)
	if config.Importer != nil {
		vm.Importer(config.Importer)
	} else {
		vm.Importer(&jsonnet.FileImporter{
			JPaths: config.LibPaths,
		})
	}
	return &VM{VM: vm, config: config}
}

// Config returns the current VM config.
func (v *VM) Config() Config {
	return v.config
}
