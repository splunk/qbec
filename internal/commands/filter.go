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
	"context"
	"fmt"

	"github.com/spf13/cobra"
	"github.com/splunk/qbec/internal/cmd"
	"github.com/splunk/qbec/internal/eval"
	"github.com/splunk/qbec/internal/filter"
	"github.com/splunk/qbec/internal/model"
	"github.com/splunk/qbec/internal/sio"
)

func addFilterParams(c *cobra.Command, includeAllFilters bool) func() (filter.Params, error) {
	fn := filter.NewParams(c.Flags(), includeAllFilters)
	return func() (filter.Params, error) {
		p, err := fn()
		if err != nil {
			return p, cmd.NewUsageError(err.Error())
		}
		return p, nil
	}
}

// keyFunc is a function that provides a string key for an object
type keyFunc func(object model.K8sMeta) string

func displayName(obj model.K8sLocalObject) string {
	group := obj.GroupVersionKind().Group
	if group != "" {
		group += "/"
	}
	ns := obj.GetNamespace()
	if ns != "" {
		ns += "/"
	}
	return fmt.Sprintf("%s%s %s%s (component: %s)", group, obj.GetKind(), ns, obj.GetName(), obj.Component())
}

func checkDuplicates(objects []model.K8sLocalObject, kf keyFunc) error {
	if kf == nil {
		return nil
	}
	objectsByKey := map[string]model.K8sLocalObject{}
	for _, o := range objects {
		if o.GetName() == "" { // generated name
			continue
		}
		key := kf(o)
		if prev, ok := objectsByKey[key]; ok {
			return fmt.Errorf("duplicate objects %s and %s", displayName(prev), displayName(o))
		}
		objectsByKey[key] = o
	}
	return nil
}

var cleanEvalMode bool

type filterOpts struct {
	fp     filter.Params
	client filter.Namespaced
	kf     keyFunc
}

func filteredObjects(_ context.Context, envCtx cmd.EnvContext, opts filterOpts) ([]model.K8sLocalObject, error) {
	fp := opts.fp
	client := opts.client
	components, err := envCtx.App().ComponentsForEnvironment(envCtx.Env(), fp.ComponentIncludes(), fp.ComponentExcludes())
	if err != nil {
		return nil, err
	}
	output, err := eval.Components(components, envCtx.EvalContext(cleanEvalMode), envCtx.ObjectProducer())
	if err != nil {
		return nil, err
	}
	if err := checkDuplicates(output, opts.kf); err != nil {
		return nil, err
	}
	if len(output) == 0 {
		return output, nil
	}

	if fp.HasNamespaceFilters() && client == nil {
		client, err = envCtx.Client()
		if err != nil {
			return nil, err
		}
	}
	defaultNs := envCtx.App().DefaultNamespace(envCtx.Env())

	var ret []model.K8sLocalObject
	for _, o := range output {
		m, err := fp.Match(o, client, defaultNs)
		if err != nil {
			return nil, err
		}
		if m {
			ret = append(ret, o)
		}
	}
	if len(ret) == 0 {
		sio.Warnf("0 of %d matches after applying filters, check for typos and kind abbreviations\n", len(output))
	}
	return ret, nil
}
