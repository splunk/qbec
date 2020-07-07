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

package k8smeta

import (
	"fmt"
	"sync"

	openapi_v2 "github.com/googleapis/gnostic/OpenAPIv2"
	"github.com/pkg/errors"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/kube-openapi/pkg/util/proto"
	"k8s.io/kube-openapi/pkg/util/proto/validation"
	"k8s.io/kubectl/pkg/util/openapi"
)

// ErrSchemaNotFound is returned when a schema could not be found.
var ErrSchemaNotFound = errors.New("schema not found") // returned when a validation schema is not found

// Validator validates documents of a specific type.
type Validator interface {
	// Validate validates the supplied object and returns a slice of validation errors.
	Validate(obj *unstructured.Unstructured) []error
}

// vsSchema implements Validator
type vsSchema struct {
	proto.Schema
}

func (v *vsSchema) Validate(obj *unstructured.Unstructured) []error {
	gvk := obj.GroupVersionKind()
	return validation.ValidateModel(obj.UnstructuredContent(), v.Schema, fmt.Sprintf("%s.%s", gvk.Version, gvk.Kind))
}

type schemaResult struct {
	validator Validator
	err       error
}

// validators produces Validator instances for k8s types.
type validators struct {
	res   openapi.Resources
	l     sync.Mutex
	cache map[schema.GroupVersionKind]*schemaResult
}

func (v *validators) validatorFor(gvk schema.GroupVersionKind) (Validator, error) {
	v.l.Lock()
	defer v.l.Unlock()
	sr := v.cache[gvk]
	if sr == nil {
		var err error
		valSchema := v.res.LookupResource(gvk)
		if valSchema == nil {
			err = ErrSchemaNotFound
		}
		sr = &schemaResult{
			validator: &vsSchema{valSchema},
			err:       err,
		}
		v.cache[gvk] = sr
	}
	return sr.validator, sr.err
}

// openapiResourceResult is the cached result of retrieving an openAPI doc from the server.
type openapiResourceResult struct {
	res        openapi.Resources
	validators *validators
	err        error
}

// SchemaDiscovery is the minimal interface needed to discover the server schema.
type SchemaDiscovery interface {
	OpenAPISchema() (*openapi_v2.Document, error)
}

// ServerSchema is a representation of the resource schema of a Kubernetes server.
type ServerSchema struct {
	ol      sync.Mutex
	oResult *openapiResourceResult
	disco   SchemaDiscovery
}

// NewServerSchema returns a server schema that can supply validators for the given discovery
// interface.
func NewServerSchema(disco SchemaDiscovery) *ServerSchema {
	return &ServerSchema{
		disco: disco,
	}
}

// ValidatorFor returns a validator for the supplied GroupVersionKind.
func (ss *ServerSchema) ValidatorFor(gvk schema.GroupVersionKind) (Validator, error) {
	_, v, err := ss.openAPIResources()
	if err != nil {
		return nil, err
	}
	return v.validatorFor(gvk)
}

// OpenAPIResources returns the OpenAPI resources for the server.
func (ss *ServerSchema) OpenAPIResources() (openapi.Resources, error) {
	r, _, err := ss.openAPIResources()
	return r, err
}

func (ss *ServerSchema) openAPIResources() (openapi.Resources, *validators, error) {
	ss.ol.Lock()
	defer ss.ol.Unlock()
	ret := ss.oResult
	if ret != nil {
		return ret.res, ret.validators, ret.err
	}
	handle := func(r openapi.Resources, err error) (openapi.Resources, *validators, error) {
		ss.oResult = &openapiResourceResult{res: r, err: err}
		if err == nil {
			ss.oResult.validators = &validators{
				res:   r,
				cache: map[schema.GroupVersionKind]*schemaResult{},
			}
		}
		return ss.oResult.res, ss.oResult.validators, ss.oResult.err
	}
	doc, err := ss.disco.OpenAPISchema()
	if err != nil {
		return handle(nil, errors.Wrap(err, "Open API doc from server"))
	}
	res, err := openapi.NewOpenAPIData(doc)
	if err != nil {
		return handle(nil, errors.Wrap(err, "get resources from validator"))
	}
	return handle(res, nil)
}
