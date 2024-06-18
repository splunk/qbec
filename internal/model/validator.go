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
	"encoding/json"
	"fmt"
	"strings"

	"github.com/ghodss/yaml"
	"github.com/go-openapi/spec"
	"github.com/go-openapi/strfmt"
	"github.com/go-openapi/validate"
	"github.com/pkg/errors"
)

// LatestAPIVersion is the latest version of the API we support.
const LatestAPIVersion = "qbec.io/v1alpha1"

type validator struct {
	swagger spec.Swagger
}

func newValidator() (*validator, error) {
	var v validator
	if err := json.Unmarshal([]byte(swaggerJSON), &v.swagger); err != nil {
		return nil, errors.Wrap(err, "load swagger")
	}
	defs := v.swagger.Definitions
	if defs == nil {
		return nil, fmt.Errorf("unable to find definitions in swagger doc")
	}
	return &v, nil
}

func (v *validator) validateYAML(content []byte) []error {
	wrap := func(err error) []error {
		return []error{err}
	}
	var data map[string]interface{}
	if err := yaml.Unmarshal(content, &data); err != nil {
		return wrap(errors.Wrap(err, "YAML unmarshal"))
	}
	apiVersion, ok := data["apiVersion"].(string)
	if !ok {
		return wrap(fmt.Errorf("missing or invalid apiVersion property"))
	}
	kind, ok := data["kind"].(string)
	if !ok {
		return wrap(fmt.Errorf("missing or invalid kind property"))
	}
	if kind != "App" {
		return wrap(fmt.Errorf("bad kind property, expected App"))
	}

	dataType := strings.Replace(apiVersion, "/", ".", -1) + "." + kind
	schema, ok := v.swagger.Definitions[dataType]
	if !ok {
		return wrap(fmt.Errorf("no schema found for %s (check for valid apiVersion and kind properties)", dataType))
	}
	ov := validate.NewSchemaValidator(&schema, v.swagger, "", strfmt.Default)
	res := ov.Validate(data)
	return res.Errors
}

func (v *validator) validateEnvYAML(content []byte) []error {
	wrap := func(err error) []error {
		return []error{err}
	}
	var data map[string]interface{}
	if err := yaml.Unmarshal(content, &data); err != nil {
		return wrap(errors.Wrap(err, "YAML unmarshal"))
	}
	apiVersion, ok := data["apiVersion"].(string)
	if !ok {
		return wrap(fmt.Errorf("missing or invalid apiVersion property"))
	}
	kind, ok := data["kind"].(string)
	if !ok {
		return wrap(fmt.Errorf("missing or invalid kind property"))
	}
	if kind != "EnvironmentMap" {
		return wrap(fmt.Errorf("bad kind property, expected EnvironmentMap"))
	}

	dataType := strings.Replace(apiVersion, "/", ".", -1) + "." + kind
	schema, ok := v.swagger.Definitions[dataType]
	if !ok {
		return wrap(fmt.Errorf("no schema found for %s (check for valid apiVersion and kind properties)", dataType))
	}
	ov := validate.NewSchemaValidator(&schema, v.swagger, "", strfmt.Default)
	res := ov.Validate(data)
	return res.Errors
}

func (v *validator) validateVarYAML(content []byte) []error {
	wrap := func(err error) []error {
		return []error{err}
	}
	var data map[string]interface{}
	if err := yaml.Unmarshal(content, &data); err != nil {
		return wrap(errors.Wrap(err, "YAML unmarshal"))
	}
	apiVersion, ok := data["apiVersion"].(string)
	if !ok {
		return wrap(fmt.Errorf("missing or invalid apiVersion property"))
	}
	kind, ok := data["kind"].(string)
	if !ok {
		return wrap(fmt.Errorf("missing or invalid kind property"))
	}
	if kind != "VariablesFile" {
		return wrap(fmt.Errorf("bad kind property, expected VariablesFile"))
	}

	dataType := strings.Replace(apiVersion, "/", ".", -1) + "." + kind
	schema, ok := v.swagger.Definitions[dataType]
	if !ok {
		return wrap(fmt.Errorf("no schema found for %s (check for valid apiVersion and kind properties)", dataType))
	}
	ov := validate.NewSchemaValidator(&schema, v.swagger, "", strfmt.Default)
	res := ov.Validate(data)
	return res.Errors
}
