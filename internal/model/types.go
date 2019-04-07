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

package model

//go:generate gen-qbec-swagger swagger.yaml swagger-schema.go

// Environment points to a specific destination and has its own set of runtime parameters.
type Environment struct {
	DefaultNamespace string   `json:"defaultNamespace"`   // default namespace to set for k8s context
	Server           string   `json:"server"`             // server URL of server
	Includes         []string `json:"includes,omitempty"` // components to be included in this env even if excluded at the app level
	Excludes         []string `json:"excludes,omitempty"` // additional components to exclude for this env
}

// Var is a base variable.
type Var struct {
	Name   string `json:"name"`             // variable name
	Secret bool   `json:"secret,omitempty"` // true if the variable is a secret
}

// TopLevelVar is a variable that is set as a TLA in the jsonnet VM. Note that there is no provision to set
// a default value - default values should be set in the jsonnet code instead.
type TopLevelVar struct {
	Var
	Components []string `json:"components,omitempty"` // the components for which this TLA is applicable
}

// ExternalVar is a variable that is set as an extVar in the jsonnet VM
type ExternalVar struct {
	Var
	Default interface{} `json:"default,omitempty"` // the default value to use if none specified on the command line.
}

// Variables is a collection of external and top-level variables.
type Variables struct {
	External []ExternalVar `json:"external,omitempty"` // collection of ext vars
	TopLevel []TopLevelVar `json:"topLevel,omitempty"` // collection of TLAs
}

// AppMeta is the simplified metadata object for a qbec app.
type AppMeta struct {
	// required: true
	Name string `json:"name"`
}

// AppSpec is the user-supplied configuration of the qbec app.
type AppSpec struct {
	// directory containing component files, default to components/
	ComponentsDir string `json:"componentsDir,omitempty"`
	// standard file containing parameters for all environments returning correct values based on qbec.io/env external
	// variable, defaults to params.libsonnet
	ParamsFile string `json:"paramsFile,omitempty"`
	// the interface for jsonnet variables.
	Vars Variables `json:"vars,omitempty"`
	// set of environments for the app
	// required: true
	Environments map[string]Environment `json:"environments"`
	// list of components to exclude by default for every environment
	Excludes []string `json:"excludes,omitempty"`
	// list of library paths to add to the jsonnet VM at evaluation
	LibPaths []string `json:"libPaths,omitempty"`
}

// QbecApp is a set of components that can be applied to multiple environments with tweaked runtime configurations.
// The list of all components for the app is derived as all the supported (jsonnet, json, yaml) files in the components subdirectory.
// swagger:model App
type QbecApp struct {
	// object kind
	// required: true
	// pattern: ^App$
	Kind string `json:"kind"`
	// requested API version
	// required: true
	APIVersion string `json:"apiVersion"`
	// app metadata
	// required: true
	Metadata AppMeta `json:"metadata,omitempty"`
	// app specification
	// required: true
	Spec AppSpec `json:"spec"`
}
