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
	"os"
	"path/filepath"

	"github.com/google/go-jsonnet"
	"github.com/splunk/qbec/internal/vm/importers"
	"github.com/splunk/qbec/internal/vm/natives"
)

// VM provides a narrow interface to the capabilities of a jsonnet VM.
type VM interface {
	// EvalFile evaluates the supplied file initializing the VM with the supplied variables
	// and returns its output as a JSON string.
	EvalFile(file string, v VariableSet) (string, error)
}

// vm is an implementation of VM
type vm struct {
	jvm *jsonnet.VM
}

// EvalFile implements the interface method.
func (v *vm) EvalFile(file string, vars VariableSet) (string, error) {
	s, err := os.Stat(file)
	if err != nil {
		if os.IsNotExist(err) {
			return "", fmt.Errorf("%s: file not found", file)
		}
		return "", err
	}
	if s.IsDir() {
		return "", fmt.Errorf("file '%s' was a directory", file)
	}
	vars.register(v.jvm)
	file = filepath.ToSlash(file)
	return v.jvm.EvaluateFile(file)
}

// defaultImporter returns the standard importer.
func defaultImporter(libPaths []string) jsonnet.Importer {
	return importers.NewCompositeImporter(
		importers.NewGlobImporter("import"),
		importers.NewGlobImporter("importstr"),
		importers.NewFileImporter(&jsonnet.FileImporter{
			JPaths: libPaths,
		}),
	)
}

// newJsonnetVM create a new jsonnet VM with native functions and importer registered.
func newJsonnetVM(libPaths []string) *jsonnet.VM {
	jvm := jsonnet.MakeVM()
	natives.Register(jvm)
	jvm.Importer(defaultImporter(libPaths))
	return jvm
}

// New constructs a new VM based on the supplied config.
func New(libPaths []string) VM {
	return &vm{jvm: newJsonnetVM(libPaths)}
}
