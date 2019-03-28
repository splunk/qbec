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
	"sort"

	"github.com/splunk/qbec/internal/model"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

var _ = unstructured.Unstructured{}

func tolerantJSON(data interface{}) string {
	b, err := json.MarshalIndent(data, "", "  ")
	if err != nil {
		return "<unable to serialize object to JSON>"
	}
	return string(b)
}

func str(data map[string]interface{}, attr string) string {
	v := data[attr]
	if s, ok := v.(string); ok {
		return s
	}
	return ""
}

type walker struct {
	app  string
	env  string
	data interface{}
}

func (w *walker) walk() ([]model.K8sLocalObject, error) {
	return w.walkObjects("$", "", w.data)
}

func (w *walker) walkObjects(path string, component string, data interface{}) ([]model.K8sLocalObject, error) {
	var ret []model.K8sLocalObject
	switch t := data.(type) {
	case []interface{}:
		for i, o := range t {
			objects, err := w.walkObjects(fmt.Sprintf("%s[%d]", path, i), component, o)
			if err != nil {
				return nil, err
			}
			ret = append(ret, objects...)
		}
	case map[string]interface{}:
		kind := str(t, "kind")
		apiVersion := str(t, "apiVersion")
		if kind != "" && apiVersion != "" {
			array, isArray := t["items"].([]interface{})
			if isArray { // kubernetes list, extract items
				objects, err := w.walkObjects(fmt.Sprintf("%s.items", path), component, array)
				if err != nil {
					return nil, err
				}
				ret = append(ret, objects...)
			} else {
				ret = append(ret, model.NewK8sLocalObject(t, w.app, component, w.env))
			}
			return ret, nil
		}
		for k, v := range t {
			comp := component
			if component == "" {
				comp = k
			}
			objects, err := w.walkObjects(fmt.Sprintf("%s.%s", path, k), comp, v)
			if err != nil {
				return nil, err
			}
			ret = append(ret, objects...)
		}
	default:
		return nil, fmt.Errorf("unexpected type for object (%v) at path %q, (json=\n%s)",
			reflect.TypeOf(data),
			path,
			tolerantJSON(data))
	}
	return ret, nil
}

func k8sObjectsFromJSON(data map[string]interface{}, app, env string) ([]model.K8sLocalObject, error) {
	w := walker{app: app, env: env, data: data}
	ret, err := w.walk()
	if err != nil {
		return nil, err
	}
	sort.Slice(ret, func(i, j int) bool {
		left := ret[i]
		right := ret[j]
		leftKey := fmt.Sprintf("%s:%s:%s:%s", left.Component(), left.GetNamespace(), left.GetObjectKind().GroupVersionKind().Kind, left.GetName())
		rightKey := fmt.Sprintf("%s:%s:%s:%s", right.Component(), right.GetNamespace(), right.GetObjectKind().GroupVersionKind().Kind, right.GetName())
		return leftKey < rightKey
	})
	return ret, nil
}
