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
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"sync"

	"github.com/google/go-jsonnet"
	"github.com/google/go-jsonnet/linter"
	"github.com/pkg/errors"
	"github.com/splunk/qbec/internal/vm/importers"
	"github.com/splunk/qbec/internal/vm/natives"
)

// Code wraps string to distinguish it from string file names
type Code struct {
	code string
}

// MakeCode returns a code object from the supplied string.
func MakeCode(s string) Code {
	return Code{code: s}
}

// Config is the configuration of the VM
type Config struct {
	LibPaths    []string               // library paths
	DataSources []importers.DataSource // data sources
}

// VM provides a narrow interface to the capabilities of a jsonnet VM.
type VM interface {
	// EvalFile evaluates the supplied file initializing the VM with the supplied variables
	// and returns its output as a JSON string.
	EvalFile(file string, v VariableSet) (string, error)
	// EvalCode evaluates the supplied code initializing the VM with the supplied variables
	// and returns its output as a JSON string.
	EvalCode(diagnosticFile string, code Code, v VariableSet) (string, error)
	// LintCode uses the jsonnet linter to lint the code and returns any errors
	LintCode(diagnosticFile string, code Code) error
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

// EvalCode implements the interface method.
func (v *vm) EvalCode(diagnosticFile string, code Code, vars VariableSet) (string, error) {
	vars.register(v.jvm)
	return v.jvm.EvaluateAnonymousSnippet(diagnosticFile, code.code)
}

// LintCode implements the interface method.
func (v *vm) LintCode(diagnosticFile string, code Code) error {
	_, err := jsonnet.SnippetToAST(diagnosticFile, code.code)
	if err != nil {
		return errors.Wrap(err, "convert code to AST")
	}
	var b bytes.Buffer
	failure := linter.LintSnippet(v.jvm, &b, diagnosticFile, code.code)
	if failure {
		return fmt.Errorf(b.String())
	}
	return nil
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

// EvalCode implements the interface method.
func (v *vmPool) EvalCode(diagnosticFile string, code Code, vars VariableSet) (string, error) {
	vm := v.pool.Get().(*vm)
	out, err := vm.EvalCode(diagnosticFile, code, vars)
	v.pool.Put(vm)
	return out, err
}

// LintCode implements the interface method.
func (v *vmPool) LintCode(diagnosticFile string, code Code) error {
	vm := v.pool.Get().(*vm)
	err := vm.LintCode(diagnosticFile, code)
	v.pool.Put(vm)
	return err
}

// defaultImporter returns the standard importer.
func defaultImporter(c Config) jsonnet.Importer {
	var imps []importers.ExtendedImporter
	for _, ds := range c.DataSources {
		imps = append(imps, importers.NewDataSourceImporter(ds))
	}
	std := []importers.ExtendedImporter{
		importers.NewGlobImporter("import"),
		importers.NewGlobImporter("importstr"),
		importers.NewFileImporter(&jsonnet.FileImporter{
			JPaths: c.LibPaths,
		}),
	}
	return importers.NewCompositeImporter(append(imps, std...)...)
}

// newJsonnetVM create a new jsonnet VM with native functions and importer registered.
func newJsonnetVM(config Config) *jsonnet.VM {
	jvm := jsonnet.MakeVM()
	natives.Register(jvm)
	jvm.Importer(defaultImporter(config))
	return jvm
}

// New constructs a new VM based on the supplied config. The returned VM interface is safe for concurrent use.
func New(config Config) VM {
	return newPool(config)
}
