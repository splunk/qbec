package cmd

import (
	"encoding/json"

	"github.com/splunk/qbec/internal/eval"
	"github.com/splunk/qbec/internal/model"
	"github.com/splunk/qbec/internal/remote"
	"github.com/splunk/qbec/internal/sio"
	"github.com/splunk/qbec/internal/vm"
)

// EnvContext is the command context for the intersection of an app and environment
type EnvContext struct {
	AppContext
	env   string
	props map[string]interface{}
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
			Vars:     baseVars,
			LibPaths: c.ext.LibPaths,
			Verbose:  c.Verbosity() > 1,
		},
		Concurrency:      c.EvalConcurrency(),
		PreProcessFiles:  c.App().PreProcessors(),
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
