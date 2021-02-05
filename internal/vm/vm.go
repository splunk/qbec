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
	"sync"

	"github.com/google/go-jsonnet"
	"github.com/splunk/qbec/internal/vm/importers"
	"github.com/splunk/qbec/internal/vm/natives"
)

// Config is the configuration of the VM
type Config struct {
	LibPaths []string // library paths
}

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

type vmPool struct {
	pool sync.Pool
}

func newPool(config Config) *vmPool {
	return &vmPool{
		pool: sync.Pool{
			New: func() interface{} {
				return &vm{jvm: newJsonnetVM(config)}
			},
		},
	}
}

// EvalFile implements the interface method.
func (v *vmPool) EvalFile(file string, vars VariableSet) (string, error) {
	vm := v.pool.Get().(*vm)
	out, err := vm.EvalFile(file, vars)
	v.pool.Put(vm)
	return out, err
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
func newJsonnetVM(config Config) *jsonnet.VM {
	jvm := jsonnet.MakeVM()
	natives.Register(jvm)
	jvm.Importer(defaultImporter(config.LibPaths))
	return jvm
}

// New constructs a new VM based on the supplied config. The returned VM interface is safe for concurrent use.
func New(config Config) VM {
	return newPool(config)
}
