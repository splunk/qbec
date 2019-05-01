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

package k8smeta

import (
	"fmt"
	"os"
	"sort"
	"strings"
	"sync"

	"github.com/pkg/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

var defaultVerbs = []string{"create", "delete", "get", "list"}

// gvkInfo is all the information we need for k8s types as represented by group-version-kind.
type gvkInfo struct {
	canonical schema.GroupVersionKind // the preferred gvk that includes aliasing (e.g. extensions/v1beta1 => apps/v1)
	resource  metav1.APIResource      // the API resource for the gvk
}

// ResourceDiscovery is the minimal interface required to gather information on
// server resources.
type ResourceDiscovery interface {
	ServerGroups() (*metav1.APIGroupList, error)
	ServerResourcesForGroupVersion(groupVersion string) (*metav1.APIResourceList, error)
}

// Resources provides resource information for a K8s cluster.
type Resources struct {
	disco    ResourceDiscovery
	registry map[schema.GroupVersionKind]*gvkInfo
}

// ResourceOpts is optional information for loading resources.
type ResourceOpts struct {
	RequiredVerbs []string             // verbs that a resource must support in order to be loaded. Defaults to create/delete/get/list
	WarnFn        func(...interface{}) // a function that can print warnings in the resource discovery.
}

func (o *ResourceOpts) setDefaults() {
	if o.WarnFn == nil {
		o.WarnFn = func(args ...interface{}) {
			fmt.Fprintln(os.Stderr, args...)
		}
	}
	if len(o.RequiredVerbs) == 0 {
		o.RequiredVerbs = defaultVerbs
	}
}

// NewResources loads server resources using the supplied discovery interface.
func NewResources(disco ResourceDiscovery, opts ResourceOpts) (*Resources, error) {
	sm := &Resources{
		disco:    disco,
		registry: map[schema.GroupVersionKind]*gvkInfo{},
	}
	opts.setDefaults()
	if err := sm.init(opts); err != nil {
		return nil, err
	}
	return sm, nil
}

// APIResource returns the API resource for the supplied group version kind or nil
// if no resource could be found.
func (r *Resources) APIResource(gvk schema.GroupVersionKind) *metav1.APIResource {
	r0, ok := r.registry[gvk]
	if !ok {
		return nil
	}
	res := r0.resource
	return &res
}

// CanonicalResources returns a map of API resources keyed by group-kind.
func (r *Resources) CanonicalResources() map[schema.GroupKind]metav1.APIResource {
	canonical := map[schema.GroupVersionKind]bool{}
	for _, v := range r.registry {
		canonical[v.canonical] = true
	}

	ret := map[schema.GroupKind]metav1.APIResource{}
	for k := range canonical {
		r0 := r.registry[k]
		res := r0.resource
		res.Group = k.Group
		res.Version = k.Version
		res.Kind = k.Kind
		ret[k.GroupKind()] = res
	}
	return ret
}

// CanonicalGroupVersionKind provides the preferred/ canonical group version kind for the supplied input.
// It takes aliases into account (e.g. extensions/Deployment same as apps/Deployment) for doing so.
func (r *Resources) CanonicalGroupVersionKind(gvk schema.GroupVersionKind) (schema.GroupVersionKind, error) {
	res, ok := r.registry[gvk]
	if !ok {
		return gvk, fmt.Errorf("server does not recognize gvk %s", gvk)
	}
	return res.canonical, nil
}

// Dump dumps resource mappings using the supplied println function.
func (r *Resources) Dump(println func(...interface{})) {
	var display []string
	for k, v := range r.registry {
		l := fmt.Sprintf("%s/%s:%s", k.Group, k.Version, k.Kind)
		r := fmt.Sprintf("%s/%s:%s", v.canonical.Group, v.canonical.Version, v.canonical.Kind)
		ns := "cluster scoped"
		if v.resource.Namespaced {
			ns = "namespaced"
		}
		display = append(display, fmt.Sprintf("\t%-70s => %s (%s)", l, r, ns))
	}
	sort.Strings(display)
	println()
	println("group version kind map:")
	for _, line := range display {
		println(line)
	}
	println()
}

type equivalence struct {
	gk1 schema.GroupKind
	gk2 schema.GroupKind
}

// equivalences from https://github.com/kubernetes/kubernetes/blob/master/pkg/kubeapiserver/default_storage_factory_builder.go
var equivalences = []equivalence{
	{
		gk1: schema.GroupKind{Group: "networking.k8s.io", Kind: "NetworkPolicy"},
		gk2: schema.GroupKind{Group: "extensions", Kind: "NetworkPolicy"},
	},
	{
		gk1: schema.GroupKind{Group: "networking.k8s.io", Kind: "Ingress"},
		gk2: schema.GroupKind{Group: "extensions", Kind: "Ingress"},
	},
	{
		gk1: schema.GroupKind{Group: "apps", Kind: "Deployment"},
		gk2: schema.GroupKind{Group: "extensions", Kind: "Deployment"},
	},
	{
		gk1: schema.GroupKind{Group: "apps", Kind: "DaemonSet"},
		gk2: schema.GroupKind{Group: "extensions", Kind: "DaemonSet"},
	},
	{
		gk1: schema.GroupKind{Group: "apps", Kind: "ReplicaSet"},
		gk2: schema.GroupKind{Group: "extensions", Kind: "ReplicaSet"},
	},
	{
		gk1: schema.GroupKind{Group: "", Kind: "Event"},
		gk2: schema.GroupKind{Group: "events.k8s.io", Kind: "Event"},
	},
	{
		gk1: schema.GroupKind{Group: "policy", Kind: "PodSecurityPolicy"},
		gk2: schema.GroupKind{Group: "extensions", Kind: "PodSecurityPolicy"},
	},
}

func eligibleResource(r metav1.APIResource, requiredVerbs []string) bool {
	for _, n := range requiredVerbs {
		found := false
		for _, v := range r.Verbs {
			if n == v {
				found = true
				break
			}
		}
		if !found {
			return false
		}
	}
	return true
}

type resolver struct {
	warnFn           func(...interface{})
	requiredVerbs    []string
	group            string
	version          string
	groupVersion     string
	preferredVersion string
	registry         map[schema.GroupVersionKind]*gvkInfo
	tracker          map[schema.GroupKind][]schema.GroupVersionKind
	err              error
}

func (r *resolver) resolve(disco ResourceDiscovery) {
	if r.warnFn == nil {
		r.warnFn = func(args ...interface{}) { fmt.Fprintln(os.Stderr, args...) }
	}
	reg := map[schema.GroupVersionKind]*gvkInfo{}
	tracker := map[schema.GroupKind][]schema.GroupVersionKind{}
	list, err := disco.ServerResourcesForGroupVersion(r.groupVersion)
	if err != nil {
		r.warnFn("error getting resources for type", r.groupVersion, ":", err)
	}
	if list != nil {
		for _, res := range list.APIResources {
			if strings.Contains(res.Name, "/") { // ignore sub-resources
				continue
			}
			if !eligibleResource(res, r.requiredVerbs) { // remove stuff we cannot manipulate.
				continue
			}

			// backfill the gv into res
			res.Group = r.group
			res.Version = r.version
			gvk := schema.GroupVersionKind{Group: res.Group, Version: res.Version, Kind: res.Kind}
			// the canonical version of the type may not be correct at this stage if the preferred group version
			// does not have the specific kind. We will fix these anomalies later when all objects have been loaded
			// and are known.
			reg[gvk] = &gvkInfo{
				canonical: schema.GroupVersionKind{Group: r.group, Version: r.preferredVersion, Kind: res.Kind},
				resource:  res,
			}
			gk := schema.GroupKind{Group: r.group, Kind: res.Kind}
			tracker[gk] = append(tracker[gk], gvk)
		}
	}
	r.registry = reg
	r.tracker = tracker
}

func (r *Resources) init(opts ResourceOpts) error {
	groups, err := r.disco.ServerGroups()
	if err != nil {
		return errors.Wrap(err, "get server groups")
	}

	order := 0
	groupOrderMap := map[string]int{}

	var resolvers []*resolver
	for _, group := range groups.Groups {
		groupName := group.Name
		order++
		groupOrderMap[groupName] = order
		preferredVersionName := group.PreferredVersion.Version
		for _, gv := range group.Versions {
			versionName := gv.Version
			resolvers = append(resolvers, &resolver{
				warnFn:           opts.WarnFn,
				requiredVerbs:    opts.RequiredVerbs,
				group:            groupName,
				version:          versionName,
				preferredVersion: preferredVersionName,
				groupVersion:     gv.GroupVersion,
			})
		}
	}

	var wg sync.WaitGroup
	wg.Add(len(resolvers))
	for _, r0 := range resolvers {
		go func(resolver *resolver) {
			defer wg.Done()
			resolver.resolve(r.disco)
		}(r0)
	}
	wg.Wait()

	reg := map[schema.GroupVersionKind]*gvkInfo{}
	// tracker tracks all known versions for a given group kind for the purposes of updating
	// the canonical versions for equivalences.
	tracker := map[schema.GroupKind][]schema.GroupVersionKind{}
	for _, r := range resolvers {
		if r.err != nil {
			return r.err
		}
		for k, v := range r.registry {
			reg[k] = v
		}
		for k, v := range r.tracker {
			tracker[k] = append(tracker[k], v...)
		}
	}

	// now deal with incorrect preferred versions when specific types do not exist for those
	var fixTypes []schema.GroupVersionKind // collect list of types to be fixed
	for k, v := range reg {
		canon := v.canonical
		if reg[canon] == nil {
			fixTypes = append(fixTypes, k)
		}
	}
	for _, k := range fixTypes {
		v := reg[k]
		reg[k] = &gvkInfo{
			canonical: k,
			resource:  v.resource,
		}
	}

	// then process aliases
	for _, eq := range equivalences {
		gk1 := eq.gk1
		gk2 := eq.gk2
		_, gk1Present := tracker[gk1]
		_, gk2Present := tracker[gk2]
		if !(gk1Present && gk2Present) {
			continue
		}
		g1Order := groupOrderMap[gk1.Group]
		g2Order := groupOrderMap[gk2.Group]
		var canonicalGK, aliasGK schema.GroupKind
		if g1Order < g2Order {
			canonicalGK, aliasGK = eq.gk1, eq.gk2
		} else {
			canonicalGK, aliasGK = eq.gk2, eq.gk1
		}
		anyGKV := tracker[canonicalGK][0]
		canonicalGKV := reg[anyGKV].canonical
		for _, gkv := range tracker[aliasGK] {
			reg[gkv] = &gvkInfo{
				canonical: canonicalGKV,
				resource:  reg[gkv].resource,
			}
		}
	}

	r.registry = reg
	return nil
}
