package rollout

import (
	"fmt"
	"reflect"
	"sync"
	"time"

	"github.com/pkg/errors"
	"github.com/splunk/qbec/internal/model"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/watch"
)

// Revisioned provides a current object revision and is an optional interface
// that can be implemented by a K8sMeta object specified for waiting on.
type Revisioned interface {
	Revision() int64
}

// ObjectStatus is the opaque status of an object.
type ObjectStatus struct {
	Description string // the description of status for display
	Done        bool   // indicator if the status is "ready"
}

func (s *ObjectStatus) withDesc(desc string) *ObjectStatus {
	s.Description = desc
	return s
}

func (s *ObjectStatus) withDone(done bool) *ObjectStatus {
	s.Done = done
	return s
}

type statusFunc func(obj *unstructured.Unstructured, revision int64) (status *ObjectStatus, err error)

func statusFuncFor(obj model.K8sMeta) statusFunc {
	gk := obj.GroupVersionKind().GroupKind()
	switch gk {
	case schema.GroupKind{Group: "apps", Kind: "Deployment"},
		schema.GroupKind{Group: "extensions", Kind: "Deployment"}:
		return deploymentStatus
	case schema.GroupKind{Group: "apps", Kind: "DaemonSet"},
		schema.GroupKind{Group: "extensions", Kind: "DaemonSet"}:
		return daemonsetStatus
	case schema.GroupKind{Group: "apps", Kind: "StatefulSet"}:
		return statefulsetStatus
	default:
		return nil
	}
}

type statusObject struct {
	obj  model.K8sRevisionedMeta
	fn   statusFunc
	ri   WatchProvider
	opts WaitOptions
}

func (s *statusObject) wait() (finalErr error) {
	defer func() {
		if finalErr != nil {
			s.opts.Listener.OnError(s.obj, finalErr)
		}
	}()
	watchXface, err := s.ri(s.obj)
	if err != nil {
		return errors.Wrap(err, "get watch interface")
	}
	var prevStatus ObjectStatus
	_, err = watch.Until(0, watchXface, func(e watch.Event) (bool, error) {
		switch e.Type {
		case watch.Deleted:
			return false, fmt.Errorf("object was deleted while waiting")
		case watch.Error:
			return false, fmt.Errorf("watch error: %v", e.Object)
		}
		un, ok := e.Object.(*unstructured.Unstructured)
		if !ok {
			return false, fmt.Errorf("dunno how to process watch object of type %v", reflect.TypeOf(e.Object))
		}
		status, err := s.fn(un, s.obj.Revision())
		if err != nil {
			return false, err
		}
		if prevStatus != *status {
			prevStatus = *status
			s.opts.Listener.OnStatusChange(s.obj, prevStatus)
		}
		if status.Done {
			return true, nil
		}
		return false, nil
	})

	return err
}

// StatusListener receives status update callbacks.
type StatusListener interface {
	OnInit(objects []model.K8sMeta)                       // the set of objects that are being monitored
	OnStatusChange(object model.K8sMeta, rs ObjectStatus) // status for specified object
	OnError(object model.K8sMeta, err error)              // watch error of some kind for specified object
	OnEnd(err error)                                      // end of status updates with final error
}

type nopListener struct{}

func (n nopListener) OnInit(objects []model.K8sMeta)                       {}
func (n nopListener) OnStatusChange(object model.K8sMeta, rs ObjectStatus) {}
func (n nopListener) OnError(object model.K8sMeta, err error)              {}
func (n nopListener) OnEnd(err error)                                      {}

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

type multiErrors struct {
	l      sync.Mutex
	errors []error
}

func (m *multiErrors) add(err error) {
	if err == nil {
		return
	}
	m.l.Lock()
	defer m.l.Unlock()
	m.errors = append(m.errors, err)
}

func (m *multiErrors) toSummaryError() error {
	m.l.Lock()
	defer m.l.Unlock()
	if len(m.errors) == 0 {
		return nil
	}
	return fmt.Errorf("%d wait errors", len(m.errors))
}

// WaitUntilComplete waits for the supplied objects to be ready and returns when they are. An error is returned
// if the function times out before all objects are ready. Any status listener provider is notified of
// individual status changes during the wait.
func WaitUntilComplete(objects []model.K8sRevisionedMeta, ri WatchProvider, opts WaitOptions) (finalErr error) {
	opts.setupDefaults()

	var statusObjects []*statusObject
	var watchObjects []model.K8sMeta
	for _, obj := range objects {
		fn := statusFuncFor(obj)
		if fn != nil {
			watchObjects = append(watchObjects, obj)
			statusObjects = append(statusObjects, &statusObject{obj: obj, fn: fn, ri: ri, opts: opts})
		}
	}
	opts.Listener.OnInit(watchObjects)
	defer func() {
		opts.Listener.OnEnd(finalErr)
	}()

	if len(statusObjects) == 0 {
		return nil
	}
	var wg sync.WaitGroup
	var errors multiErrors

	wg.Add(len(statusObjects))
	for _, so := range statusObjects {
		go func(s *statusObject) {
			defer wg.Done()
			errors.add(s.wait())
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
		return errors.toSummaryError()
	case <-timeout:
		return fmt.Errorf("rollout wait timed out after %v", opts.Timeout)
	}
}
