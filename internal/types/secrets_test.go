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

package types

import (
	"encoding/base64"
	"fmt"
	"testing"

	"sigs.k8s.io/yaml"
	"github.com/splunk/qbec/internal/model"
	"github.com/stretchr/testify/assert"
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

func TestSecrets(t *testing.T) {
	cmObj := model.NewK8sLocalObject(toData(cm), model.LocalAttrs{App: "app1", Tag: "", Component: "c1", Env: "e1"})
	secretObj := model.NewK8sLocalObject(toData(secret), model.LocalAttrs{App: "app1", Component: "c1", Env: "e1"})
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
