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

// Package objsort allows sorting of K8s objects in the order in which they should be applied to the cluster.
package objsort

import (
	"sort"

	"github.com/splunk/qbec/internal/model"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

// OrderingProvider provides a positive order for the supplied item if it wants
// to influence its apply order or 0 if it does not care.
type OrderingProvider func(item model.K8sQbecMeta) int

// Namespaced returns true if the supplied gvk is a namespaced resource.
type Namespaced func(gvk schema.GroupVersionKind) (namespaced bool, err error)

// Config is the sort configuration. The ordering provider may be nil if no custom
// ordering is required.
type Config struct {
	OrderingProvider    OrderingProvider // custom ordering provider
	NamespacedIndicator Namespaced       // indicator to determine if resource sis namespaced
}

// ordering for specific classes of objects
const (
	GenericClusterObjectOrder = 30  // any cluster-level object that does not have an assigned order
	GenericNamespacedOrder    = 80  // any namespaced object that does not have an assigned order
	GenericPodOrder           = 100 // any object that results in pod creation
	GenericLast               = 120 // any object for which server metadata was not found
)

// SpecifiedOrdering defines ordering for a set of known kubernetes objects.
var SpecifiedOrdering = map[schema.GroupKind]int{
	schema.GroupKind{Group: "extensions", Kind: "PodSecurityPolicy"}:                                10,
	schema.GroupKind{Group: "extensions", Kind: "ThirdPartyResource"}:                               20,
	schema.GroupKind{Group: "apiextensions.k8s.io", Kind: "CustomResourceDefinition"}:               20,
	schema.GroupKind{Group: "", Kind: "Namespace"}:                                                  GenericNamespacedOrder - 30,
	schema.GroupKind{Group: "", Kind: "LimitRange"}:                                                 GenericNamespacedOrder - 20,
	schema.GroupKind{Group: "", Kind: "ServiceAccount"}:                                             GenericNamespacedOrder - 20,
	schema.GroupKind{Group: "", Kind: "ConfigMap"}:                                                  GenericNamespacedOrder - 10,
	schema.GroupKind{Group: "", Kind: "Secret"}:                                                     GenericNamespacedOrder - 10,
	schema.GroupKind{Group: "extensions", Kind: "DaemonSet"}:                                        GenericPodOrder,
	schema.GroupKind{Group: "extensions", Kind: "Deployment"}:                                       GenericPodOrder,
	schema.GroupKind{Group: "extensions", Kind: "ReplicaSet"}:                                       GenericPodOrder,
	schema.GroupKind{Group: "extensions", Kind: "StatefulSet"}:                                      GenericPodOrder,
	schema.GroupKind{Group: "apps", Kind: "DaemonSet"}:                                              GenericPodOrder,
	schema.GroupKind{Group: "apps", Kind: "Deployment"}:                                             GenericPodOrder,
	schema.GroupKind{Group: "apps", Kind: "ReplicaSet"}:                                             GenericPodOrder,
	schema.GroupKind{Group: "apps", Kind: "StatefulSet"}:                                            GenericPodOrder,
	schema.GroupKind{Group: "batch", Kind: "Job"}:                                                   GenericPodOrder,
	schema.GroupKind{Group: "batch", Kind: "CronJob"}:                                               GenericPodOrder,
	schema.GroupKind{Group: "", Kind: "Service"}:                                                    GenericPodOrder + 10,
	schema.GroupKind{Group: "admissionregistration.k8s.io", Kind: "ValidatingWebhookConfiguration"}: GenericPodOrder + 20,
	schema.GroupKind{Group: "admissionregistration.k8s.io", Kind: "MutatingWebhookConfiguration"}:   GenericPodOrder + 20,
}

func getOrder(ob model.K8sQbecMeta, config Config) int {
	order := config.OrderingProvider(ob)
	if order > 0 {
		return order
	}
	gvk := ob.GetObjectKind().GroupVersionKind()
	gk := gvk.GroupKind()
	if order, ok := SpecifiedOrdering[gk]; ok {
		return order
	}
	namespaced, err := config.NamespacedIndicator(gvk)
	if err != nil {
		return GenericLast
	}
	if namespaced {
		return GenericNamespacedOrder
	}
	return GenericClusterObjectOrder
}

type sortInput struct {
	item      interface{}
	kind      string
	component string
	ns        string
	name      string
	order     int
}

type sorter struct {
	inputs []sortInput
	config Config
}

func newSorter(config Config) *sorter {
	if config.OrderingProvider == nil {
		config.OrderingProvider = func(ob model.K8sQbecMeta) int { return 0 }
	}
	return &sorter{
		config: config,
	}
}

func (s *sorter) add(o model.K8sQbecMeta, item interface{}) {
	s.inputs = append(s.inputs, sortInput{
		item:      item,
		kind:      o.GetKind(),
		component: o.Component(),
		ns:        o.GetNamespace(),
		name:      o.GetName(),
		order:     getOrder(o, s.config),
	})
}

func (s *sorter) sort() {
	items := s.inputs
	sort.Slice(items, func(i, j int) bool {
		left := items[i]
		right := items[j]
		if left.order != right.order {
			return left.order < right.order
		}
		if left.kind != right.kind {
			return left.kind < right.kind
		}
		if left.component != right.component {
			return left.component < right.component
		}
		if left.ns != right.ns {
			return left.ns < right.ns
		}
		return left.name < right.name
	})
}

// SortMeta sorts the supplied meta objects based on the config.
func SortMeta(inputs []model.K8sQbecMeta, config Config) []model.K8sQbecMeta {
	sorter := newSorter(config)
	for _, obj := range inputs {
		sorter.add(obj, obj)
	}
	sorter.sort()
	ret := make([]model.K8sQbecMeta, 0, len(inputs))
	for _, o := range sorter.inputs {
		ret = append(ret, o.item.(model.K8sQbecMeta))
	}
	return ret
}

// Sort sorts the supplied local objects based on the supplied configuration.
func Sort(inputs []model.K8sLocalObject, config Config) []model.K8sLocalObject {
	sorter := newSorter(config)
	for _, obj := range inputs {
		sorter.add(obj, obj)
	}
	sorter.sort()
	ret := make([]model.K8sLocalObject, 0, len(inputs))
	for _, o := range sorter.inputs {
		ret = append(ret, o.item.(model.K8sLocalObject))
	}
	return ret
}
