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

	"github.com/spf13/cobra"
	"github.com/splunk/qbec/internal/model"
	"github.com/splunk/qbec/internal/objsort"
	"github.com/splunk/qbec/internal/remote"
	"github.com/splunk/qbec/internal/sio"
)

// deleteClient is the remote interface needed for delete operations.
type deleteClient interface {
	listClient
	DisplayName(o model.K8sMeta) string
	Delete(obj model.K8sMeta, dryRun bool) (*remote.SyncResult, error)
}

type deleteCommandConfig struct {
	StdOptions
	dryRun         bool
	useLocal       bool
	filterFunc     func() (filterParams, error)
	clientProvider func(env string) (deleteClient, error)
}

func doDelete(args []string, config deleteCommandConfig) error {
	if len(args) != 1 {
		return newUsageError("exactly one environment required")
	}
	env := args[0]
	if env == model.Baseline { // cannot apply for the baseline environment
		return newUsageError("cannot delete baseline environment, use a real environment")
	}
	fp, err := config.filterFunc()
	if err != nil {
		return err
	}

	client, err := config.clientProvider(env)
	if err != nil {
		return err
	}

	var deletions []model.K8sQbecMeta
	if config.useLocal {
		objects, err := filteredObjects(config, env, fp)
		if err != nil {
			return err
		}
		for _, o := range objects {
			deletions = append(deletions, o)
		}
	} else {
		all, err := allObjects(config, env)
		if err != nil {
			return err
		}
		cf, err := model.NewComponentFilter(fp.includes, fp.excludes)
		if err != nil {
			return err
		}
		lister, scope, err := newRemoteLister(client, all, config.DefaultNamespace(env))
		if err != nil {
			return err
		}
		lister.start(nil, remote.ListQueryConfig{
			Application:     config.App().Name(),
			Environment:     env,
			ComponentFilter: cf,
			KindFilter:      fp.kindFilter,
			ListQueryScope:  scope,
		})
		deletions, err = lister.results()
		if err != nil {
			return err
		}
	}

	dryRun := ""
	if config.dryRun {
		dryRun = "[dry-run] "
	}

	// process deletions
	deletions = objsort.SortMeta(deletions, config.SortConfig(client.IsNamespaced))

	if !config.dryRun && len(deletions) > 0 {
		msg := fmt.Sprintf("will delete %d objects", len(deletions))
		if err := config.Confirm(msg); err != nil {
			return err
		}
	}

	var stats applyStats
	for i := len(deletions) - 1; i >= 0; i-- {
		ob := deletions[i]
		name := client.DisplayName(ob)
		res, err := client.Delete(ob, config.dryRun)
		if err != nil {
			return err
		}
		stats.update(name, res)
		sio.Noticeln(dryRun+"delete", name)
		sio.Println(res.Details)
	}

	printStats(config.Stdout(), &stats)
	if config.dryRun {
		sio.Noticeln("** dry-run mode, nothing was actually changed **")
	}
	return nil
}

func newDeleteCommand(op OptionsProvider) *cobra.Command {
	cmd := &cobra.Command{
		Use:     "delete [-n] <environment>",
		Short:   "delete one or more components from a Kubernetes cluster",
		Example: deleteExamples(),
	}

	config := deleteCommandConfig{
		clientProvider: func(env string) (deleteClient, error) {
			return op().Client(env)
		},
		filterFunc: addFilterParams(cmd, true),
	}

	cmd.Flags().BoolVarP(&config.dryRun, "dry-run", "n", false, "dry-run, do not delete resources but show what would happen")
	cmd.Flags().BoolVar(&config.useLocal, "local", false, "use local object names to delete, do not derive list from server")

	cmd.RunE = func(c *cobra.Command, args []string) error {
		config.StdOptions = op()
		return wrapError(doDelete(args, config))
	}
	return cmd
}
