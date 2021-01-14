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

import "github.com/google/go-jsonnet"

// VariableSet is an immutable set of variables to be registered with a jsonnet VM
type VariableSet struct {
	vars             map[string]string // string variables keyed by name
	codeVars         map[string]string // code variables keyed by name
	topLevelVars     map[string]string // TLA string vars keyed by name
	topLevelCodeVars map[string]string // TLA code vars keyed by name
}

func copyMap(m map[string]string) map[string]string {
	if m == nil {
		return nil
	}
	ret := map[string]string{}
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

// Clone creates a clone of this variable set.
func (vs VariableSet) clone() VariableSet {
	ret := VariableSet{}
	ret.vars = copyMap(vs.vars)
	ret.codeVars = copyMap(vs.codeVars)
	ret.topLevelVars = copyMap(vs.topLevelVars)
	ret.topLevelCodeVars = copyMap(vs.topLevelCodeVars)
	return ret
}

// Vars returns the string external variables defined for this variable set.
func (vs VariableSet) Vars() map[string]string {
	return copyMapNonNil(vs.vars)
}

// CodeVars returns the code external variables defined for this variable set.
func (vs VariableSet) CodeVars() map[string]string {
	return copyMapNonNil(vs.codeVars)
}

// TopLevelVars returns the string top-level variables defined for this variable set.
func (vs VariableSet) TopLevelVars() map[string]string {
	return copyMapNonNil(vs.topLevelVars)
}

// TopLevelCodeVars returns the code top-level variables defined for this variable set.
func (vs VariableSet) TopLevelCodeVars() map[string]string {
	return copyMapNonNil(vs.topLevelCodeVars)
}

func keyExists(m map[string]string, key string) bool {
	if m == nil {
		return false
	}
	_, ok := m[key]
	return ok
}

// HasVar returns true if the specified external variable is defined.
func (vs VariableSet) HasVar(name string) bool {
	return keyExists(vs.vars, name) || keyExists(vs.codeVars, name)
}

// HasTopLevelVar returns true if the specified TLA variable is defined.
func (vs VariableSet) HasTopLevelVar(name string) bool {
	return keyExists(vs.topLevelVars, name) || keyExists(vs.topLevelCodeVars, name)
}

// WithoutTopLevel returns a variable set that does not have any top level variables set.
func (vs VariableSet) WithoutTopLevel() VariableSet {
	if len(vs.topLevelCodeVars) == 0 && len(vs.topLevelVars) == 0 {
		return vs
	}
	clone := vs.clone()
	clone.topLevelVars = nil
	clone.topLevelCodeVars = nil
	return clone
}

// WithCodeVars returns a variable set with additional code variables in its environment.
func (vs VariableSet) WithCodeVars(add map[string]string) VariableSet {
	if len(add) == 0 {
		return vs
	}
	clone := vs.clone()
	if clone.codeVars == nil {
		clone.codeVars = map[string]string{}
	}
	for k, v := range add {
		clone.codeVars[k] = v
	}
	return clone
}

// WithTopLevelCodeVars returns a variable set with additional top-level code variables in its environment.
func (vs VariableSet) WithTopLevelCodeVars(add map[string]string) VariableSet {
	if len(add) == 0 {
		return vs
	}
	clone := vs.clone()
	if clone.topLevelCodeVars == nil {
		clone.topLevelCodeVars = map[string]string{}
	}
	for k, v := range add {
		clone.topLevelCodeVars[k] = v
	}
	return clone
}

// WithVars returns a variable set with additional string variables in its environment.
func (vs VariableSet) WithVars(add map[string]string) VariableSet {
	if len(add) == 0 {
		return vs
	}
	clone := vs.clone()
	if clone.vars == nil {
		clone.vars = map[string]string{}
	}
	for k, v := range add {
		clone.vars[k] = v
	}
	return clone
}

// WithTopLevelVars returns a variable set with additional top-level string variables in its environment.
func (vs VariableSet) WithTopLevelVars(add map[string]string) VariableSet {
	if len(add) == 0 {
		return vs
	}
	clone := vs.clone()
	if clone.topLevelVars == nil {
		clone.topLevelVars = map[string]string{}
	}
	for k, v := range add {
		clone.topLevelVars[k] = v
	}
	return clone
}

func (vs VariableSet) register(jvm *jsonnet.VM) {
	registerVars := func(m map[string]string, registrar func(k, v string)) {
		for k, v := range m {
			registrar(k, v)
		}
	}
	registerVars(vs.vars, jvm.ExtVar)
	registerVars(vs.codeVars, jvm.ExtCode)
	registerVars(vs.topLevelVars, jvm.TLAVar)
	registerVars(vs.topLevelCodeVars, jvm.TLACode)
}
