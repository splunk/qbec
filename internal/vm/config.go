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

package vm

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

// Config is the desired configuration of the Jsonnet VM.
type Config struct {
	Variables VariableSet
	LibPaths  []string // library paths in filesystem for the file importer
}

// WithLibPaths returns a config with additional library paths.
func (c Config) WithLibPaths(paths []string) Config {
	return Config{Variables: c.Variables, LibPaths: append(c.LibPaths, paths...)}
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
		if c.Variables.vars, err = getValues("ext-str", extStrings); err != nil {
			return
		}
		if c.Variables.codeVars, err = getValues("ext-code", extCodes); err != nil {
			return
		}
		if c.Variables.topLevelVars, err = getValues("tla-str", tlaStrings); err != nil {
			return
		}
		if c.Variables.topLevelCodeVars, err = getValues("tla-code", tlaCodes); err != nil {
			return
		}
		c.LibPaths = paths
		return
	}
}
