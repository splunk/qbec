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

package eval

import (
	"encoding/json"
	"fmt"
	"reflect"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

func tolerantJSON(data interface{}) string {
	b, _ := json.MarshalIndent(data, "", "  ")
	return string(b)
}

func str(data map[string]interface{}, attr string) string {
	v := data[attr]
	if s, ok := v.(string); ok {
		return s
	}
	return ""
}

type rawObjectType = int

const (
	unknownType rawObjectType = iota
	leafType
	arrayType
)

func getRawObjectType(data map[string]interface{}) rawObjectType {
	kind := str(data, "kind")
	apiVersion := str(data, "apiVersion")
	if kind == "" || apiVersion == "" {
		return unknownType
	}
	_, isArray := data["items"].([]interface{})
	if isArray { // kubernetes list, not primitive
		return arrayType
	}
	return leafType
}

func walk(data interface{}) ([]map[string]interface{}, error) {
	return walkObjects("$", data, data)
}

func walkObjects(path string, data interface{}, ctx interface{}) ([]map[string]interface{}, error) {
	var ret []map[string]interface{}
	if data == nil {
		return ret, nil
	}
	switch t := data.(type) {
	case []interface{}:
		for i, o := range t {
			objects, err := walkObjects(fmt.Sprintf("%s[%d]", path, i), o, data)
			if err != nil {
				return nil, err
			}
			ret = append(ret, objects...)
		}
	case map[string]interface{}:
		rt := getRawObjectType(t)
		switch rt {
		case arrayType:
			array := t["items"].([]interface{})
			objects, err := walkObjects(fmt.Sprintf("%s.items", path), array, data)
			if err != nil {
				return nil, err
			}
			ret = append(ret, objects...)
		case leafType:
			u := unstructured.Unstructured{Object: t}
			name := u.GetName()
			genName := u.GetGenerateName()
			if name == "" && genName == "" {
				return nil, fmt.Errorf("object (%v) did not have a name at path %q, (json=\n%s)",
					reflect.TypeOf(data),
					path,
					tolerantJSON(data))
			}
			ret = append(ret, t)
		default:
			for k, v := range t {
				objects, err := walkObjects(fmt.Sprintf("%s.%s", path, k), v, data)
				if err != nil {
					return nil, err
				}
				ret = append(ret, objects...)
			}
		}
	default:
		return nil, fmt.Errorf("unexpected type for object (%v) at path %q, (json=\n%s)",
			reflect.TypeOf(data),
			path,
			tolerantJSON(ctx))
	}
	return ret, nil
}
