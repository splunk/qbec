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

// Package commands contains the implementation of all qbec commands.
package commands

import (
	"fmt"
	"io"
	"strings"
	"sync"

	"github.com/ghodss/yaml"
	"github.com/pkg/errors"
	"github.com/spf13/cobra"
	"github.com/splunk/qbec/internal/model"
	"github.com/splunk/qbec/internal/remote"
	"github.com/splunk/qbec/internal/remote/k8smeta"
	"github.com/splunk/qbec/internal/sio"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

// usageError indicates that the user supplied incorrect arguments or flags to the command.
type usageError struct {
	error
}

// newUsageError returns a usage error
func newUsageError(msg string) error {
	return &usageError{
		error: errors.New(msg),
	}
}

// isUsageError returns if the supplied error was caused due to incorrect command usage.
func isUsageError(err error) bool {
	_, ok := err.(*usageError)
	return ok
}

// runtimeError indicates that there were runtime issues with execution.
type runtimeError struct {
	error
}

// newRuntimeError returns a runtime error
func newRuntimeError(err error) error {
	return &runtimeError{
		error: err,
	}
}

// IsRuntimeError returns if the supplied error was a runtime error as opposed to an error arising out of user input.
func IsRuntimeError(err error) bool {
	_, ok := err.(*runtimeError)
	return ok
}

// wrapError passes through usage errors and wraps all other errors with a runtime marker.
func wrapError(err error) error {
	if err == nil {
		return nil
	}
	if isUsageError(err) {
		return err
	}
	return newRuntimeError(err)
}

// Client encapsulates all remote operations needed for the superset of all commands.
type Client interface {
	DisplayName(o model.K8sMeta) string
	IsNamespaced(kind schema.GroupVersionKind) (bool, error)
	Get(obj model.K8sMeta) (*unstructured.Unstructured, error)
	Sync(obj model.K8sLocalObject, opts remote.SyncOptions) (*remote.SyncResult, error)
	ValidatorFor(gvk schema.GroupVersionKind) (k8smeta.Validator, error)
	ListExtraObjects(ignore []model.K8sQbecMeta, scope remote.ListQueryConfig) ([]model.K8sQbecMeta, error)
	Delete(obj model.K8sMeta, dryRun bool) (*remote.SyncResult, error)
}

// ConfigProvider provides standard configuration available to all commands
type ConfigProvider func() *Config

// Setup sets up all subcommands for the supplied root command.
func Setup(root *cobra.Command, cp ConfigProvider) {
	root.AddCommand(newApplyCommand(cp))
	root.AddCommand(newValidateCommand(cp))
	root.AddCommand(newShowCommand(cp))
	root.AddCommand(newDiffCommand(cp))
	root.AddCommand(newDeleteCommand(cp))
	root.AddCommand(newComponentCommand(cp))
	root.AddCommand(newParamCommand(cp))
	root.AddCommand(newInitCommand())
}

type worker func(object model.K8sLocalObject) error

func runInParallel(objs []model.K8sLocalObject, worker worker, parallel int) error {
	if parallel <= 0 {
		parallel = 1
	}

	ch := make(chan model.K8sLocalObject, len(objs))
	for _, o := range objs {
		ch <- o
	}
	close(ch)

	var wg sync.WaitGroup

	errs := make(chan error, parallel)
	for i := 0; i < parallel; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for o := range ch {
				err := worker(o)
				if err != nil {
					errs <- errors.Wrap(err, fmt.Sprint(o))
					return
				}
			}
		}()
	}
	wg.Wait()
	close(errs)

	errMsgs := []string{}
	for e := range errs {
		errMsgs = append(errMsgs, e.Error())
	}
	if len(errMsgs) > 0 {
		return errors.New(strings.Join(errMsgs, "\n"))
	}
	return nil
}

func printStats(w io.Writer, stats interface{}) {
	summary := struct {
		Stats interface{} `json:"stats"`
	}{stats}
	b, err := yaml.Marshal(summary)
	if err != nil {
		sio.Warnln("unable to print summary stats", err)
	}
	fmt.Fprintf(w, "---\n%s\n", b)
}

type lockWriter struct {
	io.Writer
	l sync.Mutex
}

func (lw *lockWriter) Write(buf []byte) (int, error) {
	lw.l.Lock()
	n, err := lw.Writer.Write(buf)
	lw.l.Unlock()
	return n, err
}
