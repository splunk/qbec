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

package commands

import "strings"

type example struct {
	command  string
	comments []string
}

func (e example) String() string {
	if len(e.comments) == 0 {
		return e.command
	}
	var prefixed []string
	for _, c := range e.comments {
		prefixed = append(prefixed, "# "+c)
	}
	return strings.Join(prefixed, "\n") + "\n" + e.command
}

func newExample(command string, comments ...string) example {
	return example{command: "qbec " + command, comments: comments}
}

func exampleHelp(examples ...example) string {
	var ret []string
	for _, eg := range examples {
		ret = append(ret, eg.String())
	}
	return "\n" + strings.Join(ret, "\n\n")
}

func applyExamples() string {
	return exampleHelp(
		newExample("apply dev --yes --wait", "create/ update all dev components and delete extra objects on the server",
			"do not ask for confirmation, wait until all objects have a ready status"),
		newExample("apply -n dev", "show what apply would do for the dev environment"),
		newExample("apply dev -c redis -K secret", "update all objects except secrets just for the redis component"),
		newExample("apply dev --gc=false", "only create/ update, do not delete extra objects from the server"),
	)
}

func showExamples() string {
	return exampleHelp(
		newExample("show dev", "show all components for the 'dev' environment in YAML"),
		newExample("show dev -c postgres -c redis -o json", "expand just 2 components and output JSON"),
		newExample("show dev -C postgres -C redis", "expand all but 2 components"),
		newExample("show dev -k deployment -k configmap", "show only deployments and config maps"),
		newExample("show dev -K secret", "show all objects except secrets"),
		newExample("show dev -O", "list all objects for the dev environment"),
	)
}

func fmtExamples() string {
	return exampleHelp(
		newExample("alpha fmt -w", "format all files in-place"),
		newExample("alpha fmt -e", "check if all files are formatted well. Non zero exit code in case a unformatted file is found"),
		newExample("alpha fmt --type=jsonnet", "format all jsonnet and libsonnet files to stdout"),
		newExample("alpha fmt somefolder file1.jsonnet file2.libsonnet", "format all json, jsonnet, libsonnet and yaml files in the somefolder, file1.jsonnet and file2.libsonnet files to stdout"),
		newExample("alpha fmt -t=yaml", "format all yaml files to stdout"),
		newExample("alpha fmt --type-json,yaml somefolder file1.yaml file2.yml file3.json", "format all json and yaml files in the somefolder, file1.yaml, file2.yml and file3.json files to stdout"),
	)
}

func deleteExamples() string {
	return exampleHelp(
		newExample("delete dev", "delete all objects created for the dev environment"),
		newExample("delete -n dev", "show objects that would be deleted for the dev environment"),
		newExample("delete dev -c redis -k secret", "delete all secrets for the redis component"),
		newExample("delete dev --local", "use object names from local component files for deletion list",
			"by default, the list is produced using server queries"),
	)
}

func diffExamples() string {
	return exampleHelp(
		newExample("diff dev", "show differences between local and remote objects for the dev environment"),
		newExample("diff dev -c redis --show-deletes=false", "show differences for the redis component for the dev environment",
			"ignore extra remote objects"),
		newExample("diff dev -ignore-all-labels", "do not take labels into account when calculating the diff"),
	)
}

func validateExamples() string {
	return exampleHelp(
		newExample("validate dev", "validate all objects for all components against the dev environment"),
	)
}

func componentListExamples() string {
	return exampleHelp(
		newExample("component list dev", "list all components for the dev environment"),
		newExample("component list dev -O", "list all objects for the dev environment"),
		newExample("component list _", "list all baseline components"),
	)
}

func componentDiffExamples() string {
	return exampleHelp(
		newExample("component diff dev", "show differences in component lists between baseline and dev"),
		newExample("component diff dev prod -O", "show differences in object lists between dev and prod"),
	)
}

func paramListExamples() string {
	return exampleHelp(
		newExample("param list dev", "list all parameters for the dev environment"),
		newExample("param list _", "list baseline parameters for all components"),
		newExample("param list dev -c redis", "list parameters for the redis component for the dev environment"),
	)
}

func paramDiffExamples() string {
	return exampleHelp(
		newExample("param diff dev", "show differences in parameter values between baseline and dev"),
		newExample("param diff dev prod", "show differences in parameter values  between dev and prod"),
	)
}

func envListExamples() string {
	return exampleHelp(
		newExample("env list", "list all environment names, one per line in sorted order"),
		newExample("env list -o json", "list all environments in JSON format, (use -o yaml for YAML)"),
	)
}

func envVarsExamples() string {
	return exampleHelp(
		newExample("env vars <env>", "print kubernetes variables for env in eval format, run as `eval $(qbec env vars env)`"),
		newExample("env vars -o json", "print kubernetes variables for env in JSON format, (use -o yaml for YAML)"),
	)
}
