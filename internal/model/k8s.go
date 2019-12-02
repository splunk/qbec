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
	"encoding/json"
	"fmt"
	"strings"

	"github.com/pkg/errors"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

// K8sMeta is the minimum metadata needed for an object and is satisfied by
// an unstructured.Unstructured instance.
type K8sMeta interface {
	GetKind() string
	GroupVersionKind() schema.GroupVersionKind
	GetNamespace() string
	GetName() string
	GetGenerateName() string
	GetAnnotations() map[string]string
}

// NameForDisplay returns the local name of the metadata object, taking
// generated names into account.
func NameForDisplay(m K8sMeta) string {
	name := m.GetName()
	if name != "" {
		return name
	}
	return m.GetGenerateName() + "<xxxxx>"
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
	return fmt.Sprintf("%s:%s:%s", k.GroupVersionKind(), k.GetNamespace(), k.GetName())
}

func toUnstructured(data map[string]interface{}) *unstructured.Unstructured {
	base := &unstructured.Unstructured{Object: data}
	if base.GetName() != "" && base.GetGenerateName() != "" { // if a name is specified for the object, nuke its generated name since it won't be used
		base.SetGenerateName("")
	}
	return base
}

// AssertMetadataValid asserts that the object metadata for the supplied unstructured object is valid.
func AssertMetadataValid(data map[string]interface{}) error {
	fixup := func(err error) error {
		if err == nil {
			return err
		}
		// allow nil values which leads to an error in NestedStringMap
		if strings.Contains(err.Error(), "<nil> is of the type <nil>") {
			return nil
		}
		return err
	}
	decorate := func(err error) error {
		if err == nil {
			return nil
		}
		obj := NewK8sObject(data)
		name := fmt.Sprintf("%s, Name=%s", obj.GroupVersionKind(), NameForDisplay(obj))
		return errors.Wrap(err, name)
	}
	if _, _, err := unstructured.NestedStringMap(data, "metadata", "labels"); fixup(err) != nil {
		return decorate(err)
	}
	if _, _, err := unstructured.NestedStringMap(data, "metadata", "annotations"); fixup(err) != nil {
		return decorate(err)
	}
	return nil
}

// NewK8sObject wraps a K8sObject implementation around the unstructured object data specified as a bag
// of attributes.
func NewK8sObject(data map[string]interface{}) K8sObject {
	return &ko{Unstructured: toUnstructured(data)}
}

// NewK8sLocalObject wraps a K8sLocalObject implementation around the unstructured object data specified as a bag
// of attributes for the supplied application, component and environment.
func NewK8sLocalObject(data map[string]interface{}, app, tag, component, env string) K8sLocalObject {
	base := toUnstructured(data)
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
