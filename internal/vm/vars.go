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
	"github.com/google/go-jsonnet"
	"github.com/google/go-jsonnet/ast"
	"github.com/splunk/qbec/internal/vm/externals"
)

type varKind int

const (
	varKindString varKind = iota
	varKindCode
	varKindNode
)

// Var is an opaque variable to be initialized for the jsonnet VM
type Var struct {
	Name  string
	kind  varKind
	value string
	node  ast.Node
}

// NewVar returns a variable that has a string value
func NewVar(name, value string) Var {
	return Var{Name: name, kind: varKindString, value: value}
}

// NewCodeVar returns a variable that has a code value
func NewCodeVar(name, code string) Var {
	return Var{Name: name, kind: varKindCode, value: code}
}

// VariableSet is an immutable set of variables to be registered with a jsonnet VM
type VariableSet struct {
	vars         map[string]Var // variables keyed by name
	topLevelVars map[string]Var // TLA string vars keyed by name
}

// VariablesFromConfig returns a variable set containing the user-supplied variables.
func VariablesFromConfig(config externals.Externals) VariableSet {
	ret := VariableSet{
		vars:         map[string]Var{},
		topLevelVars: map[string]Var{},
	}
	for name, v := range config.Variables.Vars {
		if v.Code {
			ret.vars[name] = NewCodeVar(name, v.Value)
		} else {
			ret.vars[name] = NewVar(name, v.Value)
		}
	}
	for name, v := range config.Variables.TopLevelVars {
		if v.Code {
			ret.topLevelVars[name] = NewCodeVar(name, v.Value)
		} else {
			ret.topLevelVars[name] = NewVar(name, v.Value)
		}
	}
	return ret
}

func copyMap(m map[string]Var) map[string]Var {
	if m == nil {
		return nil
	}
	ret := map[string]Var{}
	for k, v := range m {
		ret[k] = v
	}
	return ret
}

// Clone creates a clone of this variable set.
func (vs VariableSet) clone() VariableSet {
	ret := VariableSet{}
	ret.vars = copyMap(vs.vars)
	ret.topLevelVars = copyMap(vs.topLevelVars)
	return ret
}

func toVars(v map[string]Var) []Var {
	var ret []Var
	for _, v := range v {
		ret = append(ret, v)
	}
	return ret
}

// Vars returns the names of all variables defined for this variable set.
func (vs VariableSet) Vars() []Var {
	return toVars(vs.vars)
}

// TopLevelVars returns the string top-level variables defined for this variable set.
func (vs VariableSet) TopLevelVars() []Var {
	return toVars(vs.topLevelVars)
}

// HasVar returns true if the specified external variable is defined.
func (vs VariableSet) HasVar(name string) bool {
	_, ok := vs.vars[name]
	return ok
}

// HasTopLevelVar returns true if the specified TLA variable is defined.
func (vs VariableSet) HasTopLevelVar(name string) bool {
	_, ok := vs.topLevelVars[name]
	return ok
}

// WithoutTopLevel returns a variable set that does not have any top level variables set.
func (vs VariableSet) WithoutTopLevel() VariableSet {
	if len(vs.topLevelVars) == 0 {
		return vs
	}
	clone := vs.clone()
	clone.topLevelVars = nil
	return clone
}

// WithVars returns a variable set with additional variables in its environment.
func (vs VariableSet) WithVars(add ...Var) VariableSet {
	if len(add) == 0 {
		return vs
	}
	clone := vs.clone()
	if clone.vars == nil {
		clone.vars = map[string]Var{}
	}
	for _, v := range add {
		clone.vars[v.Name] = v
	}
	return clone
}

// WithTopLevelVars returns a variable set with additional top-level string variables in its environment.
func (vs VariableSet) WithTopLevelVars(add ...Var) VariableSet {
	if len(add) == 0 {
		return vs
	}
	clone := vs.clone()
	if clone.topLevelVars == nil {
		clone.topLevelVars = map[string]Var{}
	}
	for _, v := range add {
		clone.topLevelVars[v.Name] = v
	}
	return clone
}

func (vs VariableSet) register(jvm *jsonnet.VM) {
	for _, v := range vs.vars {
		switch v.kind {
		case varKindCode:
			jvm.ExtCode(v.Name, v.value)
		default:
			jvm.ExtVar(v.Name, v.value)
		}
	}
	for _, v := range vs.topLevelVars {
		switch v.kind {
		case varKindCode:
			jvm.TLACode(v.Name, v.value)
		default:
			jvm.TLAVar(v.Name, v.value)
		}
	}
}
