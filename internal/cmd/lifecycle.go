/*
   Copyright 2021 Splunk Inc.

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

package cmd

import (
	"io"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"github.com/splunk/qbec/internal/sio"
)

type closers struct {
	l       sync.Mutex
	closers []io.Closer
}

func (c *closers) add(closer io.Closer) {
	c.l.Lock()
	defer c.l.Unlock()
	c.closers = append(c.closers, closer)
}

func (c *closers) close() error {
	var lastError error
	for _, c := range c.closers {
		err := c.Close()
		if err != nil {
			lastError = err
		}
	}
	return lastError // XXX: return a multi-error later
}

var cleanup = &closers{}

// RegisterCleanupTask registers an io.Closer for eventual cleanup.
func RegisterCleanupTask(closer io.Closer) {
	cleanup.add(closer)
}

// Close closes all the closers for the process and returns an error
// if any of the closers return an error.
func Close() error {
	return cleanup.close()
}

// RegisterSignalHandlers registers signal handlers for resource cleanup.
func RegisterSignalHandlers() {
	ch := make(chan os.Signal, 5)
	signal.Notify(ch, os.Interrupt, syscall.SIGTERM)
	go func() {
		<-ch
		var err error
		done := make(chan struct{})
		go func() {
			defer close(done)
			err = Close()
		}()
		gracePeriod := 5 * time.Second
		sio.Println()
		for {
			select {
			case <-time.After(gracePeriod):
				sio.Errorln("cleanup took too long, exit")
				os.Exit(1)
			case <-done:
				if err != nil {
					sio.Errorln(err)
				} else {
					sio.Errorln("interrupted")
				}
				os.Exit(1)
			}
		}
	}()
}
