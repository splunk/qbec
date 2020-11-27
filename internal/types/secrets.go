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

package types

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"fmt"

	"github.com/splunk/qbec/internal/model"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

var randomPrefix string

func initRandomPrefix() {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		panic(fmt.Errorf("unable to initialize random prefix: %v", err))
	}
	randomPrefix = base64.RawURLEncoding.EncodeToString(b)
}

func init() {
	initRandomPrefix()
}

func obfuscate(value string) string {
	realValue := randomPrefix + ":" + value
	h := sha256.New()
	_, _ = h.Write([]byte(realValue)) // guaranteed to never fail per docs
	shasum := h.Sum(nil)
	return fmt.Sprintf("redacted.%s", base64.RawURLEncoding.EncodeToString(shasum))
}

func obfuscateMap(in map[string]interface{}) map[string]interface{} {
	if len(in) == 0 {
		return nil
	}
	ret := map[string]interface{}{}
	for k, v := range in {
		value := obfuscate(fmt.Sprintf("%s:%s", k, v))
		ret[k] = base64.StdEncoding.EncodeToString([]byte(value))
	}
	return ret
}

// HasSensitiveInfo returns true if the supplied object has sensitive data that might need
// to be hidden.
func HasSensitiveInfo(obj *unstructured.Unstructured) bool {
	gk := obj.GroupVersionKind().GroupKind()
	return gk.Group == "" && gk.Kind == "Secret"
}

// HideSensitiveInfo creates a new object for secrets where secret values have been replaced with
// stable strings that can still be diff-ed. It returns a boolean to indicate that the return value
// was modified from the original object. When no modifications are needed, the original object
// is returned as-is.
func HideSensitiveInfo(obj *unstructured.Unstructured) (*unstructured.Unstructured, bool) {
	if obj == nil {
		return obj, false
	}
	if !HasSensitiveInfo(obj) {
		return obj, false
	}

	clone := obj.DeepCopy()
	for _, section := range []string{"data", "stringData"} {
		secretData, _, _ := unstructured.NestedMap(obj.Object, section)
		changed := obfuscateMap(secretData)
		if changed != nil {
			clone.Object[section] = changed
		}
	}
	return clone, true
}

// HideSensitiveLocalInfo is like HideSensitiveInfo but for local objects.
func HideSensitiveLocalInfo(in model.K8sLocalObject) (model.K8sLocalObject, bool) {
	obj, changed := HideSensitiveInfo(in.ToUnstructured())
	if !changed {
		return in, false
	}
	return model.NewK8sLocalObject(obj.Object, in.Application(), in.Tag(), in.Component(), in.Environment()), true
}
