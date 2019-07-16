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
	"fmt"
	"io"
	"sync"
	"time"

	"github.com/pkg/errors"
	"github.com/splunk/qbec/internal/model"
	"github.com/splunk/qbec/internal/rollout"
	"github.com/splunk/qbec/internal/sio"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/client-go/dynamic"
)

type waitListener struct {
	l             sync.Mutex
	start         time.Time
	out           io.Writer
	remaining     map[string]bool
	displayNameFn func(meta model.K8sMeta) string
}

func (w *waitListener) since() time.Duration {
	return time.Since(w.start).Round(time.Second)
}

func (w *waitListener) OnInit(objects []model.K8sMeta) {
	w.start = time.Now()
	w.remaining = map[string]bool{}
	sio.Noticef("waiting for readiness of %d objects\n", len(objects))
	for _, o := range objects {
		name := w.displayNameFn(o)
		w.remaining[name] = true
		sio.Printf("\t- %s\n", w.displayNameFn(o))
	}
	sio.Println()
}

func (w *waitListener) OnStatusChange(object model.K8sMeta, rs rollout.ObjectStatus) {
	w.l.Lock()
	defer w.l.Unlock()
	if rs.Done {
		name := w.displayNameFn(object)
		delete(w.remaining, name)
		sio.Noticef("✓ %6s: %s :: %s (%d remaining)\n", w.since(), w.displayNameFn(object), rs.Description, len(w.remaining))
		return
	}
	sio.Debugf("  %6s: %s :: %s\n", w.since(), w.displayNameFn(object), rs.Description)
}

func (w *waitListener) OnError(object model.K8sMeta, err error) {
	w.l.Lock()
	defer w.l.Unlock()
	sio.Errorf("%6s: %s :: %v\n", w.since(), w.displayNameFn(object), err)
}

func (w *waitListener) OnEnd(err error) {
	w.l.Lock()
	defer w.l.Unlock()
	sio.Println()
	if err == nil {
		sio.Noticef("%s: rollout complete\n", w.since())
		return
	}
	if len(w.remaining) == 0 {
		return
	}
	sio.Printf("%s: rollout not complete for the following %d objects\n", w.since(), len(w.remaining))
	for name := range w.remaining {
		sio.Printf("\t- %s\n", name)
	}
}

type resourceInterfaceProvider func(gvk schema.GroupVersionKind, namespace string) (dynamic.ResourceInterface, error)

func waitWatcher(ri resourceInterfaceProvider, obj model.K8sMeta) (watch.Interface, error) {
	in, err := ri(obj.GroupVersionKind(), obj.GetNamespace())
	if err != nil {
		return nil, errors.Wrap(err, "get resource provider")
	}
	_, err = in.Get(obj.GetName(), metav1.GetOptions{})
	if err != nil { // object must exist
		return nil, errors.Wrap(err, "get object")
	}
	watchXface, err := in.Watch(metav1.ListOptions{
		FieldSelector: fmt.Sprintf(`metadata.name=%s`, obj.GetName()), // XXX: escaping
	})
	if err != nil { // XXX: implement fallback to poll with get if watch has permissions issues
		return nil, errors.Wrap(err, "get watch interface")
	}
	return watchXface, nil
}
