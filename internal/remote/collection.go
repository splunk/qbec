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

package remote

import (
	"sort"

	"github.com/splunk/qbec/internal/model"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

type objectKey struct {
	gvk       schema.GroupVersionKind
	namespace string
	name      string
}

type basicObject struct {
	objectKey
	app       string
	tag       string
	component string
	env       string
}

func (b *basicObject) GetObjectKind() schema.ObjectKind                { return b }
func (b *basicObject) GroupVersionKind() schema.GroupVersionKind       { return b.gvk }
func (b *basicObject) GetGroupVersionKind() schema.GroupVersionKind    { return b.gvk }
func (b *basicObject) SetGroupVersionKind(gvk schema.GroupVersionKind) { b.gvk = gvk }
func (b *basicObject) GetKind() string                                 { return b.gvk.Kind }
func (b *basicObject) GetNamespace() string                            { return b.namespace }
func (b *basicObject) GetName() string                                 { return b.name }
func (b *basicObject) Application() string                             { return b.app }
func (b *basicObject) Tag() string                                     { return b.tag }
func (b *basicObject) Component() string                               { return b.component }
func (b *basicObject) Environment() string                             { return b.env }

type collectMetadata interface {
	IsNamespaced(gvk schema.GroupVersionKind) (bool, error)
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

type collectionStats struct {
	namespaces            []string                  // distinct namespaces across all objects
	namespacedObjectCount int                       // count of namespaced objects
	clusterObjectCount    int                       // count of cluster objects
	types                 []schema.GroupVersionKind // distinct types
}

func (c *collection) stats() collectionStats {
	var ret collectionStats
	seenGVK := map[schema.GroupVersionKind]bool{}
	seenNS := map[string]bool{}
	for _, v := range c.objects {
		ns := v.GetNamespace()
		if ns == "" {
			ret.clusterObjectCount++
		} else {
			ret.namespacedObjectCount++
		}
		seenNS[ns] = true
		seenGVK[v.GetObjectKind().GroupVersionKind()] = true
	}
	for k := range seenNS {
		ret.namespaces = append(ret.namespaces, k)
	}
	sort.Strings(ret.namespaces)
	for k := range seenGVK {
		ret.types = append(ret.types, k)
	}
	sort.Slice(ret.types, func(i, j int) bool {
		left := ret.types[i]
		right := ret.types[j]
		if left.Group < right.Group {
			return true
		}
		return left.Kind < right.Kind
	})
	return ret
}

// add adds the supplied object potentially transforming its gvk to its canonical form.
func (c *collection) add(object model.K8sQbecMeta) error {
	gvk := object.GetObjectKind().GroupVersionKind()
	canonicalGVK, err := c.meta.canonicalGroupVersionKind(gvk)
	if err != nil {
		return err
	}
	namespaced, err := c.meta.IsNamespaced(canonicalGVK)
	if err != nil {
		return err
	}
	ns := object.GetNamespace()
	if namespaced {
		if ns == "" {
			ns = c.defaultNs
		}
	} else {
		ns = ""
	}
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
	}
	c.objects[key] = resultObject
	return nil
}

// subtract returns a collection of objects present in the receiver's collection but missing in the
// supplied one.
func (c *collection) subtract(other *collection) *collection {
	ret := newCollection(c.defaultNs, c.meta)
	for k, v := range c.objects {
		if _, ok := other.objects[k]; !ok {
			ret.objects[k] = v
		}
	}
	return ret
}

// toList returns the list of objects in this collection in arbitrary order.
func (c *collection) toList() []model.K8sQbecMeta {
	var ret []model.K8sQbecMeta
	for _, v := range c.objects {
		ret = append(ret, v)
	}
	return ret
}
