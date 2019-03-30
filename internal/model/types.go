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

// Variable is a late-bound value that is not known in advance (e.g. an image tag in a CI build) or should not be
// checked into source control (e.g. secrets). The name is an identifier.
//
// The value of the variable, if not passed in explicitly, is derived from an environment variable of the form
// QBEC_VAR_MODIFIED_NAME where MODIFIED_NAME is uppercase version of the variable name converted to snake-case.
// (e.g. a variable with name fooBar is picked up from the QBEC_VAR_FOO_BAR environment variable).
//
// In the jsonnet code, the value of the variable can be gotten from the external variable called `vars.qbec.io/name`
// (e.g. the fooBar variable can be accessed as std.extVar('vars.qbec.io/fooBar')).
//
// Variables must NOT have a blank value. A blank string is treated as equivalent to not setting it.
//
// `qbec apply` MUST set all variables at the time of invoking the command either via environment variables
// or on the command line. Other commands (including `qbec apply --dry-run`) need to only set the variables
// that do not have a default value.
//
// A variable may be tagged as a secret in which case it can only be set from the
// environment and its value is obfuscated by default (this may be displayed using the --show-secrets flag).
//
// Variables may be optionally associated to components. Doing this has the following implications.
//
// - The variable value is available only for the associated components.
//
// - The caller need not set the variable value if all of its associated components have been excluded from the
//   apply command using component filters.
type Variable struct {
	Name       string   `json:"name"`                 // variable name
	Default    string   `json:"default,omitempty"`    // default value, blank requires the variable to be set.
	Components []string `json:"components,omitempty"` // components that use the variable, optional
	Secret     bool     `json:"secret"`               // true if the variable's value is a secret
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
	// set of environments for the app
	// required: true
	Environments map[string]Environment `json:"environments"`
	// list of components to exclude by default for every environment
	Excludes []string `json:"excludes,omitempty"`
	// list of library paths to add to the jsonnet VM at evaluation
	LibPaths []string `json:"libPaths,omitempty"`
	// list of late-bound variables for the app
	Vars []Variable `json:"vars,omitempty"`
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
