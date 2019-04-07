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

// Package model contains the app definition and interfaces for dealing with K8s objects.
package model

import (
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"github.com/ghodss/yaml"
	"github.com/pkg/errors"
	"github.com/splunk/qbec/internal/sio"
)

// Baseline is a special environment name that represents the baseline environment with no customizations.
const Baseline = "_"

// Default values
const (
	DefaultComponentsDir = "components"       // the default components directory
	DefaultParamsFile    = "params.libsonnet" // the default params files
)

var supportedExtensions = map[string]bool{
	".jsonnet": true,
	".yaml":    true,
	".json":    true,
}

// Component is a file that contains objects to be applied to a cluster.
type Component struct {
	Name         string   // component name
	File         string   // path to component file
	TopLevelVars []string // the top-level variables used by the component
}

// App is a qbec application wrapped with some runtime attributes.
type App struct {
	QbecApp
	root              string               // derived root directory of the app
	allComponents     map[string]Component // all components whether or not included anywhere
	defaultComponents map[string]Component // all components enabled by default
}

// NewApp returns an app loading its details from the supplied file.
func NewApp(file string) (*App, error) {
	b, err := ioutil.ReadFile(file)
	if err != nil {
		return nil, err
	}
	var qApp QbecApp
	if err := yaml.Unmarshal(b, &qApp); err != nil {
		return nil, errors.Wrap(err, "unmarshal YAML")
	}

	// validate YAML against schema
	v, err := newValidator()
	if err != nil {
		return nil, errors.Wrap(err, "create schema validator")
	}
	errs := v.validateYAML(b)
	if len(errs) > 0 {
		var msgs []string
		for _, err := range errs {
			msgs = append(msgs, err.Error())
		}
		return nil, fmt.Errorf("%d schema validation error(s): %s", len(errs), strings.Join(msgs, "\n"))
	}

	app := App{QbecApp: qApp}
	dir := filepath.Dir(file)
	if !filepath.IsAbs(dir) {
		var err error
		dir, err = filepath.Abs(dir)
		if err != nil {
			return nil, errors.Wrap(err, "abs path for "+dir)
		}
	}
	app.root = dir
	app.setupDefaults()
	app.allComponents, err = app.loadComponents()
	if err != nil {
		return nil, errors.Wrap(err, "load components")
	}
	if err := app.verifyEnvAndComponentReferences(); err != nil {
		return nil, err
	}
	if err := app.verifyVariables(); err != nil {
		return nil, err
	}

	app.updateComponentTopLevelVars()

	app.defaultComponents = make(map[string]Component, len(app.allComponents))
	for k, v := range app.allComponents {
		app.defaultComponents[k] = v
	}
	for _, k := range app.Spec.Excludes {
		delete(app.defaultComponents, k)
	}
	return &app, nil
}

func (a *App) setupDefaults() {
	if a.Spec.ComponentsDir == "" {
		a.Spec.ComponentsDir = DefaultComponentsDir
	}
	if a.Spec.ParamsFile == "" {
		a.Spec.ParamsFile = DefaultParamsFile
	}
}

// Name returns the name of the application.
func (a *App) Name() string {
	return a.Metadata.Name
}

// ComponentsForEnvironment returns a slice of components for the specified
// environment, taking intrinsic as well as specified inclusions and exclusions into account.
// All names in the supplied subsets must be valid component names. If a specified component is valid but has been excluded
// for the environment, it is simply not returned. The environment can be specified as the baseline
// environment.
func (a *App) ComponentsForEnvironment(env string, includes, excludes []string) ([]Component, error) {
	toList := func(m map[string]Component) []Component {
		var ret []Component
		for _, v := range m {
			ret = append(ret, v)
		}
		sort.Slice(ret, func(i, j int) bool {
			return ret[i].Name < ret[j].Name
		})
		return ret
	}

	cf, err := NewComponentFilter(includes, excludes)
	if err != nil {
		return nil, err
	}
	if err := a.verifyComponentList("specified components", includes); err != nil {
		return nil, err
	}
	if err := a.verifyComponentList("specified components", excludes); err != nil {
		return nil, err
	}
	ret := map[string]Component{}
	if env == Baseline {
		for k, v := range a.defaultComponents {
			ret[k] = v
		}
	} else {
		e, ok := a.Spec.Environments[env]
		if !ok {
			return nil, fmt.Errorf("invalid environment %q", env)
		}
		for k, v := range a.defaultComponents {
			ret[k] = v
		}
		for _, k := range e.Excludes {
			if _, ok := ret[k]; !ok {
				sio.Warnf("component %s excluded from %s is already excluded by default\n", k, env)
			}
			delete(ret, k)
		}
		for _, k := range e.Includes {
			if _, ok := ret[k]; ok {
				sio.Warnf("component %s included from %s is already included by default\n", k, env)
			}
			ret[k] = a.allComponents[k]
		}
	}
	if !cf.HasFilters() {
		return toList(ret), nil
	}

	for _, k := range includes {
		if _, ok := ret[k]; !ok {
			sio.Noticef("not including component %s since it is not part of the component list for %s\n", k, env)
		}
	}

	subret := map[string]Component{}
	for k, v := range ret {
		if cf.ShouldInclude(v.Name) {
			subret[k] = v
		}
	}
	return toList(subret), nil
}

// DeclaredVars returns defaults for all declared external variables, keyed by variable name.
func (a *App) DeclaredVars() map[string]interface{} {
	ret := map[string]interface{}{}
	for _, v := range a.Spec.Vars.External {
		ret[v.Name] = v.Default
	}
	return ret
}

// DeclaredTopLevelVars returns a map of all declared TLA variables, keyed by variable name.
// The values are always `true`.
func (a *App) DeclaredTopLevelVars() map[string]interface{} {
	ret := map[string]interface{}{}
	for _, v := range a.Spec.Vars.TopLevel {
		ret[v.Name] = true
	}
	return ret
}

// loadComponents loads metadata for all components for the app.
// The data is returned as a map keyed by component name. It does _not_ recurse
// into subdirectories.
func (a *App) loadComponents() (map[string]Component, error) {
	var list []Component
	dir := strings.TrimSuffix(filepath.Clean(a.Spec.ComponentsDir), "/")
	err := filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if path == dir {
			return nil
		}
		if info.IsDir() {
			return filepath.SkipDir
		}
		extension := filepath.Ext(path)
		if supportedExtensions[extension] {
			list = append(list, Component{
				Name: strings.TrimSuffix(filepath.Base(path), extension),
				File: path,
			})
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	m := make(map[string]Component, len(list))
	for _, c := range list {
		if old, ok := m[c.Name]; ok {
			return nil, fmt.Errorf("duplicate component %s, found %s and %s", c.Name, old.File, c.File)
		}
		m[c.Name] = c
	}
	return m, nil
}

func (a *App) verifyComponentList(src string, comps []string) error {
	var bad []string
	for _, c := range comps {
		if _, ok := a.allComponents[c]; !ok {
			bad = append(bad, c)
		}
	}
	if len(bad) > 0 {
		return fmt.Errorf("%s: bad component reference(s): %s", src, strings.Join(bad, ","))
	}
	return nil
}

var reEnvName = regexp.MustCompile(`^(([A-Za-z0-9][-A-Za-z0-9_.]*)?[A-Za-z0-9])$`) // XXX: duplicated in swagger

func (a *App) verifyEnvAndComponentReferences() error {
	var errs []string
	localVerify := func(src string, comps []string) {
		if err := a.verifyComponentList(src, comps); err != nil {
			errs = append(errs, err.Error())
		}
	}
	localVerify("default exclusions", a.Spec.Excludes)
	for e, env := range a.Spec.Environments {
		if e == Baseline {
			return fmt.Errorf("cannot use _ as an environment name since it has a special meaning")
		}
		if !reEnvName.MatchString(e) {
			return fmt.Errorf("invalid environment %s, must match %s", e, reEnvName)
		}
		localVerify(e+" inclusions", env.Includes)
		localVerify(e+" exclusions", env.Excludes)
		includeMap := map[string]bool{}
		for _, inc := range env.Includes {
			includeMap[inc] = true
		}
		for _, exc := range env.Excludes {
			if includeMap[exc] {
				errs = append(errs, fmt.Sprintf("env %s: component %s present in both include and exclude sections", e, exc))
			}
		}
	}

	for _, tla := range a.Spec.Vars.TopLevel {
		localVerify("components for TLA "+tla.Name, tla.Components)
	}

	if len(errs) > 0 {
		return fmt.Errorf("invalid component references\n:\t%s", strings.Join(errs, "\n\t"))
	}
	return nil
}

func (a *App) verifyVariables() error {
	seenTLA := map[string]bool{}
	for _, v := range a.Spec.Vars.TopLevel {
		if seenTLA[v.Name] {
			return fmt.Errorf("duplicate top-level variable %s", v.Name)
		}
		seenTLA[v.Name] = true
	}
	seenVar := map[string]bool{}
	for _, v := range a.Spec.Vars.External {
		if seenVar[v.Name] {
			return fmt.Errorf("duplicate external variable %s", v.Name)
		}
		seenVar[v.Name] = true
	}
	return nil
}

func (a *App) updateComponentTopLevelVars() {
	componentTLAMap := map[string][]string{}

	for _, tla := range a.Spec.Vars.TopLevel {
		for _, comp := range tla.Components {
			componentTLAMap[comp] = append(componentTLAMap[comp], tla.Name)
		}
	}

	for name, tlas := range componentTLAMap {
		comp := a.allComponents[name]
		comp.TopLevelVars = tlas
		a.allComponents[name] = comp
	}
}
