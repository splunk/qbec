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

package commands

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/pkg/errors"
	"github.com/splunk/qbec/internal/model"
	"github.com/splunk/qbec/internal/sio"
	"github.com/splunk/qbec/internal/types"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/client-go/dynamic"
)

// waitListener listens to rollout status updates and provides feedback to the user.
type waitListener struct {
	start         time.Time                       // start time using which relative progress times are printed
	displayNameFn func(meta model.K8sMeta) string // MUST produce distinct strings for each object, name used as internal key
	l             sync.Mutex                      // locks concurrent access to field below
	remaining     map[string]bool                 // objects not yet marked "done"
}

func (w *waitListener) since() time.Duration {
	return time.Since(w.start).Round(time.Second)
}

// OnInit implements the interface method and prints the name of all objects on which we ware waiting
func (w *waitListener) OnInit(objects []model.K8sMeta) {
	w.start = time.Now()
	w.remaining = map[string]bool{}
	sio.Noticef("waiting for readiness of %d objects\n", len(objects))
	for _, o := range objects {
		name := w.displayNameFn(o)
		w.remaining[name] = true
		sio.Printf("  - %s\n", w.displayNameFn(o))
	}
	sio.Println()
}

// OnStatusChange prints the updated status of the object and removes it from the internal list of remaining items
// if the status is marked done.
func (w *waitListener) OnStatusChange(object model.K8sMeta, rs types.RolloutStatus) {
	w.l.Lock()
	defer w.l.Unlock()
	if rs.Done {
		name := w.displayNameFn(object)
		delete(w.remaining, name)
		sio.Noticef("✓ %-6s: %s :: %s (%d remaining)\n", w.since(), w.displayNameFn(object), rs.Description, len(w.remaining))
		return
	}
	sio.Debugf("  %-6s: %s :: %s\n", w.since(), w.displayNameFn(object), rs.Description)
}

// OnError prints the error for the object to console.
func (w *waitListener) OnError(object model.K8sMeta, err error) {
	w.l.Lock()
	defer w.l.Unlock()
	sio.Errorf("%-6s: %s :: %v\n", w.since(), w.displayNameFn(object), err)
}

// OnEnd prints a list of objects that are not marked complete.
func (w *waitListener) OnEnd(err error) {
	w.l.Lock()
	defer w.l.Unlock()
	sio.Println()
	if len(w.remaining) > 0 {
		sio.Printf("%s: rollout not complete for the following %d objects\n", w.since(), len(w.remaining))
		for name := range w.remaining {
			sio.Printf("  - %s\n", name)
		}
	}
	if err == nil {
		sio.Noticef("✓ %s: rollout complete\n", w.since())
		return
	}
}

type resourceInterfaceProvider func(gvk schema.GroupVersionKind, namespace string) (dynamic.ResourceInterface, error)

func waitWatcher(ctx context.Context, ri resourceInterfaceProvider, obj model.K8sMeta) (watch.Interface, error) {
	in, err := ri(obj.GroupVersionKind(), obj.GetNamespace())
	if err != nil {
		return nil, errors.Wrap(err, "get resource provider")
	}
	_, err = in.Get(ctx, obj.GetName(), metav1.GetOptions{})
	if err != nil { // object must exist
		return nil, errors.Wrap(err, "get object")
	}
	watchXface, err := in.Watch(ctx, metav1.ListOptions{
		FieldSelector: fmt.Sprintf(`metadata.name=%s`, obj.GetName()), // XXX: escaping
	})
	if err != nil { // XXX: implement fallback to poll with get if watch has permissions issues
		return nil, errors.Wrap(err, "get watch interface")
	}
	return watchXface, nil
}
