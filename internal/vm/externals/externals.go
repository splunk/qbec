/*
   Copyright 2021 Splunk Inc.

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

package externals

import (
	"bufio"
	"bytes"
	"fmt"
	"io/ioutil"
	"os"
	"strings"

	"github.com/pkg/errors"
	"github.com/spf13/cobra"
)

// UserVal is a user-supplied variable value to be initialized for the jsonnet VM
type UserVal struct {
	Value string // variable value
	Code  bool   // whether the value should be treated as jsonnet code
}

// newVal returns a value that is interpreted as a string.
func newVal(value string) UserVal {
	return UserVal{Value: value}
}

// newCodeVal returns a values that is interpreted as code.
func newCodeVal(code string) UserVal {
	return UserVal{Value: code, Code: true}
}

// UserVariables is a set of user-provided variables to be registered with a jsonnet VM
type UserVariables struct {
	Vars         map[string]UserVal // variables keyed by name
	TopLevelVars map[string]UserVal // TLA string vars keyed by name
}

// Externals is the desired configuration of the Jsonnet VM as specified by the user.
type Externals struct {
	Variables   UserVariables // variables specified on command line
	LibPaths    []string      // library paths in filesystem for the file importer
	DataSources []string      // data sources defined on the command line
}

// WithLibPaths returns a config with additional library paths.
func (c Externals) WithLibPaths(paths []string) Externals {
	return Externals{Variables: c.Variables, LibPaths: append(c.LibPaths, paths...)}
}

type strFiles struct {
	strings []string
	files   []string
	lists   []string
}

func getValues(ret map[string]UserVal, name string, s strFiles, fn func(value string) UserVal) error {
	processStr := func(s string, ctx string) error {
		parts := strings.SplitN(s, "=", 2)
		if len(parts) == 2 {
			ret[parts[0]] = fn(parts[1])
			return nil
		}
		v, ok := os.LookupEnv(s)
		if !ok {
			return fmt.Errorf("%sno value found from environment for %s", ctx, s)
		}
		ret[s] = fn(v)
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
		ret[parts[0]] = fn(string(b))
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
			return err
		}
	}
	for _, s := range s.strings {
		if err := processStr(s, name+" "); err != nil {
			return err
		}
	}
	for _, s := range s.files {
		if err := processFile(s); err != nil {
			return err
		}
	}
	return nil
}

// FromCommandParams attaches VM related flags to the specified command and returns
// a function that provides the config based on command line flags.
func FromCommandParams(cmd *cobra.Command, prefix string, addShortcuts bool) func() (Externals, error) {
	var (
		extStrings  strFiles
		extCodes    strFiles
		tlaStrings  strFiles
		tlaCodes    strFiles
		paths       []string
		dataSources []string
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
	fs.StringArrayVar(&dataSources, prefix+"data-source", nil, "additional data sources in URL format")

	return func() (c Externals, err error) {
		vars := map[string]UserVal{}
		tlaVars := map[string]UserVal{}
		if err = getValues(vars, "ext-str", extStrings, newVal); err != nil {
			return
		}
		if err = getValues(vars, "ext-code", extCodes, newCodeVal); err != nil {
			return
		}
		if err = getValues(tlaVars, "tla-str", tlaStrings, newVal); err != nil {
			return
		}
		if err = getValues(tlaVars, "tla-code", tlaCodes, newCodeVal); err != nil {
			return
		}
		c.Variables.Vars = vars
		c.Variables.TopLevelVars = tlaVars
		c.LibPaths = paths
		c.DataSources = dataSources
		return
	}
}
