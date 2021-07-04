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
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/ghodss/yaml"
	"github.com/pkg/errors"
	"github.com/splunk/qbec/internal/filematcher"
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

// Basic http client
var httpClient = &http.Client{Timeout: time.Second * 10}

// Component is one or more logically related files that contains objects to be applied to a cluster.
type Component struct {
	Name         string   // component name
	Files        []string // path to main component file and possibly additional files
	TopLevelVars []string // the top-level variables used by the component
}

// App is a qbec application wrapped with some runtime attributes.
type App struct {
	inner             QbecApp              // the app object from serialization
	overrideNs        string               // any override to the default namespace
	tag               string               // the tag to be used for the current command invocation
	root              string               // derived root directory of the app
	allComponents     map[string]Component // all components whether or not included anywhere
	defaultComponents map[string]Component // all components enabled by default
}

func makeValError(file string, errs []error) error {
	var msgs []string
	for _, err := range errs {
		msgs = append(msgs, err.Error())
	}
	return fmt.Errorf("file: %s, %d schema validation error(s): %s", file, len(errs), strings.Join(msgs, "\n"))

}

func downloadEnvFile(url string) ([]byte, error) {
	res, err := httpClient.Get(url)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()

	if res.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("status : %s", res.Status)
	}
	payload, err := ioutil.ReadAll(res.Body)
	if err != nil {
		return nil, err
	}
	return payload, err
}

func readEnvFile(file string) ([]byte, error) {
	if filematcher.IsRemoteFile(file) {
		b, err := downloadEnvFile(file)
		if err != nil {
			return nil, errors.Wrapf(err, "download environments from %s", file)
		}
		return b, nil
	}
	return ioutil.ReadFile(file)
}

func loadEnvFiles(app *QbecApp, additionalFiles []string, v *validator) error {
	if app.Spec.Environments == nil {
		app.Spec.Environments = map[string]Environment{}
	}
	sources := map[string]string{}
	for k := range app.Spec.Environments {
		sources[k] = "inline"
	}

	var envFiles []string
	envFiles = append(envFiles, app.Spec.EnvFiles...)
	envFiles = append(envFiles, additionalFiles...)
	var allFiles []string
	for _, filePattern := range envFiles {
		matchedFiles, err := filematcher.Match(filePattern)
		if err != nil {
			return err
		}
		allFiles = append(allFiles, matchedFiles...)
	}
	for _, file := range allFiles {
		b, err := readEnvFile(file)
		if err != nil {
			return err
		}
		var qEnvs QbecEnvironmentMap
		if err := yaml.Unmarshal(b, &qEnvs); err != nil {
			return errors.Wrap(err, fmt.Sprintf("%s: unmarshal YAML", file))
		}
		errs := v.validateEnvYAML(b)
		if len(errs) > 0 {
			return makeValError(file, errs)
		}
		for k, v := range qEnvs.Spec.Environments {
			old, ok := sources[k]
			if ok {
				sio.Warnf("override env definition '%s' from file %s (previous: %s)\n", k, file, old)
			}
			sources[k] = file
			app.Spec.Environments[k] = v
		}
	}
	return nil
}

// NewApp returns an app loading its details from the supplied file.
func NewApp(file string, envFiles []string, tag string) (*App, error) {
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
		return nil, makeValError(file, errs)
	}

	if err := loadEnvFiles(&qApp, envFiles, v); err != nil {
		return nil, err
	}

	if len(qApp.Spec.Environments) == 0 {
		return nil, fmt.Errorf("%s: no environments defined for app", file)
	}

	for name, env := range qApp.Spec.Environments {
		if err := env.assertValid(); err != nil {
			return nil, errors.Wrapf(err, "verify environment %s", name)
		}
	}

	app := App{inner: qApp}
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
	if err := app.verifyProcessors(); err != nil {
		return nil, err
	}

	app.updateComponentTopLevelVars()

	app.defaultComponents = make(map[string]Component, len(app.allComponents))
	for k, v := range app.allComponents {
		app.defaultComponents[k] = v
	}
	for _, k := range app.inner.Spec.Excludes {
		delete(app.defaultComponents, k)
	}

	if tag != "" {
		if !reLabelValue.MatchString(tag) {
			return nil, fmt.Errorf("invalid tag name '%s', must match %v", tag, reLabelValue)
		}
	}

	app.tag = tag
	return &app, nil
}

// SetOverrideNamespace sets an override namespace that is returned in preference to the value
// configured in qbec.yaml for any environment.
func (a *App) SetOverrideNamespace(ns string) {
	if ns != "" {
		sio.Warnln("force default namespace to", ns)
	}
	a.overrideNs = ns
}

func (a *App) setupDefaults() {
	if a.inner.Spec.ComponentsDir == "" {
		a.inner.Spec.ComponentsDir = DefaultComponentsDir
	}
	if a.inner.Spec.ParamsFile == "" {
		a.inner.Spec.ParamsFile = DefaultParamsFile
	}
}

// Name returns the name of the application.
func (a *App) Name() string {
	return a.inner.Metadata.Name
}

// Tag returns the tag to be used for the current invocation.
func (a *App) Tag() string {
	return a.tag
}

// ParamsFile returns the runtime parameters file for the app.
func (a *App) ParamsFile() string {
	return a.inner.Spec.ParamsFile
}

func splitPath(s string) []string {
	if s == "" {
		return nil
	}
	return strings.Split(s, ":")
}

// PostProcessors returns the post processor files for the app.
func (a *App) PostProcessors() []string {
	return splitPath(a.inner.Spec.PostProcessor)
}

// LibPaths returns the library paths set up for the app.
func (a *App) LibPaths() []string {
	return a.inner.Spec.LibPaths
}

// AddComponentLabel returns if the qbec component name should be added as an object label in addition to the
// standard annotation.
func (a *App) AddComponentLabel() bool {
	return a.inner.Spec.AddComponentLabel
}

func (a *App) envObject(env string) (Environment, error) {
	envObj, ok := a.inner.Spec.Environments[env]
	if !ok {
		return envObj, fmt.Errorf("invalid environment %q", env)
	}
	return envObj, nil
}

// ServerURL returns the server URL for the supplied environment.
func (a *App) ServerURL(env string) (string, error) {
	e, err := a.envObject(env)
	if err != nil {
		return "", err
	}
	return e.Server, nil
}

// Context returns the context for the supplied environment, if set.
func (a *App) Context(env string) (string, error) {
	e, err := a.envObject(env)
	if err != nil {
		return "", err
	}
	return e.Context, nil
}

// BaseProperties returns the baseline properties defined for the app.
func (a *App) BaseProperties() map[string]interface{} {
	p := a.inner.Spec.BaseProperties
	if p == nil {
		return map[string]interface{}{}
	}
	return p
}

func deepMerge(base, overrides map[string]interface{}) map[string]interface{} {
	ret := map[string]interface{}{}
	for k, v := range base {
		ret[k] = v
	}
	for k := range overrides {
		v1, present := base[k]
		v2 := overrides[k]
		if !present {
			ret[k] = v2
			continue
		}
		v1Map, ok1 := v1.(map[string]interface{})
		v2Map, ok2 := v2.(map[string]interface{})
		if ok1 && ok2 {
			ret[k] = deepMerge(v1Map, v2Map)
			continue
		}
		ret[k] = v2
	}
	return ret
}

// Properties returns the configured properties for the supplied environment, merge patched into
// the base properties object.
func (a *App) Properties(env string) (map[string]interface{}, error) {
	if env == Baseline {
		return a.BaseProperties(), nil
	}
	e, err := a.envObject(env)
	if err != nil {
		return nil, err
	}
	eProps := e.Properties
	if e.Properties == nil {
		eProps = map[string]interface{}{}
	}
	return deepMerge(a.BaseProperties(), eProps), nil
}

// DefaultNamespace returns the default namespace for the environment, potentially
// suffixing it with any app-tag, if configured.
func (a *App) DefaultNamespace(env string) string {
	var ns string
	if a.overrideNs != "" {
		ns = a.overrideNs
	} else {
		envObj, ok := a.inner.Spec.Environments[env]
		if ok {
			ns = envObj.DefaultNamespace
		}
		if ns == "" {
			ns = "default"
		}
	}
	if a.tag != "" && a.inner.Spec.NamespaceTagSuffix {
		ns += "-" + a.tag
	}
	return ns
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
		e, err := a.envObject(env)
		if err != nil {
			return nil, err
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

// Environments returns the environments defined for the app.
func (a *App) Environments() map[string]Environment {
	return a.inner.Spec.Environments
}

// DeclaredVars returns defaults for all declared external variables, keyed by variable name.
func (a *App) DeclaredVars() map[string]interface{} {
	ret := map[string]interface{}{}
	for _, v := range a.inner.Spec.Vars.External {
		ret[v.Name] = v.Default
	}
	return ret
}

// DeclaredTopLevelVars returns a map of all declared TLA variables, keyed by variable name.
// The values are always `true`.
func (a *App) DeclaredTopLevelVars() map[string]interface{} {
	ret := map[string]interface{}{}
	for _, v := range a.inner.Spec.Vars.TopLevel {
		ret[v.Name] = true
	}
	return ret
}

// DeclaredComputedVars returns a list of all computed variables.
func (a *App) DeclaredComputedVars() []ComputedVar {
	return a.inner.Spec.Vars.Computed
}

// DataSources returns the datasource URIs defined for the app.
func (a *App) DataSources() []string {
	return a.inner.Spec.DataSources
}

// DataSourceExamples returns example output for data sources keyed by name.
func (a *App) DataSourceExamples() map[string]interface{} {
	ret := a.inner.Spec.DataSourceExamples
	if ret == nil {
		return map[string]interface{}{}
	}
	return ret
}

// loadComponents loads metadata for all components for the app. It first expands the components directory
// for glob patterns and loads components from all directories that match. It does _not_ recurse
// into subdirectories. The data is returned as a map keyed by component name.
// Note that component names must be unique across all directories. Support for multiple directories is just a
// way to partition classes of components and does not introduce any namespace semantics.
func (a *App) loadComponents() (map[string]Component, error) {
	var list []Component
	loadDirComponents := func(dir string) error {
		err := filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
			if err != nil {
				return err
			}
			if path == dir {
				return nil
			}
			if info.IsDir() {
				files, err := filepath.Glob(filepath.Join(path, "*"))
				if err != nil {
					return err
				}
				var staticFiles []string
				hasIndexJsonnet := false
				hasIndexYAML := false
				for _, f := range files {
					stat, err := os.Stat(f)
					if err != nil {
						return err
					}
					if stat.IsDir() {
						continue
					}
					switch filepath.Base(f) {
					case "index.jsonnet":
						hasIndexJsonnet = true
					case "index.yaml":
						hasIndexYAML = true
					}
					if strings.HasSuffix(f, ".json") || strings.HasSuffix(f, ".yaml") {
						staticFiles = append(staticFiles, f)
					}
				}
				switch {
				case hasIndexJsonnet:
					list = append(list, Component{
						Name:  filepath.Base(path),
						Files: []string{filepath.Join(path, "index.jsonnet")},
					})
				case hasIndexYAML:
					list = append(list, Component{
						Name:  filepath.Base(path),
						Files: staticFiles,
					})
				}
				return filepath.SkipDir
			}
			extension := filepath.Ext(path)
			if supportedExtensions[extension] {
				list = append(list, Component{
					Name:  strings.TrimSuffix(filepath.Base(path), extension),
					Files: []string{path},
				})
			}
			return nil
		})
		return err
	}
	ds, err := filepath.Glob(a.inner.Spec.ComponentsDir)
	if err != nil {
		return nil, err
	}
	var dirs []string
	for _, d := range ds {
		s, err := os.Stat(d)
		if err != nil {
			return nil, err
		}
		if s.IsDir() {
			dirs = append(dirs, d)
		}
	}
	if len(dirs) == 0 {
		return nil, fmt.Errorf("no component directories found after expanding %s", a.inner.Spec.ComponentsDir)
	}
	for _, d := range dirs {
		err := loadDirComponents(d)
		if err != nil {
			return nil, err
		}
	}
	m := make(map[string]Component, len(list))
	for _, c := range list {
		if old, ok := m[c.Name]; ok {
			return nil, fmt.Errorf("duplicate component %s, found %s and %s", c.Name, old.Files[0], c.Files[0])
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

var reLabelValue = regexp.MustCompile(`^(([A-Za-z0-9][-A-Za-z0-9_.]*)?[A-Za-z0-9])$`) // XXX: duplicated in swagger

func (a *App) verifyEnvAndComponentReferences() error {
	var errs []string
	localVerify := func(src string, comps []string) {
		if err := a.verifyComponentList(src, comps); err != nil {
			errs = append(errs, err.Error())
		}
	}
	localVerify("default exclusions", a.inner.Spec.Excludes)
	for e, env := range a.inner.Spec.Environments {
		if e == Baseline {
			return fmt.Errorf("cannot use _ as an environment name since it has a special meaning")
		}
		if !reLabelValue.MatchString(e) {
			return fmt.Errorf("invalid environment %s, must match %s", e, reLabelValue)
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

	for _, tla := range a.inner.Spec.Vars.TopLevel {
		localVerify("components for TLA "+tla.Name, tla.Components)
	}

	if len(errs) > 0 {
		return fmt.Errorf("invalid component references\n:\t%s", strings.Join(errs, "\n\t"))
	}
	return nil
}

func (a *App) verifyVariables() error {
	seenTLA := map[string]bool{}
	for _, v := range a.inner.Spec.Vars.TopLevel {
		if seenTLA[v.Name] {
			return fmt.Errorf("duplicate top-level variable %s", v.Name)
		}
		seenTLA[v.Name] = true
	}
	seenVar := map[string]bool{}
	for _, v := range a.inner.Spec.Vars.External {
		if seenVar[v.Name] {
			return fmt.Errorf("duplicate external variable %s", v.Name)
		}
		seenVar[v.Name] = true
	}
	for _, v := range a.inner.Spec.Vars.Computed {
		if seenVar[v.Name] {
			return fmt.Errorf("duplicate external variable %s", v.Name)
		}
		seenVar[v.Name] = true
	}
	return nil
}

func baseName(file string) string {
	base := filepath.Base(file)
	pos := strings.LastIndex(base, ".")
	if pos > 0 {
		base = base[:pos]
	}
	return base
}

func checkProcessors(pType string, files []string) error {
	seen := map[string]string{}
	for _, file := range files {
		b := baseName(file)
		if seen[b] != "" {
			return fmt.Errorf("invalid %s-processor '%s', has the same base name as '%s'", pType, file, seen[b])
		}
		seen[b] = file
	}
	return nil
}

func (a *App) verifyProcessors() error {
	if err := checkProcessors("post", a.PostProcessors()); err != nil {
		return err
	}
	return nil
}

func (a *App) updateComponentTopLevelVars() {
	componentTLAMap := map[string][]string{}

	for _, tla := range a.inner.Spec.Vars.TopLevel {
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

// ClusterScopedLists returns the value of the qbec app attribute to determine if cluster scope
// lists should be performed when multiple namespaces are present.
func (a *App) ClusterScopedLists() bool {
	return a.inner.Spec.ClusterScopedLists
}
