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

package commands

import (
	"sort"
	"time"

	"github.com/splunk/qbec/internal/model"
	"github.com/splunk/qbec/internal/remote"
	"github.com/splunk/qbec/internal/sio"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

type listClient interface {
	IsNamespaced(gvk schema.GroupVersionKind) (bool, error)
	ListObjects(scope remote.ListQueryConfig) (remote.Collection, error)
}

// lister lists remote objects and returns a list of objects to be deleted.
type lister interface {
	// start starts listing objects based on the supplied query config in the background.
	start(config remote.ListQueryConfig)
	// deletions returns a list of objects to be deleted given a list of objects to be retained and
	// a filter function that should return true for other objects if they can be deleted.
	deletions(ignore []model.K8sLocalObject, filter func(obj model.K8sQbecMeta) bool) ([]model.K8sQbecMeta, error)
}

type stubLister struct{}

func (s *stubLister) start(config remote.ListQueryConfig) {}
func (s *stubLister) deletions(ignore []model.K8sLocalObject, filter func(obj model.K8sQbecMeta) bool) ([]model.K8sQbecMeta, error) {
	return nil, nil
}

type remoteLister struct {
	client       listClient
	ch           chan listResult
	cfg          remote.ListQueryConfig
	unknownTypes map[schema.GroupVersionKind]bool
}

type listResult struct {
	data     remote.Collection
	duration time.Duration
	err      error
}

func newRemoteLister(client listClient, allObjects []model.K8sLocalObject, defaultNs string) (*remoteLister, remote.ListQueryScope, error) {
	nsMap := map[string]bool{}
	if defaultNs != "" {
		nsMap[defaultNs] = true
	}
	unknown := map[schema.GroupVersionKind]bool{}

	clusterObjects := false
	for _, o := range allObjects {
		kind := o.GroupVersionKind()
		b, err := client.IsNamespaced(kind)
		if err != nil {
			if !unknown[kind] {
				sio.Warnf("unable to get metadata for %v, continue\n", o.GroupVersionKind())
				unknown[kind] = true
			}
			continue
		}
		if b {
			ns := o.GetNamespace()
			if ns == "" {
				ns = defaultNs
			}
			nsMap[ns] = true
		} else {
			clusterObjects = true
		}
	}
	var nsList []string
	for k := range nsMap {
		nsList = append(nsList, k)
	}
	sort.Strings(nsList)

	return &remoteLister{
			client:       client,
			ch:           make(chan listResult, 1),
			unknownTypes: unknown,
		},
		remote.ListQueryScope{
			Namespaces:     nsList,
			ClusterObjects: clusterObjects,
		},
		nil
}

func (r *remoteLister) start(config remote.ListQueryConfig) {
	r.cfg = config
	if config.KindFilter == nil {
		config.KindFilter = func(_ schema.GroupVersionKind) bool { return true }
	}
	orig := config.KindFilter
	config.KindFilter = func(gvk schema.GroupVersionKind) bool {
		if r.unknownTypes[gvk] {
			return false
		}
		return orig(gvk)
	}
	go func() {
		start := time.Now()
		list, err := r.client.ListObjects(config)
		r.ch <- listResult{data: list, err: err, duration: time.Since(start).Round(time.Millisecond)}
	}()
}

func (r *remoteLister) deletions(all []model.K8sLocalObject, filter func(obj model.K8sQbecMeta) bool) ([]model.K8sQbecMeta, error) {
	if len(r.ch) == 0 {
		sio.Debugln("waiting for deletion list to be returned")
	}
	lr := <-r.ch
	if lr.err != nil {
		return nil, lr.err
	}
	sio.Debugf("server objects load took %v\n", lr.duration)

	cfg := r.cfg

	var removals []model.K8sQbecMeta
	for _, c := range all {
		if !cfg.KindFilter(c.GroupVersionKind()) {
			continue
		}
		if r.unknownTypes[c.GroupVersionKind()] {
			continue
		}
		removals = append(removals, c)
	}

	coll := lr.data
	if err := coll.Remove(removals); err != nil {
		return nil, err
	}

	retained := coll.ToList()
	var ret []model.K8sQbecMeta
	for _, o := range retained {
		if filter(o) {
			ret = append(ret, o)
		}
	}

	return ret, nil
}
