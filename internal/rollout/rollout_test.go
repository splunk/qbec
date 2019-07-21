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

package rollout

import (
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/splunk/qbec/internal/model"
	"github.com/splunk/qbec/internal/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/watch"
)

type testEvent struct {
	wait  time.Duration
	event watch.Event
}

type testWatcher struct {
	ch   chan watch.Event
	once sync.Once
}

func (t *testWatcher) Stop() {
	t.once.Do(func() {
		close(t.ch)
	})
}

func (t *testWatcher) emit(events []testEvent) {
	for _, e := range events {
		time.Sleep(e.wait)
		t.ch <- e.event
	}
}

func (t *testWatcher) ResultChan() <-chan watch.Event {
	return t.ch
}

func newTestWatcher(events []testEvent) *testWatcher {
	tw := &testWatcher{ch: make(chan watch.Event, 10)}
	go tw.emit(events)
	return tw
}

func testKey(obj model.K8sMeta) string {
	return fmt.Sprintf("%s/%s %s/%s", obj.GroupVersionKind().Group, obj.GroupVersionKind().Kind, obj.GetNamespace(), obj.GetName())
}

type watchFactory struct {
	eventsMap map[string][]testEvent
}

func (w *watchFactory) getWatcher(obj model.K8sMeta) (watch.Interface, error) {
	k := testKey(obj)
	events, ok := w.eventsMap[k]
	if !ok {
		return nil, fmt.Errorf("unable to produce events for %s", k)
	}
	return newTestWatcher(events), nil
}

type testListener struct {
	t                *testing.T
	l                sync.Mutex
	initCalled       bool
	endCalled        bool
	initObjects      int
	remainingObjects int
	statuses         map[string][]string
	errors           map[string]string
}

func newTestListener(t *testing.T) *testListener {
	return &testListener{
		t:        t,
		statuses: map[string][]string{},
		errors:   map[string]string{},
	}
}

func (l *testListener) OnInit(objects []model.K8sMeta) {
	l.l.Lock()
	defer l.l.Unlock()
	l.initObjects = len(objects)
	l.remainingObjects = l.initObjects
	l.initCalled = true
	l.t.Log("init", len(objects), "objects")
}

func (l *testListener) OnStatusChange(object model.K8sMeta, rs types.RolloutStatus) {
	l.l.Lock()
	defer l.l.Unlock()
	k := testKey(object)
	l.statuses[k] = append(l.statuses[k], rs.Description)
	if rs.Done {
		l.remainingObjects--
	}
	l.t.Log("k=", k, "desc=", rs.Description, "done=", rs.Done)
}

func (l *testListener) OnError(object model.K8sMeta, err error) {
	l.l.Lock()
	defer l.l.Unlock()
	k := testKey(object)
	l.errors[k] = err.Error()
	l.t.Log("k=", k, "err=", err)
}

func (l *testListener) OnEnd(err error) {
	l.l.Lock()
	defer l.l.Unlock()
	l.endCalled = true
	l.t.Log("end", "err=", err, "remaining=", l.remainingObjects)
}

func newObject(kind string, name string, status *types.RolloutStatus, err error) map[string]interface{} {
	anns := map[string]interface{}{}
	ret := map[string]interface{}{
		"apiVersion": "apps/v1",
		"kind":       kind,
		"metadata": map[string]interface{}{
			"namespace":   "test-ns",
			"name":        name,
			"annotations": anns,
		},
	}
	if status != nil {
		anns["status/desc"] = status.Description
		anns["status/done"] = fmt.Sprint(status.Done)
	}
	if err != nil {
		anns["status/error"] = err.Error()
	}
	return ret
}

func extractStatus(obj *unstructured.Unstructured, _ int64) (*types.RolloutStatus, error) {
	desc := obj.GetAnnotations()["status/desc"]
	errmsg := obj.GetAnnotations()["status/error"]
	done := obj.GetAnnotations()["status/done"]
	if errmsg != "" {
		return nil, fmt.Errorf(errmsg)
	}
	return &types.RolloutStatus{Description: desc, Done: done == "true"}, nil
}

func testStatusMapper(obj model.K8sMeta) types.RolloutStatusFunc {
	switch obj.GetKind() {
	case "Foo", "Bar":
		return extractStatus
	default:
		return nil
	}
}

func newTestMeta(kind string, name string) model.K8sMeta {
	return model.NewK8sObject(newObject(kind, name, nil, nil))
}

func newUnstructured(kind string, name string, status *types.RolloutStatus, err error) *unstructured.Unstructured {
	return &unstructured.Unstructured{Object: newObject(kind, name, status, err)}
}

func TestWaitUntilComplete(t *testing.T) {
	statusMapper = testStatusMapper
	defer func() {
		statusMapper = types.StatusFuncFor
	}()

	foo1, foo2 := newTestMeta("Foo", "foo1"), newTestMeta("Foo", "foo2")
	bar1 := newTestMeta("Bar", "bar1")
	baz1 := newTestMeta("Baz", "baz1")

	wf := &watchFactory{
		eventsMap: map[string][]testEvent{
			testKey(foo1): {
				{
					wait: 0,
					event: watch.Event{
						Type:   watch.Modified,
						Object: newUnstructured(foo1.GetKind(), foo1.GetName(), &types.RolloutStatus{Description: "start"}, nil),
					},
				},
				{
					wait: 10 * time.Millisecond,
					event: watch.Event{
						Type:   watch.Modified,
						Object: newUnstructured(foo1.GetKind(), foo1.GetName(), &types.RolloutStatus{Description: "mid"}, nil),
					},
				},
				{
					wait: 20 * time.Millisecond,
					event: watch.Event{
						Type:   watch.Modified,
						Object: newUnstructured(foo1.GetKind(), foo1.GetName(), &types.RolloutStatus{Description: "end", Done: true}, nil),
					},
				},
			},
			testKey(foo2): {
				{
					wait: 5 * time.Millisecond,
					event: watch.Event{
						Type:   watch.Modified,
						Object: newUnstructured(foo2.GetKind(), foo2.GetName(), &types.RolloutStatus{Description: "done", Done: true}, nil),
					},
				},
			},
			testKey(bar1): {
				{
					wait: 30 * time.Millisecond,
					event: watch.Event{
						Type:   watch.Modified,
						Object: newUnstructured(bar1.GetKind(), bar1.GetName(), &types.RolloutStatus{Description: "done", Done: true}, nil),
					},
				},
			},
		},
	}
	listener := newTestListener(t)
	err := WaitUntilComplete(
		[]model.K8sMeta{foo1, foo2, bar1, baz1},
		wf.getWatcher,
		WaitOptions{Listener: listener, Timeout: time.Second},
	)
	require.Nil(t, err)
	a := assert.New(t)
	a.Equal(3, listener.initObjects)
	a.Equal(0, listener.remainingObjects)
	a.Equal([]string{"start", "mid", "end"}, listener.statuses[testKey(foo1)])
	a.Equal([]string{"done"}, listener.statuses[testKey(foo2)])
	a.Equal([]string{"done"}, listener.statuses[testKey(bar1)])
}

func TestWaitUntilCompleteDefaultOpts(t *testing.T) {
	statusMapper = testStatusMapper
	defer func() {
		statusMapper = types.StatusFuncFor
	}()

	bar1 := newTestMeta("Bar", "bar1")

	wf := &watchFactory{
		eventsMap: map[string][]testEvent{
			testKey(bar1): {
				{
					wait: 30 * time.Millisecond,
					event: watch.Event{
						Type:   watch.Modified,
						Object: newUnstructured(bar1.GetKind(), bar1.GetName(), &types.RolloutStatus{Description: "done", Done: true}, nil),
					},
				},
			},
		},
	}
	err := WaitUntilComplete(
		[]model.K8sMeta{bar1},
		wf.getWatcher,
		WaitOptions{},
	)
	require.Nil(t, err)
}

type runtimeFoo struct{}

func (r runtimeFoo) GetObjectKind() schema.ObjectKind {
	bar1 := newTestMeta("Bar", "bar1")
	return newUnstructured(bar1.GetKind(), bar1.GetName(), nil, nil)
}

func (r runtimeFoo) DeepCopyObject() runtime.Object {
	return r
}

func TestWaitNegative(t *testing.T) {
	statusMapper = testStatusMapper
	defer func() {
		statusMapper = types.StatusFuncFor
	}()
	foo1, foo2 := newTestMeta("Foo", "foo1"), newTestMeta("Foo", "foo2")
	bar1 := newTestMeta("Bar", "bar1")
	baz1 := newTestMeta("Baz", "baz1")

	tests := []struct {
		name     string
		objs     []model.K8sMeta
		init     func() *watchFactory
		asserter func(t *testing.T, err error, listener *testListener)
	}{
		{
			name: "timeout",
			objs: []model.K8sMeta{foo1, bar1},
			init: func() *watchFactory {
				return &watchFactory{
					eventsMap: map[string][]testEvent{
						testKey(foo1): {
							{
								wait: 0,
								event: watch.Event{
									Type:   watch.Modified,
									Object: newUnstructured(foo1.GetKind(), foo1.GetName(), &types.RolloutStatus{Description: "start"}, nil),
								},
							},
							{
								wait: 10 * time.Millisecond,
								event: watch.Event{
									Type:   watch.Modified,
									Object: newUnstructured(foo1.GetKind(), foo1.GetName(), &types.RolloutStatus{Description: "mid"}, nil),
								},
							},
						},
						testKey(bar1): {
							{
								wait: 30 * time.Millisecond,
								event: watch.Event{
									Type:   watch.Modified,
									Object: newUnstructured(bar1.GetKind(), bar1.GetName(), &types.RolloutStatus{Description: "done", Done: true}, nil),
								},
							},
						},
					},
				}
			},
			asserter: func(t *testing.T, err error, listener *testListener) {
				require.NotNil(t, err)
				a := assert.New(t)
				a.Equal("wait timed out after 1s", err.Error())
				a.Equal(2, listener.initObjects)
				a.Equal(1, listener.remainingObjects)
				a.Equal([]string{"start", "mid"}, listener.statuses[testKey(foo1)])
				a.Equal([]string{"done"}, listener.statuses[testKey(bar1)])
			},
		},
		{
			name: "watch error event",
			objs: []model.K8sMeta{foo1},
			init: func() *watchFactory {
				return &watchFactory{
					eventsMap: map[string][]testEvent{
						testKey(foo1): {
							{
								wait: 0,
								event: watch.Event{
									Type:   watch.Error,
									Object: newUnstructured(foo1.GetKind(), foo1.GetName(), nil, nil),
								},
							},
						},
					},
				}
			},
			asserter: func(t *testing.T, err error, listener *testListener) {
				require.NotNil(t, err)
				a := assert.New(t)
				a.Equal("1 wait errors", err.Error())
				a.Equal(1, listener.initObjects)
				a.Equal(1, listener.remainingObjects)
				var dummy []string
				a.EqualValues(dummy, listener.statuses[testKey(foo1)])
				e := listener.errors[testKey(foo1)]
				require.NotNil(t, err)
				a.Contains(e, "watch error")
			},
		},
		{
			name: "obj delete",
			objs: []model.K8sMeta{foo1},
			init: func() *watchFactory {
				return &watchFactory{
					eventsMap: map[string][]testEvent{
						testKey(foo1): {
							{
								wait: 0,
								event: watch.Event{
									Type: watch.Deleted,
								},
							},
						},
					},
				}
			},
			asserter: func(t *testing.T, err error, listener *testListener) {
				require.NotNil(t, err)
				a := assert.New(t)
				a.Equal("1 wait errors", err.Error())
				a.Equal(1, listener.initObjects)
				a.Equal(1, listener.remainingObjects)
				var dummy []string
				a.EqualValues(dummy, listener.statuses[testKey(foo1)])
				e := listener.errors[testKey(foo1)]
				require.NotNil(t, err)
				a.Contains(e, "object was deleted")
			},
		},
		{
			name: "get watch error",
			objs: []model.K8sMeta{foo1},
			init: func() *watchFactory {
				return &watchFactory{
					eventsMap: map[string][]testEvent{},
				}
			},
			asserter: func(t *testing.T, err error, listener *testListener) {
				require.NotNil(t, err)
				a := assert.New(t)
				a.Equal("1 wait errors", err.Error())
				a.Equal(1, listener.initObjects)
				a.Equal(1, listener.remainingObjects)
				var dummy []string
				a.EqualValues(dummy, listener.statuses[testKey(foo1)])
				e := listener.errors[testKey(foo1)]
				require.NotNil(t, err)
				a.Contains(e, "unable to produce events")
			},
		},
		{
			name: "status func error",
			objs: []model.K8sMeta{foo1},
			init: func() *watchFactory {
				return &watchFactory{
					eventsMap: map[string][]testEvent{
						testKey(foo1): {
							{
								wait: 0,
								event: watch.Event{
									Type:   watch.Modified,
									Object: newUnstructured(foo1.GetKind(), foo1.GetName(), nil, fmt.Errorf("fubar")),
								},
							},
						},
					},
				}
			},
			asserter: func(t *testing.T, err error, listener *testListener) {
				require.NotNil(t, err)
				a := assert.New(t)
				a.Equal("1 wait errors", err.Error())
				a.Equal(1, listener.initObjects)
				a.Equal(1, listener.remainingObjects)
				var dummy []string
				a.EqualValues(dummy, listener.statuses[testKey(foo1)])
				e := listener.errors[testKey(foo1)]
				require.NotNil(t, err)
				a.Contains(e, "fubar")
			},
		},
		{
			name: "no objects",
			objs: []model.K8sMeta{baz1},
			init: func() *watchFactory {
				return &watchFactory{
					eventsMap: map[string][]testEvent{},
				}
			},
			asserter: func(t *testing.T, err error, listener *testListener) {
				require.Nil(t, err)
				a := assert.New(t)
				a.True(listener.initCalled)
				a.True(listener.endCalled)
			},
		},
		{
			name: "bad object",
			objs: []model.K8sMeta{foo1},
			init: func() *watchFactory {
				return &watchFactory{
					eventsMap: map[string][]testEvent{
						testKey(foo1): {
							{
								wait: 0,
								event: watch.Event{
									Type:   watch.Modified,
									Object: runtimeFoo{},
								},
							},
						},
					},
				}
			},
			asserter: func(t *testing.T, err error, listener *testListener) {
				require.NotNil(t, err)
				a := assert.New(t)
				a.Equal("1 wait errors", err.Error())
				a.True(listener.initCalled)
				a.True(listener.endCalled)
				e := listener.errors[testKey(foo1)]
				require.NotNil(t, err)
				a.Contains(e, "unexpected watch object type")
			},
		},
	}
	t.Log(foo2, baz1)
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			wf := test.init()
			listener := newTestListener(t)
			err := WaitUntilComplete(
				test.objs,
				wf.getWatcher,
				WaitOptions{Listener: listener, Timeout: time.Second},
			)
			test.asserter(t, err, listener)
		})
	}
}
