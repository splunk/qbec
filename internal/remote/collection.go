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

package remote

import (
	"fmt"

	"github.com/splunk/qbec/internal/model"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

type objectKey struct {
	gvk       schema.GroupVersionKind
	namespace string
	name      string
}

func (o objectKey) GroupVersionKind() schema.GroupVersionKind { return o.gvk }
func (o objectKey) GetKind() string                           { return o.gvk.Kind }
func (o objectKey) GetNamespace() string                      { return o.namespace }
func (o objectKey) GetName() string                           { return o.name }

type basicObject struct {
	objectKey
	app       string
	tag       string
	component string
	env       string
	anns      map[string]string
}

func (b *basicObject) Application() string               { return b.app }
func (b *basicObject) Tag() string                       { return b.tag }
func (b *basicObject) Component() string                 { return b.component }
func (b *basicObject) Environment() string               { return b.env }
func (b *basicObject) GetGenerateName() string           { return "" }
func (b *basicObject) GetAnnotations() map[string]string { return b.anns }

type collectMetadata interface {
	objectNamespace(obj model.K8sMeta) string
	canonicalGroupVersionKind(in schema.GroupVersionKind) (schema.GroupVersionKind, error)
}

type collection struct {
	defaultNs string
	meta      collectMetadata
	objects   map[objectKey]model.K8sQbecMeta
}

func newCollection(defaultNs string, meta collectMetadata) *collection {
	if defaultNs == "" {
		defaultNs = "default"
	}
	return &collection{
		defaultNs: defaultNs,
		meta:      meta,
		objects:   map[objectKey]model.K8sQbecMeta{},
	}
}

// add adds the supplied object potentially transforming its gvk to its canonical form.
func (c *collection) add(object model.K8sQbecMeta) error {
	gvk := object.GroupVersionKind()
	canonicalGVK, err := c.meta.canonicalGroupVersionKind(gvk)
	if err != nil {
		return err
	}
	if object.GetName() == "" {
		return fmt.Errorf("internal error: object %v did not have a name", object)
	}
	ns := c.meta.objectNamespace(object)
	key := objectKey{
		gvk:       canonicalGVK,
		namespace: ns,
		name:      object.GetName(),
	}
	resultObject := &basicObject{
		objectKey: key,
		app:       object.Application(),
		tag:       object.Tag(),
		component: object.Component(),
		env:       object.Environment(),
		anns:      object.GetAnnotations(),
	}
	c.objects[key] = resultObject
	return nil
}

// Remove removes objects from its internal collection for each
// matching object supplied.
func (c *collection) Remove(objs []model.K8sQbecMeta) error {
	sub := newCollection(c.defaultNs, c.meta)
	for _, o := range objs {
		if err := sub.add(o); err != nil {
			return err
		}
	}
	retainedSet := map[objectKey]model.K8sQbecMeta{}
	for k, v := range c.objects {
		if _, ok := sub.objects[k]; !ok {
			retainedSet[k] = v
		}
	}
	c.objects = retainedSet
	return nil
}

// ToList returns the list of objects in this collection in arbitrary order.
func (c *collection) ToList() []model.K8sQbecMeta {
	var ret []model.K8sQbecMeta
	for _, v := range c.objects {
		ret = append(ret, v)
	}
	return ret
}
