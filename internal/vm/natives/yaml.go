/*
   Copyright 2021 Splunk Inc.

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

package natives

import (
	"io"

	"k8s.io/apimachinery/pkg/util/yaml"
)

// ParseYAMLDocuments parses the contents of the reader into an array of
// objects, one for each non-null document in the input.
func ParseYAMLDocuments(reader io.Reader) ([]interface{}, error) {
	ret := []interface{}{}
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
