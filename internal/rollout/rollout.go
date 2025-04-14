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

// Package rollout implements waiting for rollout completion of a set of objects.
package rollout

import (
	"fmt"
	"reflect"
	"sync"
	"time"

	"github.com/pkg/errors"
	"github.com/splunk/qbec/internal/model"
	"github.com/splunk/qbec/internal/types"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/watch"
)

type statusTracker struct {
	obj      model.K8sMeta
	fn       types.RolloutStatusFunc
	wp       WatchProvider
	listener StatusListener
}

func (s *statusTracker) wait() (finalErr error) {
	defer func() {
		if finalErr != nil {
			s.listener.OnError(s.obj, finalErr)
		}
	}()
	watcher, err := s.wp(s.obj)
	if err != nil {
		return errors.Wrap(err, "get watch interface")
	}
	var prevStatus types.RolloutStatus
	_, err = until(0, watcher, func(e watch.Event) (bool, error) {
		switch e.Type {
		case watch.Deleted:
			return false, fmt.Errorf("object was deleted")
		case watch.Error:
			return false, fmt.Errorf("watch error: %v", e.Object)
		default:
			un, ok := e.Object.(*unstructured.Unstructured)
			if !ok {
				return false, fmt.Errorf("unexpected watch object type: want *unstructured.Unstructured, got %v", reflect.TypeOf(e.Object))
			}
			status, err := s.fn(un, 0)
			if err != nil {
				return false, err
			}
			if prevStatus != *status {
				prevStatus = *status
				s.listener.OnStatusChange(s.obj, prevStatus)
			}
			return status.Done, nil
		}
	})

	return err
}

// StatusListener receives status update callbacks.
type StatusListener interface {
	OnInit(objects []model.K8sMeta)                              // the set of objects that are being monitored
	OnStatusChange(object model.K8sMeta, rs types.RolloutStatus) // status for specified object
	OnError(object model.K8sMeta, err error)                     // watch error of some kind for specified object
	OnEnd(err error)                                             // end of status updates with final error
}

// nopListener is the sentinel used when caller doesn't provide a listener.
type nopListener struct{}

func (n nopListener) OnInit(objects []model.K8sMeta)                              {}
func (n nopListener) OnStatusChange(object model.K8sMeta, rs types.RolloutStatus) {}
func (n nopListener) OnError(object model.K8sMeta, err error)                     {}
func (n nopListener) OnEnd(err error)                                             {}

// WatchProvider provides a resource interface for a specific object type and namespace.
type WatchProvider func(obj model.K8sMeta) (watch.Interface, error)

// WaitOptions are options to the wait function.
type WaitOptions struct {
	Listener StatusListener
	Timeout  time.Duration
}

func (w *WaitOptions) setupDefaults() {
	if w.Listener == nil {
		w.Listener = nopListener{}
	}
	if w.Timeout == 0 {
		w.Timeout = 5 * time.Minute
	}
}

// errCounter tracks a count of seen errors.
type errCounter struct {
	l     sync.Mutex
	count int
}

func (ec *errCounter) add(err error) {
	if err == nil {
		return
	}
	ec.l.Lock()
	ec.count++
	ec.l.Unlock()
}

func (ec *errCounter) toSummaryError() error {
	ec.l.Lock()
	defer ec.l.Unlock()
	if ec.count == 0 {
		return nil
	}
	return fmt.Errorf("%d wait errors", ec.count)
}

// allow standard status function map to be overridden for tests.
var statusMapper = types.StatusFuncFor

// WaitUntilComplete waits for the supplied objects to be ready and returns when they are. An error is returned
// if the function times out before all objects are ready. Any status listener provider is notified of
// individual status changes and errors during the wait. Individual watches having errors are turned into a
// aggregate error.
func WaitUntilComplete(objects []model.K8sMeta, wp WatchProvider, opts WaitOptions) (finalErr error) {
	opts.setupDefaults()

	var watchObjects []model.K8sMeta // the subset of objects we will actually watch
	var trackers []*statusTracker    // the list of trackers that we will run

	// extract objects to wait for
	for _, obj := range objects {
		fn := statusMapper(obj)
		if fn != nil {
			watchObjects = append(watchObjects, obj)
			trackers = append(trackers, &statusTracker{obj: obj, fn: fn, wp: wp, listener: opts.Listener})
		}
	}
	// notify listeners
	opts.Listener.OnInit(watchObjects)
	defer func() {
		opts.Listener.OnEnd(finalErr)
	}()

	if len(trackers) == 0 {
		return nil
	}
	var wg sync.WaitGroup
	var counter errCounter

	wg.Add(len(trackers))
	for _, so := range trackers {
		go func(s *statusTracker) {
			defer wg.Done()
			counter.add(s.wait())
		}(so)
	}

	done := make(chan struct{})
	go func() {
		wg.Wait()
		close(done)
	}()

	timeout := make(chan struct{})
	go func() {
		time.Sleep(opts.Timeout)
		close(timeout)
	}()

	select {
	case <-done:
		return counter.toSummaryError()
	case <-timeout:
		return fmt.Errorf("wait timed out after %v", opts.Timeout)
	}
}
