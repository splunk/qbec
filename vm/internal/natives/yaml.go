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

package natives

import (
	"io"

	v3yaml "gopkg.in/yaml.v3"
	"k8s.io/apimachinery/pkg/util/yaml"
)

// ParseYAMLDocuments parses the contents of the reader into an array of
// objects, one for each non-nil document in the input.
func ParseYAMLDocuments(reader io.Reader) ([]interface{}, error) {
	ret := make([]interface{}, 0)
	d := yaml.NewYAMLToJSONDecoder(reader)
	for {
		var doc interface{}
		if err := d.Decode(&doc); err != nil {
			if err == io.EOF {
				break
			}
			return nil, err
		}
		if doc != nil {
			ret = append(ret, doc)
		}
	}
	return ret, nil
}

// RenderYAMLDocuments renders the supplied data as a series of YAML documents if the input is an array
// or a single document when it is not. Nils are excluded from output.
// If the caller wants an array to be rendered as a single document,
// they need to wrap it in an array first. Note that this function is not a drop-in replacement for
// data that requires ghodss/yaml to be rendered correctly.
func RenderYAMLDocuments(data interface{}, writer io.Writer) (retErr error) {
	out, ok := data.([]interface{})
	if !ok {
		out = []interface{}{data}
	}
	enc := v3yaml.NewEncoder(writer)
	enc.SetIndent(2)
	defer func() {
		err := enc.Close()
		if err == nil && retErr == nil {
			retErr = err
		}
	}()
	for _, doc := range out {
		if doc == nil {
			continue
		}
		if err := enc.Encode(doc); err != nil {
			return err
		}
	}
	return nil
}
