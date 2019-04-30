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

package remote

import (
	"io/ioutil"
	"path/filepath"
	"testing"

	"github.com/golang/protobuf/proto"
	openapi_v2 "github.com/googleapis/gnostic/OpenAPIv2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

type sd struct{}

func (d sd) OpenAPISchema() (*openapi_v2.Document, error) {
	b, err := ioutil.ReadFile(filepath.Join("testdata", "swagger-2.0.0.pb-v1"))
	if err != nil {
		return nil, err
	}
	var doc openapi_v2.Document
	if err := proto.Unmarshal(b, &doc); err != nil {
		return nil, err
	}
	return &doc, nil
}

func TestMetadataValidator(t *testing.T) {
	a := assert.New(t)
	ss := newServerSchema(sd{})
	v, err := ss.validatorFor(schema.GroupVersionKind{Group: "", Version: "v1", Kind: "Namespace"})
	require.Nil(t, err)
	errs := v.Validate(loadObject(t, "ns-good.json").ToUnstructured())
	require.Nil(t, errs)

	errs = v.Validate(loadObject(t, "ns-bad.json").ToUnstructured())
	require.NotNil(t, errs)
	a.Equal(1, len(errs))
	a.Contains(errs[0].Error(), `unknown field "foo"`)

	_, err = ss.validatorFor(schema.GroupVersionKind{Group: "", Version: "v1", Kind: "FooBar"})
	require.NotNil(t, err)
	a.Equal(ErrSchemaNotFound, err)

}
