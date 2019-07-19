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
	*Config
	syncOptions remote.SyncOptions
	gc          bool
	wait        bool
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
	objects, err := filteredObjects(config.Config, env, client.ObjectKey, fp)
	if err != nil {
		return err
	}

	opts := config.syncOptions
	if !opts.DryRun && len(objects) > 0 {
		msg := fmt.Sprintf("will synchronize %d object(s)", len(objects))
		if err := config.Confirm(msg); err != nil {
			return err
		}
	}

	// prepare for GC with object list of deletions
	var lister lister = &stubLister{}
	var all []model.K8sLocalObject
	var retainObjects []model.K8sLocalObject
	if config.gc {
		all, err = allObjects(config.Config, env)
		if err != nil {
			return err
		}
		for _, o := range all {
			if o.GetName() != "" {
				retainObjects = append(retainObjects, o)
			}
		}
		var scope remote.ListQueryScope
		lister, scope, err = newRemoteLister(client, all, config.app.DefaultNamespace(env))
		if err != nil {
			return err
		}
		lister.start(remote.ListQueryConfig{
			Application:    config.App().Name(),
			Tag:            config.App().Tag(),
			Environment:    env,
			KindFilter:     fp.kindFilter,
			ListQueryScope: scope,
		})
	}

	// continue with apply
	objects = objsort.Sort(objects, sortConfig(client.IsNamespaced))

	dryRun := ""
	if opts.DryRun {
		dryRun = "[dry-run] "
	}

	var stats applyStats
	var changedObjects []model.K8sMeta

	for _, ob := range objects {
		res, err := client.Sync(ob, opts)
		if err != nil {
			return err
		}
		if res.Type == remote.SyncCreated || res.Type == remote.SyncUpdated {
			changedObjects = append(changedObjects, metaWrap{K8sMeta: ob})
		}
		if res.GeneratedName != "" {
			ob = nameWrap{name: res.GeneratedName, K8sLocalObject: ob}
			retainObjects = append(retainObjects, ob)
		}
		name := client.DisplayName(ob)
		stats.update(name, res)
		show := res.Type != remote.SyncObjectsIdentical || config.Verbosity() > 0
		if show {
			sio.Noticeln(dryRun+"sync", name)
			sio.Println(res.Details)
		}
	}

	// process deletions
	deletions, err := lister.deletions(retainObjects, fp.Includes)
	if err != nil {
		return err
	}

	if !opts.DryRun && len(deletions) > 0 {
		msg := fmt.Sprintf("will delete %d object(s))", len(deletions))
		if err := config.Confirm(msg); err != nil {
			return err
		}
	}

	deletions = objsort.SortMeta(deletions, sortConfig(client.IsNamespaced))
	for i := len(deletions) - 1; i >= 0; i-- {
		ob := deletions[i]
		name := client.DisplayName(ob)
		res, err := client.Delete(ob, opts.DryRun)
		if err != nil {
			return err
		}
		stats.update(name, res)
		sio.Noticeln(dryRun+"delete", name)
		sio.Println(res.Details)
	}

	printStats(config.Stdout(), &stats)
	if opts.DryRun {
		sio.Noticeln("** dry-run mode, nothing was actually changed **")
	}

	defaultNs := config.app.DefaultNamespace(env)
	if config.wait {
		wl := &waitListener{
			displayNameFn: client.DisplayName,
		}
		return applyWaitFn(changedObjects,
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

func newApplyCommand(cp ConfigProvider) *cobra.Command {
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
	cmd.Flags().BoolVar(&config.gc, "gc", true, "garbage collect extra objects on the server")
	cmd.Flags().BoolVar(&config.wait, "wait", false, "wait for objects to be ready")
	var waitTime string
	cmd.Flags().StringVar(&waitTime, "wait-timeout", "5m", "wait timeout")

	cmd.RunE = func(c *cobra.Command, args []string) error {
		config.Config = cp()
		var err error
		config.waitTimeout, err = time.ParseDuration(waitTime)
		if err != nil {
			return newUsageError(fmt.Sprintf("invalid wait timeout: %s, %v", waitTime, err))
		}
		if config.syncOptions.DryRun {
			config.wait = false
		}
		return wrapError(doApply(args, config))
	}
	return cmd
}
