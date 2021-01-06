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
	"bufio"
	"bytes"
	"fmt"
	"io/ioutil"
	"os"
	"strings"

	"github.com/google/go-jsonnet"
	"github.com/pkg/errors"
	"github.com/spf13/cobra"
	"github.com/splunk/qbec/internal/vm/importers"
)

// Config is the desired configuration of the Jsonnet VM.
type Config struct {
	vars             map[string]string // string variables keyed by name
	codeVars         map[string]string // code variables keyed by name
	topLevelVars     map[string]string // TLA string vars keyed by name
	topLevelCodeVars map[string]string // TLA code vars keyed by name
	importer         jsonnet.Importer  // optional custom importer - default is the filesystem importer
	libPaths         []string          // library paths in filesystem, ignored when a custom importer is specified
}

func copyArray(in []string) []string {
	return append([]string{}, in...)
}

func copyMap(m map[string]string) map[string]string {
	ret := map[string]string{}
	if m == nil {
		return nil
	}
	for k, v := range m {
		ret[k] = v
	}
	return ret
}

func copyMapNonNil(m map[string]string) map[string]string {
	ret := copyMap(m)
	if ret == nil {
		ret = map[string]string{}
	}
	return ret
}

// Clone creates a clone of this config.
func (c Config) clone() Config {
	ret := Config{
		importer: c.importer,
	}
	ret.vars = copyMap(c.vars)
	ret.codeVars = copyMap(c.codeVars)
	ret.topLevelVars = copyMap(c.topLevelVars)
	ret.topLevelCodeVars = copyMap(c.topLevelCodeVars)
	ret.libPaths = copyArray(c.libPaths)
	return ret
}

// Vars returns the string external variables defined for this config.
func (c Config) Vars() map[string]string {
	return copyMapNonNil(c.vars)
}

// CodeVars returns the code external variables defined for this config.
func (c Config) CodeVars() map[string]string {
	return copyMapNonNil(c.codeVars)
}

// TopLevelVars returns the string top-level variables defined for this config.
func (c Config) TopLevelVars() map[string]string {
	return copyMapNonNil(c.topLevelVars)
}

// TopLevelCodeVars returns the code top-level variables defined for this config.
func (c Config) TopLevelCodeVars() map[string]string {
	return copyMapNonNil(c.topLevelCodeVars)
}

// LibPaths returns the library paths for this config.
func (c Config) LibPaths() []string {
	return copyArray(c.libPaths)
}

func keyExists(m map[string]string, key string) bool {
	if m == nil {
		return false
	}
	_, ok := m[key]
	return ok
}

// HasVar returns true if the specified external variable is defined.
func (c Config) HasVar(name string) bool {
	return keyExists(c.vars, name) || keyExists(c.codeVars, name)
}

// HasTopLevelVar returns true if the specified TLA variable is defined.
func (c Config) HasTopLevelVar(name string) bool {
	return keyExists(c.topLevelVars, name) || keyExists(c.topLevelCodeVars, name)
}

// WithoutTopLevel returns a config that does not have any top level variables set.
func (c Config) WithoutTopLevel() Config {
	if len(c.topLevelCodeVars) == 0 && len(c.topLevelVars) == 0 {
		return c
	}
	clone := c.clone()
	clone.topLevelVars = nil
	clone.topLevelCodeVars = nil
	return clone
}

// WithCodeVars returns a config with additional code variables in its environment.
func (c Config) WithCodeVars(add map[string]string) Config {
	if len(add) == 0 {
		return c
	}
	clone := c.clone()
	if clone.codeVars == nil {
		clone.codeVars = map[string]string{}
	}
	for k, v := range add {
		clone.codeVars[k] = v
	}
	return clone
}

// WithTopLevelCodeVars returns a config with additional top-level code variables in its environment.
func (c Config) WithTopLevelCodeVars(add map[string]string) Config {
	if len(add) == 0 {
		return c
	}
	clone := c.clone()
	if clone.topLevelCodeVars == nil {
		clone.topLevelCodeVars = map[string]string{}
	}
	for k, v := range add {
		clone.topLevelCodeVars[k] = v
	}
	return clone
}

// WithVars returns a config with additional string variables in its environment.
func (c Config) WithVars(add map[string]string) Config {
	if len(add) == 0 {
		return c
	}
	clone := c.clone()
	if clone.vars == nil {
		clone.vars = map[string]string{}
	}
	for k, v := range add {
		clone.vars[k] = v
	}
	return clone
}

// WithTopLevelVars returns a config with additional top-level string variables in its environment.
func (c Config) WithTopLevelVars(add map[string]string) Config {
	if len(add) == 0 {
		return c
	}
	clone := c.clone()
	if clone.topLevelVars == nil {
		clone.topLevelVars = map[string]string{}
	}
	for k, v := range add {
		clone.topLevelVars[k] = v
	}
	return clone
}

// WithLibPaths returns a config with additional library paths.
func (c Config) WithLibPaths(paths []string) Config {
	if len(paths) == 0 {
		return c
	}
	clone := c.clone()
	clone.libPaths = append(clone.libPaths, paths...)
	return clone
}

// WithImporter returns a config with the supplied importer.
func (c Config) WithImporter(importer jsonnet.Importer) Config {
	clone := c.clone()
	clone.importer = importer
	return clone
}

type strFiles struct {
	strings []string
	files   []string
	lists   []string
}

func getValues(name string, s strFiles) (map[string]string, error) {
	ret := map[string]string{}

	processStr := func(s string, ctx string) error {
		parts := strings.SplitN(s, "=", 2)
		if len(parts) == 2 {
			ret[parts[0]] = parts[1]
			return nil
		}
		v, ok := os.LookupEnv(s)
		if !ok {
			return fmt.Errorf("%sno value found from environment for %s", ctx, s)
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
	processList := func(l string) error {
		b, err := ioutil.ReadFile(l)
		if err != nil {
			return err
		}
		scanner := bufio.NewScanner(bytes.NewReader(b))
		num := 0
		for scanner.Scan() {
			num++
			line := scanner.Text()
			if line != "" {
				err := processStr(line, "")
				if err != nil {
					return errors.Wrap(err, fmt.Sprintf("process list %s, line %d", l, num))
				}
			}
		}
		if err := scanner.Err(); err != nil {
			return errors.Wrap(err, fmt.Sprintf("process list %s", l))
		}
		return nil
	}
	for _, s := range s.lists {
		if err := processList(s); err != nil {
			return nil, err
		}
	}
	for _, s := range s.strings {
		if err := processStr(s, name+" "); err != nil {
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
func ConfigFromCommandParams(cmd *cobra.Command, prefix string, addShortcuts bool) func() (Config, error) {
	var (
		extStrings strFiles
		extCodes   strFiles
		tlaStrings strFiles
		tlaCodes   strFiles
		paths      []string
	)
	fs := cmd.PersistentFlags()
	if addShortcuts {
		fs.StringArrayVarP(&extStrings.strings, prefix+"ext-str", "V", nil, "external string: <var>=[val], if <val> is omitted, get from environment var <var>")
	} else {
		fs.StringArrayVar(&extStrings.strings, prefix+"ext-str", nil, "external string: <var>=[val], if <val> is omitted, get from environment var <var>")
	}
	fs.StringArrayVar(&extStrings.files, prefix+"ext-str-file", nil, "external string from file: <var>=<filename>")
	fs.StringArrayVar(&extStrings.lists, prefix+"ext-str-list", nil, "file containing lines of the form <var>[=<val>]")
	fs.StringArrayVar(&extCodes.strings, prefix+"ext-code", nil, "external code: <var>=[val], if <val> is omitted, get from environment var <var>")
	fs.StringArrayVar(&extCodes.files, prefix+"ext-code-file", nil, "external code from file: <var>=<filename>")
	if addShortcuts {
		fs.StringArrayVarP(&tlaStrings.strings, prefix+"tla-str", "A", nil, "top-level string: <var>=[val], if <val> is omitted, get from environment var <var>")
	} else {
		fs.StringArrayVar(&tlaStrings.strings, prefix+"tla-str", nil, "top-level string: <var>=[val], if <val> is omitted, get from environment var <var>")
	}
	fs.StringArrayVar(&tlaStrings.files, prefix+"tla-str-file", nil, "top-level string from file: <var>=<filename>")
	fs.StringArrayVar(&tlaStrings.lists, prefix+"tla-str-list", nil, "file containing lines of the form <var>[=<val>]")
	fs.StringArrayVar(&tlaCodes.strings, prefix+"tla-code", nil, "top-level code: <var>=[val], if <val> is omitted, get from environment var <var>")
	fs.StringArrayVar(&tlaCodes.files, prefix+"tla-code-file", nil, "top-level code from file: <var>=<filename>")
	fs.StringArrayVar(&paths, prefix+"jpath", nil, "additional jsonnet library path")

	return func() (c Config, err error) {
		if c.vars, err = getValues("ext-str", extStrings); err != nil {
			return
		}
		if c.codeVars, err = getValues("ext-code", extCodes); err != nil {
			return
		}
		if c.topLevelVars, err = getValues("tla-str", tlaStrings); err != nil {
			return
		}
		if c.topLevelCodeVars, err = getValues("tla-code", tlaCodes); err != nil {
			return
		}
		c.libPaths = paths
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
		for k, v := range m {
			registrar(k, v)
		}
	}
	registerVars(config.vars, vm.ExtVar)
	registerVars(config.codeVars, vm.ExtCode)
	registerVars(config.topLevelVars, vm.TLAVar)
	registerVars(config.topLevelCodeVars, vm.TLACode)
	if config.importer != nil {
		vm.Importer(config.importer)
	} else {
		vm.Importer(
			importers.NewCompositeImporter(
				importers.NewGlobImporter("import"),
				importers.NewGlobImporter("importstr"),
				importers.NewFileImporter(&jsonnet.FileImporter{
					JPaths: config.libPaths,
				}),
			),
		)
	}
	return &VM{VM: vm, config: config}
}

// Config returns the current VM config.
func (v *VM) Config() Config {
	return v.config
}
