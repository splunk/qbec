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
	"encoding/base64"
	"encoding/json"
	"fmt"
	"testing"

	"github.com/ghodss/yaml"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var cm = `
---
apiVersion:  v1
kind: ConfigMap
metadata:
  namespace: ns1
  name: cm
data:
  foo: bar
`

var b64 = base64.StdEncoding.EncodeToString([]byte("changeme"))

var secret = fmt.Sprintf(`
---
apiVersion:  v1
kind: Secret
metadata:
  namespace: ns1
  name: s
data:
  foo: %s
`, b64)

func toData(s string) map[string]interface{} {
	var data map[string]interface{}
	err := yaml.Unmarshal([]byte(s), &data)
	if err != nil {
		panic(err)
	}
	return data
}

func TestK8sObject(t *testing.T) {
	obj := NewK8sObject(toData(cm))
	a := assert.New(t)
	a.Equal("cm", obj.GetName())
	a.Equal("ns1", obj.GetNamespace())
	a.Equal("ConfigMap", obj.GetKind())
	a.Equal("", obj.GetObjectKind().GroupVersionKind().Group)
	a.Equal("v1", obj.GetObjectKind().GroupVersionKind().Version)
	a.Equal("ConfigMap", obj.GetObjectKind().GroupVersionKind().Kind)
	a.NotNil(obj.ToUnstructured())
	a.Equal("/v1, Kind=ConfigMap:ns1:cm", fmt.Sprint(obj))
	b, err := json.Marshal(obj)
	require.Nil(t, err)
	s := string(b)
	a.Contains(s, `"foo":"bar"`)
}

func TestK8sLocalObject(t *testing.T) {
	obj := NewK8sLocalObject(toData(cm), "app1", "c1", "e1")
	a := assert.New(t)
	a.Equal("app1", obj.Application())
	a.Equal("c1", obj.Component())
	a.Equal("e1", obj.Environment())
}

func TestSecrets(t *testing.T) {
	cmObj := NewK8sLocalObject(toData(cm), "app1", "c1", "e1")
	secretObj := NewK8sLocalObject(toData(secret), "app1", "c1", "e1")
	a := assert.New(t)
	a.False(HasSensitiveInfo(cmObj.ToUnstructured()))
	a.True(HasSensitiveInfo(secretObj.ToUnstructured()))
	changed, ok := HideSensitiveLocalInfo(cmObj)
	a.Equal(cmObj, changed)
	a.False(ok)
	changed, ok = HideSensitiveLocalInfo(secretObj)
	a.NotEqual(secretObj, changed)
	a.True(ok)
	v := changed.ToUnstructured().Object["data"].(map[string]interface{})["foo"]
	a.NotEqual(b64, v)
}
