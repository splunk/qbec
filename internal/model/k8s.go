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
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

// K8sKind is something that has a kind.
type K8sKind interface {
	GetKind() string
}

// K8sMeta is the minimum metadata needed for an object and is satisfied by
// an unstructured.Unstructured instance.
type K8sMeta interface {
	K8sKind
	GetObjectKind() schema.ObjectKind
	GetNamespace() string
	GetName() string
}

// QbecMeta provides qbec metadata.
type QbecMeta interface {
	Application() string // the application name
	Component() string   // the component name
	Environment() string // the environment name
	Tag() string         // the GC Tag name
}

// K8sQbecMeta has K8s as well as qbec metadata.
type K8sQbecMeta interface {
	K8sMeta
	QbecMeta
}

// K8sObject is the interface to access a kubernetes object.
// It can be serialized to valid JSON or YAML that represents the fully-formed object.
type K8sObject interface {
	K8sMeta
	ToUnstructured() *unstructured.Unstructured
}

// K8sLocalObject is the interface to access a kubernetes object that has additional qbec attributes.
// It can be serialized to valid JSON or YAML that represents the fully-formed object.
type K8sLocalObject interface {
	K8sObject
	QbecMeta
}

type ko struct {
	*unstructured.Unstructured
	app, comp, tag, env string
}

func (k *ko) Application() string                        { return k.app }
func (k *ko) Tag() string                                { return k.tag }
func (k *ko) Component() string                          { return k.comp }
func (k *ko) Environment() string                        { return k.env }
func (k *ko) MarshalJSON() ([]byte, error)               { return json.Marshal(k.Unstructured) }
func (k *ko) ToUnstructured() *unstructured.Unstructured { return k.Unstructured }
func (k *ko) String() string {
	return fmt.Sprintf("%s:%s:%s", k.GetObjectKind().GroupVersionKind(), k.GetNamespace(), k.GetName())
}

// NewK8sObject wraps a K8sObject implementation around the unstructured object data specified as a bag
// of attributes.
func NewK8sObject(data map[string]interface{}) K8sObject {
	base := &unstructured.Unstructured{Object: data}
	ret := &ko{Unstructured: base}
	return ret
}

// NewK8sLocalObject wraps a K8sLocalObject implementation around the unstructured object data specified as a bag
// of attributes for the supplied application, component and environment.
func NewK8sLocalObject(data map[string]interface{}, app, tag, component, env string) K8sLocalObject {
	base := &unstructured.Unstructured{Object: data}
	ret := &ko{Unstructured: base, app: app, tag: tag, comp: component, env: env}
	labels := base.GetLabels()
	if labels == nil {
		labels = map[string]string{}
	}
	labels[QbecNames.ApplicationLabel] = app
	if tag != "" {
		labels[QbecNames.TagLabel] = tag
	}
	labels[QbecNames.EnvironmentLabel] = env
	base.SetLabels(labels)

	anns := base.GetAnnotations()
	if anns == nil {
		anns = map[string]string{}
	}
	anns[QbecNames.ComponentAnnotation] = component
	base.SetAnnotations(anns)
	return ret
}

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
	h.Write([]byte(realValue))
	shasum := h.Sum(nil)
	return fmt.Sprintf("redacted.%s", base64.RawURLEncoding.EncodeToString(shasum))
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
	secretData, _, _ := unstructured.NestedMap(obj.Object, "data")
	if secretData == nil {
		secretData = map[string]interface{}{}
	}
	changedData := map[string]interface{}{}
	for k, v := range secretData {
		value := obfuscate(fmt.Sprintf("%s:%s", k, v))
		changedData[k] = base64.StdEncoding.EncodeToString([]byte(value))
	}
	clone.Object["data"] = changedData
	return clone, true
}

// HideSensitiveLocalInfo is like HideSensitiveInfo but for local objects.
func HideSensitiveLocalInfo(in K8sLocalObject) (K8sLocalObject, bool) {
	obj, changed := HideSensitiveInfo(in.ToUnstructured())
	if !changed {
		return in, false
	}
	return NewK8sLocalObject(obj.Object, in.Application(), in.Tag(), in.Component(), in.Environment()), true
}
