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

import (
	"fmt"
	"strings"
)

//go:generate gen-qbec-swagger swagger.yaml swagger-schema.go

// Environment points to a specific destination and has its own set of runtime parameters.
type Environment struct {
	DefaultNamespace string                 `json:"defaultNamespace"`     // default namespace to set for k8s context
	Server           string                 `json:"server,omitempty"`     // server URL of server, must be present unless
	Context          string                 `json:"context,omitempty"`    // named context to use instead of deriving from server URL
	Includes         []string               `json:"includes,omitempty"`   // components to be included in this env even if excluded at the app level
	Excludes         []string               `json:"excludes,omitempty"`   // additional components to exclude for this env
	Properties       map[string]interface{} `json:"properties,omitempty"` // properties attached to the environment, exposed via an extvar
}

func (e Environment) assertValid() error {
	if e.Server == "" && e.Context == "" {
		return fmt.Errorf("neither server nor context was set")
	}
	if e.Server != "" && e.Context != "" {
		return fmt.Errorf("only one of server or context may be set")
	}
	if strings.HasPrefix(e.Context, "__") { // do not allow context to be a keyword
		return fmt.Errorf("context for environment ('%s') may not start with __", e.Context)
	}
	return nil
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

// ComputedVar is a variable that is computed based on evaluating jsonnet code.
type ComputedVar struct {
	Var
	Code string `json:"code"` // inline code
}

// Variables is a collection of external and top-level variables.
type Variables struct {
	External []ExternalVar `json:"external,omitempty"` // collection of ext vars
	TopLevel []TopLevelVar `json:"topLevel,omitempty"` // collection of TLAs
	Computed []ComputedVar `json:"computed,omitempty"` // ordered collection of computed vars
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
	// file containing jsonnet code that can be used to post-process all objects, typically adding metadata like
	// annotations.
	PostProcessor string `json:"postProcessor,omitempty"`
	// the interface for jsonnet variables.
	Vars Variables `json:"vars,omitempty"`
	// data sources defined for the app.
	DataSources []string `json:"dataSources,omitempty"`
	// example outputs for data sources for linter use
	DataSourceExamples map[string]interface{} `json:"dsExamples,omitempty"`
	// set of environments for the app
	Environments map[string]Environment `json:"environments"`
	// additional environments pulled in from external files
	EnvFiles []string `json:"envFiles,omitempty"`
	// list of components to exclude by default for every environment
	Excludes []string `json:"excludes,omitempty"`
	// list of library paths to add to the jsonnet VM at evaluation
	LibPaths []string `json:"libPaths,omitempty"`
	// automatically suffix default namespace defined for environment when app-tag provided.
	NamespaceTagSuffix bool `json:"namespaceTagSuffix,omitempty"`
	// properties for the baseline environment, can be used to define what env properties should look like
	BaseProperties map[string]interface{} `json:"baseProperties,omitempty"`
	// whether remote lists for GC purposes should use cluster scoped queries
	// when multiple namespaces are present. Not used when only one namespace is present.
	ClusterScopedLists bool `json:"clusterScopedLists,omitempty"`
	// add component name as label to Kubernetes objects, default to false
	AddComponentLabel bool `json:"addComponentLabel,omitempty"`
}

// QbecEnvironmentMapSpec is the spec for a QbecEnvironmentMap object.
type QbecEnvironmentMapSpec struct {
	// set of declared environments, keyed by name
	// required: true
	Environments map[string]Environment `json:"environments"`
}

// QbecEnvironmentMap is a standalone object that contains a map of environments keyed by name.
type QbecEnvironmentMap struct {
	// object kind
	// required: true
	// pattern: ^Environments$
	Kind string `json:"kind"`
	// requested API version
	// required: true
	APIVersion string `json:"apiVersion"`
	// environments spec
	// require: true
	Spec QbecEnvironmentMapSpec `json:"spec"`
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
