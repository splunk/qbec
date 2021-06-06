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
	"strconv"
	"strings"
	"sync"

	"github.com/ghodss/yaml"
	"github.com/pkg/errors"
	"github.com/spf13/cobra"
	"github.com/splunk/qbec/internal/cmd"
	"github.com/splunk/qbec/internal/model"
	"github.com/splunk/qbec/internal/objsort"
	"github.com/splunk/qbec/internal/remote"
	"github.com/splunk/qbec/internal/sio"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

// ctxProvider provides standard configuration available to all commands
type ctxProvider func() cmd.AppContext

// setupCommands sets up all subcommands for the supplied root command.
func setupCommands(root *cobra.Command, cp ctxProvider) {
	root.AddCommand(newApplyCommand(cp))
	root.AddCommand(newValidateCommand(cp))
	root.AddCommand(newShowCommand(cp))
	root.AddCommand(newEvalCommand(cp))
	root.AddCommand(newDiffCommand(cp))
	root.AddCommand(newDeleteCommand(cp))
	root.AddCommand(newComponentCommand(cp))
	root.AddCommand(newParamCommand(cp))
	root.AddCommand(newEnvCommand(cp))
	root.AddCommand(newInitCommand(cp))
	root.AddCommand(newCompletionCommand(root))
	root.AddCommand(newFmtCommand(cp))
	alplhaCmd := newAlphaCommand()
	alplhaCmd.AddCommand(newFmtCommand(cp))
	root.AddCommand(alplhaCmd)
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

func startRemoteList(envCtx cmd.EnvContext, client cmd.KubeClient, fp filterParams) (_ lister, retainObjects []model.K8sLocalObject, _ error) {
	all, err := filteredObjects(envCtx, nil, filterParams{})
	if err != nil {
		return nil, nil, err
	}
	for _, o := range all {
		if o.GetName() != "" {
			retainObjects = append(retainObjects, o)
		}
	}
	var scope remote.ListQueryScope
	lister, scope, err := newRemoteLister(client, all, envCtx.App().DefaultNamespace(envCtx.Env()))
	if err != nil {
		return nil, nil, err
	}
	clusterScopedLists := false
	if len(scope.Namespaces) > 1 && envCtx.App().ClusterScopedLists() {
		clusterScopedLists = true
	}
	lister.start(remote.ListQueryConfig{
		Application:        envCtx.App().Name(),
		Tag:                envCtx.App().Tag(),
		Environment:        envCtx.Env(),
		KindFilter:         fp.GVKFilter,
		ListQueryScope:     scope,
		ClusterScopedLists: clusterScopedLists,
	})
	return lister, retainObjects, nil
}

func ordering(item model.K8sQbecMeta) int {
	a := item.GetAnnotations()
	if a == nil {
		return 0
	}
	v := a[model.QbecNames.Directives.ApplyOrder]
	if v == "" {
		return 0
	}
	val, err := strconv.Atoi(v)
	if err != nil {
		sio.Warnf("invalid apply order directive '%s' for %s, ignored\n", v, model.NameForDisplay(item))
		return 0
	}
	return val
}

// sortConfig returns the sort configuration.
func sortConfig(provider objsort.Namespaced) objsort.Config {
	return objsort.Config{
		NamespacedIndicator: func(gvk schema.GroupVersionKind) (bool, error) {
			ret, err := provider(gvk)
			if err != nil {
				return false, err
			}
			return ret, nil
		},
		OrderingProvider: ordering,
	}
}
