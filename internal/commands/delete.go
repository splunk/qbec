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

package commands

import (
	"context"
	"fmt"

	"github.com/spf13/cobra"
	"github.com/splunk/qbec/internal/cmd"
	"github.com/splunk/qbec/internal/model"
	"github.com/splunk/qbec/internal/objsort"
	"github.com/splunk/qbec/internal/remote"
	"github.com/splunk/qbec/internal/sio"
)

type deleteCommandConfig struct {
	cmd.AppContext
	dryRun     bool
	useLocal   bool
	filterFunc func() (model.Filters, error)
}

func doDelete(ctx context.Context, args []string, config deleteCommandConfig) error {
	if len(args) != 1 {
		return cmd.NewUsageError(fmt.Sprintf("exactly one environment required, but provided: %q", args))
	}
	env := args[0]
	if env == model.Baseline { // cannot apply for the baseline environment
		return cmd.NewUsageError("cannot delete baseline environment, use a real environment")
	}
	fp, err := config.filterFunc()
	if err != nil {
		return err
	}
	envCtx, err := config.EnvContext(env)
	if err != nil {
		return err
	}

	client, err := envCtx.Client()
	if err != nil {
		return err
	}

	var deletions []model.K8sQbecMeta
	if config.useLocal {
		objects, err := generateObjects(ctx, envCtx, makeFilterOpts(fp, client))
		if err != nil {
			return err
		}
		for _, o := range objects {
			if o.GetName() != "" {
				deletions = append(deletions, o)
			}
		}
	} else {
		lister, _, err := startRemoteList(ctx, envCtx, client, fp)
		if err != nil {
			return err
		}
		deletions, err = lister.deletions(nil, fp.Match)
		if err != nil {
			return err
		}
	}

	dryRun := ""
	if config.dryRun {
		dryRun = "[dry-run] "
	}

	// process deletions
	deletions = objsort.SortMeta(deletions, sortConfig(client.IsNamespaced))

	if !config.dryRun && len(deletions) > 0 {
		msg := fmt.Sprintf("will delete %d object(s)", len(deletions))
		if err := config.Confirm(msg); err != nil {
			return err
		}
	}

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
		if res.Details != "" {
			sio.Println(res.Details)
		}
	}

	var stats applyStats
	dp := newDeletePolicy(client.IsNamespaced, config.App().DefaultNamespace(env))
	delOpts := remote.DeleteOptions{
		DryRun:          config.dryRun,
		DisableDeleteFn: dp.disableDelete,
	}
	for i := len(deletions) - 1; i >= 0; i-- {
		ob := deletions[i]
		name := client.DisplayName(ob)
		res, err := client.Delete(ctx, ob, delOpts)
		printDelStatus(name, res, err)
		if err != nil {
			return err
		}
		stats.update(name, res)
	}

	printStats(config.Stdout(), &stats)
	if config.dryRun {
		sio.Noticeln("** dry-run mode, nothing was actually changed **")
	}
	return nil
}

func newDeleteCommand(cp ctxProvider) *cobra.Command {
	c := &cobra.Command{
		Use:     "delete [-n] <environment>",
		Short:   "delete one or more components from a Kubernetes cluster",
		Example: deleteExamples(),
	}

	config := deleteCommandConfig{
		filterFunc: addFilterParams(c, true),
	}

	c.Flags().BoolVarP(&config.dryRun, "dry-run", "n", false, "dry-run, do not delete resources but show what would happen")
	c.Flags().BoolVar(&config.useLocal, "local", false, "use local object names to delete, do not derive list from server")

	c.RunE = func(c *cobra.Command, args []string) error {
		config.AppContext = cp()
		return cmd.WrapError(doDelete(c.Context(), args, config))
	}
	return c
}
