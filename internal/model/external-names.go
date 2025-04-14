// Copyright 2025 Splunk Inc.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package model

// QBECMetadataPrefix is the leading path for all metadata set by qbec.
const QBECMetadataPrefix = "qbec.io/"

// QBECDirectivesNamespace is the leading path for all directives set by the user for qbec use.
const QBECDirectivesNamespace = "directives.qbec.io/"

// Directives is the list of directive names we support.
type Directives struct {
	ApplyOrder   string // numeric apply order for object
	DeletePolicy string // delete policy "default" | "never"
	UpdatePolicy string // update policy "default" | "never"
	WaitPolicy   string // wait policy "default" | "never"
}

// QbecNames is the set of names used by Qbec.
var QbecNames = struct {
	ApplicationLabel    string // the label to use for tagging an object with an application name
	TagLabel            string // the label to use for tagging an object with a scoped GC tag
	ComponentAnnotation string // the label to use for tagging an object with a component
	ComponentLabel      string // the label to use for tagging an object with a component
	EnvironmentLabel    string // the label to use for tagging an object with an annotation
	PristineAnnotation  string // the annotation to use for storing the pristine object
	EnvVarName          string // the name of the external variable that has the environment name
	EnvPropsVarName     string // the name of the external variable that has the environment properties object
	TagVarName          string // the name of the external variable that has the tag name
	DefaultNsVarName    string // the name of the external variable that has the default namespace
	CleanModeVarName    string // name of external variable that has the indicator for clean mode
	Directives          Directives
}{
	ApplicationLabel:    QBECMetadataPrefix + "application",
	TagLabel:            QBECMetadataPrefix + "tag",
	ComponentAnnotation: QBECMetadataPrefix + "component",
	ComponentLabel:      QBECMetadataPrefix + "component",
	EnvironmentLabel:    QBECMetadataPrefix + "environment",
	PristineAnnotation:  QBECMetadataPrefix + "last-applied",
	EnvVarName:          QBECMetadataPrefix + "env",
	EnvPropsVarName:     QBECMetadataPrefix + "envProperties",
	TagVarName:          QBECMetadataPrefix + "tag",
	DefaultNsVarName:    QBECMetadataPrefix + "defaultNs",
	CleanModeVarName:    QBECMetadataPrefix + "cleanMode",
	Directives: Directives{
		ApplyOrder:   QBECDirectivesNamespace + "apply-order",
		DeletePolicy: QBECDirectivesNamespace + "delete-policy",
		UpdatePolicy: QBECDirectivesNamespace + "update-policy",
		WaitPolicy:   QBECDirectivesNamespace + "wait-policy",
	},
}
