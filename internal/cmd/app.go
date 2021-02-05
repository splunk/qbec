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
	"encoding/json"
	"fmt"
	"strings"

	"github.com/splunk/qbec/internal/model"
	"github.com/splunk/qbec/internal/sio"
	"github.com/splunk/qbec/internal/vm"
)

// AppContext is a context that also has a validated app.
type AppContext struct {
	Context
	app  *model.App
	vars vm.VariableSet
	vmc  vm.Config
}

// App returns the app set up for this context.
func (c AppContext) App() *model.App {
	return c.app
}

func (c *AppContext) init() error {
	var msgs []string
	c.ext = c.ext.WithLibPaths(c.app.LibPaths())
	vs := vm.VariablesFromConfig(c.ext)
	vars := vs.Vars()
	tlaVars := vs.TopLevelVars()

	declaredExternals := c.app.DeclaredVars()
	declaredTLAs := c.app.DeclaredTopLevelVars()

	checkStrict := func(tla bool, declared map[string]interface{}, src []vm.Var) {
		kind := "external"
		if tla {
			kind = "top level"
		}
		// check that all specified variables have been declared
		for _, v := range src {
			_, ok := declared[v.Name]
			if !ok {
				msgs = append(msgs, fmt.Sprintf("specified %s variable '%s' not declared for app", kind, v.Name))
			}
		}
		// check that all declared variables have been specified
		var fn func(string) bool
		if tla {
			fn = vs.HasTopLevelVar
		} else {
			fn = vs.HasVar
		}
		for k := range declared {
			ok := fn(k)
			if !ok {
				msgs = append(msgs, fmt.Sprintf("declared %s variable '%s' not specfied for command", kind, k))
			}
		}
	}

	if c.strictVars {
		checkStrict(false, declaredExternals, vars)
		checkStrict(true, declaredTLAs, tlaVars)
		if len(msgs) > 0 {
			return fmt.Errorf("strict vars check failures\n\t%s", strings.Join(msgs, "\n\t"))
		}
	}

	// apply default values for external vars
	var addVars []vm.Var

	for k, v := range declaredExternals {
		if vs.HasVar(k) {
			continue
		}
		if v == nil {
			sio.Warnf("no/ nil default specified for variable %q\n", k)
			continue
		}
		switch t := v.(type) {
		case string:
			addVars = append(addVars, vm.NewVar(k, t))
		default:
			b, err := json.Marshal(v)
			if err != nil {
				return fmt.Errorf("json marshal: unexpected error marshaling default for variable %s, %v", k, err)
			}
			addVars = append(addVars, vm.NewCodeVar(k, string(b)))
		}
	}
	c.vars = vs.WithVars(addVars...)
	c.vmc = vm.Config{
		LibPaths: c.ext.LibPaths,
	}
	return nil
}

// EnvContext returns an execution context for the specified environment.
func (c AppContext) EnvContext(env string) (EnvContext, error) {
	props, err := c.app.Properties(env)
	if err != nil {
		return EnvContext{}, err
	}
	ret := EnvContext{AppContext: c, env: env, props: props}
	fc, err := c.forceOptsFn()
	if err != nil {
		return EnvContext{}, err
	}
	sp := stdClientProvider{
		app:          c.app,
		config:       c.remote,
		verbosity:    c.verbose,
		forceContext: fc.K8sContext,
	}
	if ret.clp == nil {
		ret.clp = sp.Client
	}
	if ret.attrsp == nil {
		ret.attrsp = sp.Attrs
	}
	return ret, nil
}
