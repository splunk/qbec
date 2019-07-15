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
	"io"
	"sync"
	"time"

	"github.com/splunk/qbec/internal/model"
	"github.com/splunk/qbec/internal/rollout"
	"github.com/splunk/qbec/internal/sio"
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
		sio.Noticef("%6s: rollout complete\n", w.since())
		return
	}
	if len(w.remaining) == 0 {
		return
	}
	sio.Printf("%6s: rollout not complete for the following %d objects\n", w.since(), len(w.remaining))
	for name := range w.remaining {
		sio.Printf("\t- %s\n", name)
	}
}
