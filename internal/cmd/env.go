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

	"github.com/pkg/errors"
	"github.com/splunk/qbec/internal/eval"
	"github.com/splunk/qbec/internal/model"
	"github.com/splunk/qbec/internal/remote"
	"github.com/splunk/qbec/internal/sio"
	"github.com/splunk/qbec/vm"
	vmds "github.com/splunk/qbec/vm/datasource"
)

// EnvContext is the command context for the intersection of an app and environment
type EnvContext struct {
	AppContext
	env         string
	props       map[string]interface{}
	dataSources []vmds.DataSource
}

func (c *EnvContext) configProvider(name string) (string, error) {
	baseCtx := c.EvalContext(false).BaseContext
	code := fmt.Sprintf(`std.extVar('%s')`, name)
	return eval.Code(fmt.Sprintf("<%s>", name), vm.MakeCode(code), baseCtx)
}

func (c *EnvContext) createDataSources() error {
	sources, closer, err := vm.CreateDataSources(c.App().DataSources(), c.configProvider)
	if closer != nil {
		RegisterCleanupTask(closer)
	}
	if err != nil {
		return err
	}
	c.dataSources = sources
	return nil
}

func (c *EnvContext) computeVars() error {
	cVars := c.App().DeclaredComputedVars()
	for _, varObj := range cVars {
		name := varObj.Name
		baseCtx := c.EvalContext(false).BaseContext
		jsonData, err := eval.Code(fmt.Sprintf("<%s>", name), vm.MakeCode(varObj.Code), baseCtx)
		if err != nil {
			return errors.Wrapf(err, "eval computed var %s", name)
		}
		c.vars = c.vars.WithVars(vm.NewCodeVar(name, jsonData))
	}
	return nil
}

func (c *EnvContext) initEnv() error {
	if err := c.createDataSources(); err != nil {
		return err
	}
	if err := c.computeVars(); err != nil {
		return err
	}
	return nil
}

// Env returns the environment name for this context.
func (c EnvContext) Env() string { return c.env }

// EvalContext returns the evaluation context for the supplied environment.
func (c EnvContext) EvalContext(cleanMode bool) eval.Context {
	p, err := json.Marshal(c.props)
	if err != nil {
		sio.Warnln("unable to serialize env properties to JSON:", err)
	}
	cm := "off"
	if cleanMode {
		cm = "on"
	}
	baseVars := c.vars.WithVars(
		vm.NewVar(model.QbecNames.EnvVarName, c.env),
		vm.NewVar(model.QbecNames.TagVarName, c.app.Tag()),
		vm.NewVar(model.QbecNames.DefaultNsVarName, c.app.DefaultNamespace(c.env)),
		vm.NewVar(model.QbecNames.CleanModeVarName, cm),
		vm.NewCodeVar(model.QbecNames.EnvPropsVarName, string(p)),
	)
	return eval.Context{
		BaseContext: eval.BaseContext{
			Vars:        baseVars,
			LibPaths:    c.ext.LibPaths,
			DataSources: c.dataSources,
			Verbose:     c.Verbosity() > 1,
		},
		Concurrency:      c.EvalConcurrency(),
		PostProcessFiles: c.App().PostProcessors(),
	}
}

// ObjectProducer returns a local object producer for the app and environment.
func (c EnvContext) ObjectProducer() eval.LocalObjectProducer {
	return func(component string, data map[string]interface{}) model.K8sLocalObject {
		app := c.app
		return model.NewK8sLocalObject(data, model.LocalAttrs{
			App:               app.Name(),
			Tag:               app.Tag(),
			Component:         component,
			Env:               c.env,
			SetComponentLabel: app.AddComponentLabel(),
		})
	}
}

// Client returns a kubernetes client for the supplied environment
func (c EnvContext) Client() (KubeClient, error) {
	return c.clp(c.env)
}

// KubeAttributes returns the kubernetes attributes for the supplied environment
func (c EnvContext) KubeAttributes() (*remote.KubeAttributes, error) {
	return c.attrsp(c.env)
}
