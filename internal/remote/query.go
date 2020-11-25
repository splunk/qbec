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
	"fmt"
	"reflect"
	"strings"
	"sync"
	"time"

	"github.com/pkg/errors"
	"github.com/splunk/qbec/internal/model"
	"github.com/splunk/qbec/internal/sio"
	apiErrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"
)

type queryConfig struct {
	scope            ListQueryConfig
	resourceProvider func(gvk schema.GroupVersionKind, namespace string) (dynamic.ResourceInterface, error)
	namespacedTypes  []schema.GroupVersionKind
	clusterTypes     []schema.GroupVersionKind
	verbosity        int
}

type objectLister struct {
	queryConfig
}

func (o *objectLister) listObjectsOfType(gvk schema.GroupVersionKind, namespace string) ([]*basicObject, error) {
	startTime := time.Now()
	defer func() {
		if o.verbosity > 0 {
			sio.Debugf("list objects: type=%s,namespace=%q took %v\n", gvk, namespace, time.Since(startTime).Round(time.Millisecond))
		}
	}()
	xface, err := o.resourceProvider(gvk, namespace)
	if err != nil {
		return nil, errors.Wrap(err, fmt.Sprintf("get resource interface for %s", gvk))
	}
	ls := fmt.Sprintf("%s=%s,%s=%s", model.QbecNames.ApplicationLabel, o.scope.Application, model.QbecNames.EnvironmentLabel, o.scope.Environment)
	if o.scope.Tag == "" {
		ls = fmt.Sprintf("%s,!%s", ls, model.QbecNames.TagLabel)
	} else {
		ls = fmt.Sprintf("%s,%s=%s", ls, model.QbecNames.TagLabel, o.scope.Tag)
	}
	list, err := xface.List(metav1.ListOptions{
		LabelSelector: ls,
	})
	if err != nil {
		if apiErrors.IsForbidden(err) {
			sio.Warnf("not authorized to list %s, error ignored\n", gvk)
			return nil, nil
		}
		return nil, err
	}
	objs, err := meta.ExtractList(list)
	if err != nil {
		return nil, errors.Wrap(err, fmt.Sprintf("extract items for %s", gvk))
	}
	var ret []*basicObject

outer:
	for _, obj := range objs {
		un, ok := obj.(*unstructured.Unstructured)
		if !ok {
			return nil, fmt.Errorf("dunno how to process object of type %v", reflect.TypeOf(obj))
		}

		// check if the object has been created by a controller and, if so, skip it
		refs := un.GetOwnerReferences()
		for _, ref := range refs {
			if ref.Controller != nil && *ref.Controller {
				if o.verbosity > 0 {
					sio.Debugf("ignore %s %s since it was created by a controller\n", gvk, un.GetName())
				}
				continue outer
			}
		}

		labels := un.GetLabels()
		if labels == nil {
			labels = map[string]string{}
		}
		anns := un.GetAnnotations()
		if anns == nil {
			anns = map[string]string{}
		}
		mm := &basicObject{
			objectKey: objectKey{
				gvk:       un.GroupVersionKind(),
				namespace: un.GetNamespace(),
				name:      un.GetName(),
			},
			app:       labels[model.QbecNames.ApplicationLabel],
			component: anns[model.QbecNames.ComponentAnnotation],
			env:       labels[model.QbecNames.EnvironmentLabel],
			anns:      un.GetAnnotations(),
		}
		ret = append(ret, mm)
	}
	return ret, nil
}

type errorContext struct {
	gvk       schema.GroupVersionKind
	namespace string
	err       error
}

type errorCollection struct {
	errors []errorContext
}

func (e *errorCollection) add(ec errorContext) {
	e.errors = append(e.errors, ec)
}

func (e *errorCollection) Error() string {
	var lines []string
	for _, e := range e.errors {
		lines = append(lines, fmt.Sprintf("list %s %s: %v", e.gvk, e.namespace, e.err))
	}
	return "list errors:" + strings.Join(lines, "\n\t")
}

func (o *objectLister) serverObjects(coll *collection) error {
	var l sync.Mutex
	var errs errorCollection
	add := func(gvk schema.GroupVersionKind, ns string, objects []*basicObject, err error) {
		l.Lock()
		defer l.Unlock()
		if err != nil {
			errs.add(errorContext{gvk: gvk, namespace: ns, err: err})
		}
		for _, o := range objects {
			coll.objects[o.objectKey] = o
		}
	}

	var workers []func()
	addQueries := func(types []schema.GroupVersionKind, ns string) {
		for _, gvk := range types {
			if o.scope.KindFilter != nil && !o.scope.KindFilter(gvk) {
				continue
			}
			localType := gvk
			workers = append(workers, func() {
				ret, err := o.listObjectsOfType(localType, ns)
				add(localType, ns, ret, err)
			})
		}
	}

	if len(o.scope.Namespaces) > 0 {
		switch {
		case len(o.scope.Namespaces) == 1 || !o.scope.ClusterScopedLists:
			for _, ns := range o.scope.Namespaces {
				addQueries(o.namespacedTypes, ns)
			}
		default:
			sio.Debugln("using cluster scoped queries for multiple namespaces")
			addQueries(o.namespacedTypes, "")
		}
	}

	if o.scope.ClusterObjects {
		addQueries(o.clusterTypes, "")
	}

	concurrency := o.scope.Concurrency
	if concurrency <= 0 {
		concurrency = 5
	}

	ch := make(chan func(), len(workers))
	for _, w := range workers {
		ch <- w
	}
	close(ch)

	var wg sync.WaitGroup
	wg.Add(concurrency)
	for i := 0; i < concurrency; i++ {
		go func() {
			defer wg.Done()
			for fn := range ch {
				fn()
			}
		}()
	}

	wg.Wait()

	if len(errs.errors) > 0 {
		return &errs
	}
	return nil
}
