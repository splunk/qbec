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
	"time"

	"github.com/spf13/cobra"
	"github.com/splunk/qbec/internal/model"
	"github.com/splunk/qbec/internal/objsort"
	"github.com/splunk/qbec/internal/remote"
	"github.com/splunk/qbec/internal/rollout"
	"github.com/splunk/qbec/internal/sio"
	"k8s.io/apimachinery/pkg/watch"
)

type applyStats struct {
	Created []string `json:"created,omitempty"`
	Updated []string `json:"updated,omitempty"`
	Skipped []string `json:"skipped,omitempty"`
	Deleted []string `json:"deleted,omitempty"`
	Same    int      `json:"same,omitempty"`
}

func (a *applyStats) update(name string, s *remote.SyncResult) {
	switch s.Type {
	case remote.SyncObjectsIdentical:
		a.Same++
	case remote.SyncSkip:
		a.Skipped = append(a.Skipped, name)
	case remote.SyncCreated:
		a.Created = append(a.Created, name)
	case remote.SyncUpdated:
		a.Updated = append(a.Updated, name)
	case remote.SyncDeleted:
		a.Deleted = append(a.Deleted, name)
	}
}

type applyCommandConfig struct {
	*config
	syncOptions remote.SyncOptions
	showDetails bool
	gc          bool
	wait        bool
	waitAll     bool
	waitTimeout time.Duration
	filterFunc  func() (filterParams, error)
}

type nameWrap struct {
	name string
	model.K8sLocalObject
}

func (nw nameWrap) GetName() string {
	return nw.name
}

type metaWrap struct {
	model.K8sMeta
}

type nsWrap struct {
	model.K8sMeta
	ns string
}

func (n nsWrap) GetNamespace() string {
	base := n.K8sMeta.GetNamespace()
	if base == "" {
		return n.ns
	}
	return base
}

var applyWaitFn = rollout.WaitUntilComplete // allow override in tests

func doApply(args []string, config applyCommandConfig) error {
	if len(args) != 1 {
		return newUsageError("exactly one environment required")
	}
	env := args[0]
	if env == model.Baseline { // cannot apply for the baseline environment
		return newUsageError("cannot apply baseline environment, use a real environment")
	}
	fp, err := config.filterFunc()
	if err != nil {
		return err
	}
	client, err := config.Client(env)
	if err != nil {
		return err
	}
	objects, err := filteredObjects(config.config, env, client.ObjectKey, fp)
	if err != nil {
		return err
	}

	opts := config.syncOptions
	opts.DisableUpdateFn = newUpdatePolicy().disableUpdate

	if !opts.DryRun && len(objects) > 0 {
		msg := fmt.Sprintf("will synchronize %d object(s)", len(objects))
		if err := config.Confirm(msg); err != nil {
			return err
		}
	}

	// prepare for GC with object list of deletions
	var lister lister = &stubLister{}
	var retainObjects []model.K8sLocalObject
	if config.gc {
		lister, retainObjects, err = startRemoteList(env, config.config, client, fp)
		if err != nil {
			return err
		}
	}

	// continue with apply
	objects = objsort.Sort(objects, sortConfig(client.IsNamespaced))

	dryRun := ""
	if opts.DryRun {
		dryRun = "[dry-run] "
	}

	var stats applyStats
	var waitObjects []model.K8sMeta

	printSyncStatus := func(name string, res *remote.SyncResult, err error) {
		if err != nil {
			sio.Errorf("%ssync %s failed\n", dryRun, name)
			return
		}
		if res.Type == remote.SyncObjectsIdentical {
			if config.Verbosity() > 0 {
				sio.Noticef("%sno changes to %s\n", dryRun, name)
				if res.Details != "" {
					sio.Println(res.Details)
				}
			}
			return
		}
		verb := "create"
		if res.Type == remote.SyncUpdated {
			verb = "update"
		}
		if res.Type == remote.SyncSkip {
			verb = "skip"
		}
		sio.Noticef("%s%s %s\n", dryRun, verb, name)
		if config.showDetails || config.Verbosity() > 0 {
			if res.Details != "" {
				sio.Println(res.Details)
			}
		}
	}

	waitPolicy := newWaitPolicy()
	for _, ob := range objects {
		name := client.DisplayName(ob)
		res, err := client.Sync(ob, opts)
		if res != nil && res.GeneratedName != "" {
			ob = nameWrap{name: res.GeneratedName, K8sLocalObject: ob}
			name = client.DisplayName(ob)
			retainObjects = append(retainObjects, ob)
		}
		printSyncStatus(name, res, err)
		if err != nil {
			return err
		}
		shouldWait := config.waitAll || (res.Type == remote.SyncCreated || res.Type == remote.SyncUpdated)
		if shouldWait {
			if waitPolicy.disableWait(ob) {
				sio.Debugf("%s: wait disabled by policy\n", name)
			} else {
				waitObjects = append(waitObjects, metaWrap{K8sMeta: ob})
			}
		}
		stats.update(name, res)
	}

	// process deletions
	deletions, err := lister.deletions(retainObjects, fp.Includes)
	if err != nil {
		return err
	}

	if !opts.DryRun && len(deletions) > 0 {
		msg := fmt.Sprintf("will delete %d object(s)", len(deletions))
		if err := config.Confirm(msg); err != nil {
			return err
		}
	}

	dp := newDeletePolicy(client.IsNamespaced, config.App().DefaultNamespace(env))
	deleteOpts := remote.DeleteOptions{DryRun: opts.DryRun, DisableDeleteFn: dp.disableDelete}

	deletions = objsort.SortMeta(deletions, sortConfig(client.IsNamespaced))

	printDelStatus := func(name string, res *remote.SyncResult, err error) {
		if err != nil {
			sio.Errorf("%sdelete %s failed\n", dryRun, name)
			return
		}
		verb := "delete"
		if res.Type == remote.SyncSkip {
			verb = "skip delete"
		}
		sio.Noticef("%s%s %s\n", dryRun, verb, name)
		if config.showDetails || config.Verbosity() > 0 {
			if res.Details != "" {
				sio.Println(res.Details)
			}
		}
	}

	for i := len(deletions) - 1; i >= 0; i-- {
		ob := deletions[i]
		name := client.DisplayName(ob)
		res, err := client.Delete(ob, deleteOpts)
		printDelStatus(name, res, err)
		if err != nil {
			return err
		}
		stats.update(name, res)
	}

	printStats(config.Stdout(), &stats)
	if opts.DryRun {
		sio.Noticeln("** dry-run mode, nothing was actually changed **")
	}

	defaultNs := config.app.DefaultNamespace(env)
	if config.wait || config.waitAll {
		wl := &waitListener{
			displayNameFn: client.DisplayName,
		}
		return applyWaitFn(waitObjects,
			func(obj model.K8sMeta) (watch.Interface, error) {
				return waitWatcher(client.ResourceInterface, nsWrap{K8sMeta: obj, ns: defaultNs})
			},
			rollout.WaitOptions{
				Listener: wl,
				Timeout:  config.waitTimeout,
			},
		)
	}

	return nil
}

func newApplyCommand(cp configProvider) *cobra.Command {
	cmd := &cobra.Command{
		Use:     "apply [-n] <environment>",
		Short:   "apply one or more components to a Kubernetes cluster",
		Example: applyExamples(),
	}

	config := applyCommandConfig{
		filterFunc: addFilterParams(cmd, true),
	}

	cmd.Flags().BoolVar(&config.syncOptions.DisableCreate, "skip-create", false, "set to true to only update existing resources but not create new ones")
	cmd.Flags().BoolVarP(&config.syncOptions.DryRun, "dry-run", "n", false, "dry-run, do not create/ update resources but show what would happen")
	cmd.Flags().BoolVarP(&config.syncOptions.ShowSecrets, "show-secrets", "S", false, "do not obfuscate secret values in the output")
	cmd.Flags().BoolVar(&config.showDetails, "show-details", false, "show details for object operations")
	cmd.Flags().BoolVar(&config.gc, "gc", true, "garbage collect extra objects on the server")
	cmd.Flags().BoolVar(&config.wait, "wait", false, "wait for objects to be ready")
	cmd.Flags().BoolVar(&config.waitAll, "wait-all", false, "wait for all objects to be ready, not just the ones that have changed")
	var waitTime string
	cmd.Flags().StringVar(&waitTime, "wait-timeout", "5m", "wait timeout")

	cmd.RunE = func(c *cobra.Command, args []string) error {
		config.config = cp()
		var err error
		config.waitTimeout, err = time.ParseDuration(waitTime)
		if err != nil {
			return newUsageError(fmt.Sprintf("invalid wait timeout: %s, %v", waitTime, err))
		}
		if config.syncOptions.DryRun {
			config.wait = false
		}
		if !cmd.Flag("show-details").Changed {
			config.showDetails = config.syncOptions.DryRun
		}
		return wrapError(doApply(args, config))
	}
	return cmd
}
