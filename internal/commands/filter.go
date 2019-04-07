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
	"github.com/spf13/cobra"
	"github.com/splunk/qbec/internal/eval"
	"github.com/splunk/qbec/internal/model"
	"github.com/splunk/qbec/internal/sio"
)

type filterParams struct {
	includes   []string
	excludes   []string
	kindFilter model.Filter
}

func addFilterParams(cmd *cobra.Command, includeKindFilters bool) func() (filterParams, error) {
	var includes, excludes, kindIncludes, kindExcludes []string

	cmd.Flags().StringArrayVarP(&includes, "component", "c", nil, "include just this component")
	cmd.Flags().StringArrayVarP(&excludes, "exclude-component", "C", nil, "exclude this component")
	if includeKindFilters {
		cmd.Flags().StringArrayVarP(&kindIncludes, "kind", "k", nil, "include objects with this kind")
		cmd.Flags().StringArrayVarP(&kindExcludes, "exclude-kind", "K", nil, "exclude objects with this kind")
	}
	return func() (filterParams, error) {
		if len(includes) > 0 && len(excludes) > 0 {
			return filterParams{}, newUsageError("cannot include as well as exclude components, specify one or the other")
		}
		of, err := model.NewKindFilter(kindIncludes, kindExcludes)
		if err != nil {
			return filterParams{}, newUsageError(err.Error())
		}
		return filterParams{
			includes:   includes,
			excludes:   excludes,
			kindFilter: of,
		}, nil
	}
}

func allObjects(cfg *Config, env string) ([]model.K8sLocalObject, error) {
	return filteredObjects(cfg, env, filterParams{kindFilter: nil})
}

func filteredObjects(cfg *Config, env string, fp filterParams) ([]model.K8sLocalObject, error) {
	components, err := cfg.App().ComponentsForEnvironment(env, fp.includes, fp.excludes)
	if err != nil {
		return nil, err
	}
	output, err := eval.Components(components, eval.Context{
		App:         cfg.App().Name(),
		Tag:         cfg.App().Tag(),
		Env:         env,
		VMConfig:    cfg.VMConfig,
		Verbose:     cfg.Verbosity() > 1,
		Concurrency: cfg.EvalConcurrency(),
	})
	if err != nil {
		return nil, err
	}
	of := fp.kindFilter
	if of == nil || !of.HasFilters() {
		return output, nil
	}
	var ret []model.K8sLocalObject
	for _, o := range output {
		if of.ShouldInclude(o.GetKind()) {
			ret = append(ret, o)
		}
	}
	if len(output) > 0 && len(ret) == 0 {
		sio.Warnf("0 of %d matches for kind filter, check for typos and abbreviations\n", len(output))
	}
	return ret, nil
}
