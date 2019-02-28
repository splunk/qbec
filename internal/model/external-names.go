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

const qbecLeading = "qbec.io"

// QbecNames is the set of names used by Qbec.
var QbecNames = struct {
	ApplicationLabel    string // the label to use for tagging an object with an application name
	ComponentAnnotation string // the label to use for tagging an object with a component
	EnvironmentLabel    string // the label to use for tagging an object with an annotation
	PristineAnnotation  string // the annotation to use for storing the pristine object
	ParamsCodeVarName   string // the name of the code variable that stores env params
	EnvVarName          string // the name of the external variable that has the environment name
}{
	ApplicationLabel:    qbecLeading + "/application",
	ComponentAnnotation: qbecLeading + "/component",
	EnvironmentLabel:    qbecLeading + "/environment",
	PristineAnnotation:  qbecLeading + "/last-applied",
	ParamsCodeVarName:   qbecLeading + "/params",
	EnvVarName:          qbecLeading + "/env",
}
