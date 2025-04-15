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
//
// This file is derived from:
// https://github.com/kubernetes/client-go/blob/master/tools/watch/until.go
// Licensed under the Apache License, Version 2.0.

package rollout

import (
	"errors"
	"time"

	"k8s.io/apimachinery/pkg/watch"
)

// errWaitTimeout is returned when the condition exited without success.
var errWaitTimeout = errors.New("timed out waiting for the condition")

// conditionFunc returns true if the condition has been reached, false if it has not been reached yet,
// or an error if the condition cannot be checked and should terminate. In general, it is better to define
// level driven conditions over edge driven conditions (pod has ready=true, vs pod modified and ready changed
// from false to true).
type conditionFunc func(event watch.Event) (bool, error)

// errWatchClosed is returned when the watch channel is closed before timeout in Until.
var errWatchClosed = errors.New("watch closed before Until timeout")

// Until reads items from the watch until each provided condition succeeds, and then returns the last watch
// encountered. The first condition that returns an error terminates the watch (and the event is also returned).
// If no event has been received, the returned event will be nil.
// Conditions are satisfied sequentially so as to provide a useful primitive for higher level composition.
// A zero timeout means to wait forever.
func until(timeout time.Duration, watcher watch.Interface, conditions ...conditionFunc) (*watch.Event, error) {
	ch := watcher.ResultChan()
	defer watcher.Stop()
	var after <-chan time.Time
	if timeout > 0 {
		after = time.After(timeout)
	} else {
		ch := make(chan time.Time)
		defer close(ch)
		after = ch
	}
	var lastEvent *watch.Event
	for _, condition := range conditions {
		// check the next condition against the previous event and short circuit waiting for the next watch
		if lastEvent != nil {
			done, err := condition(*lastEvent)
			if err != nil {
				return lastEvent, err
			}
			if done {
				continue
			}
		}
	outer:
		for {
			select {
			case event, ok := <-ch:
				if !ok {
					return lastEvent, errWatchClosed
				}
				lastEvent = &event

				// TODO: check for watch expired error and retry watch from latest point?
				done, err := condition(event)
				if err != nil {
					return lastEvent, err
				}
				if done {
					break outer
				}

			case <-after:
				return lastEvent, errWaitTimeout
			}
		}
	}
	return lastEvent, nil
}
