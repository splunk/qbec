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
	"io"
	"sort"
	"sync"

	"github.com/ghodss/yaml"
	"github.com/spf13/cobra"
	"github.com/splunk/qbec/internal/diff"
	"github.com/splunk/qbec/internal/model"
	"github.com/splunk/qbec/internal/objsort"
	"github.com/splunk/qbec/internal/remote"
	"github.com/splunk/qbec/internal/sio"
	"github.com/splunk/qbec/internal/types"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

type diffIgnores struct {
	allAnnotations  bool
	allLabels       bool
	annotationNames []string
	labelNames      []string
}

func (di diffIgnores) preprocess(obj *unstructured.Unstructured) {
	if di.allLabels || len(di.labelNames) > 0 {
		labels := obj.GetLabels()
		if labels == nil {
			labels = map[string]string{}
		}
		if di.allLabels {
			labels = map[string]string{}
		} else {
			for _, l := range di.labelNames {
				delete(labels, l)
			}
		}
		obj.SetLabels(labels)
	}
	if di.allAnnotations || len(di.annotationNames) > 0 {
		annotations := obj.GetAnnotations()
		if annotations == nil {
			annotations = map[string]string{}
		}
		if di.allAnnotations {
			annotations = map[string]string{}
		} else {
			for _, l := range di.annotationNames {
				delete(annotations, l)
			}
		}
		obj.SetAnnotations(annotations)
	}
}

type diffStats struct {
	l         sync.Mutex
	Additions []string `json:"additions,omitempty"`
	Changes   []string `json:"changes,omitempty"`
	Deletions []string `json:"deletions,omitempty"`
	SameCount int      `json:"same,omitempty"`
	Errors    []string `json:"errors,omitempty"`
}

func (d *diffStats) added(s string) {
	d.l.Lock()
	defer d.l.Unlock()
	d.Additions = append(d.Additions, s)
}

func (d *diffStats) changed(s string) {
	d.l.Lock()
	defer d.l.Unlock()
	d.Changes = append(d.Changes, s)
}

func (d *diffStats) deleted(s string) {
	d.l.Lock()
	defer d.l.Unlock()
	d.Deletions = append(d.Deletions, s)
}

func (d *diffStats) same(s string) {
	d.l.Lock()
	defer d.l.Unlock()
	d.SameCount++
}

func (d *diffStats) errors(s string) {
	d.l.Lock()
	defer d.l.Unlock()
	d.Errors = append(d.Errors, s)
}

func (d *diffStats) done() {
	sort.Strings(d.Additions)
	sort.Strings(d.Changes)
	sort.Strings(d.Errors)
}

type differ struct {
	w           io.Writer
	client      kubeClient
	opts        diff.Options
	stats       diffStats
	ignores     diffIgnores
	showSecrets bool
	verbose     int
}

func (d *differ) names(ob model.K8sMeta) (name, leftName, rightName string) {
	name = d.client.DisplayName(ob)
	leftName = "live " + name
	rightName = "config " + name
	return
}

type namedUn struct {
	name string
	obj  *unstructured.Unstructured
}

// writeDiff writes the diff between the left and right objects. Either of these
// objects may be nil in which case the supplied object text is diffed against
// a blank string. Care must be taken to ensure that only a single write is made to the writer for every invocation.
// Otherwise output will be interleaved across diffs.
func (d *differ) writeDiff(name string, left, right namedUn) (finalErr error) {
	asYaml := func(obj interface{}) (string, error) {
		b, err := yaml.Marshal(obj)
		if err != nil {
			return "", err
		}
		return string(b), nil
	}
	addLeader := func(s, leader string) string {
		l := fmt.Sprintf("#\n# %s\n#\n", leader)
		return l + s
	}
	defer func() {
		if finalErr != nil {
			d.stats.errors(name)
			sio.Errorf("error diffing %s, %v\n", name, finalErr)
		}
	}()

	fileOpts := d.opts
	fileOpts.LeftName = left.name
	fileOpts.RightName = right.name
	switch {
	case left.obj == nil && right.obj == nil:
		return fmt.Errorf("internal error: both left and right objects were nil for diff")
	case left.obj != nil && right.obj != nil:
		b, err := diff.Objects(left.obj, right.obj, fileOpts)
		if err != nil {
			return err
		}
		if len(b) == 0 {
			if d.verbose > 0 {
				fmt.Fprintf(d.w, "%s unchanged\n", name)
			}
			d.stats.same(name)
		} else {
			fmt.Fprintln(d.w, string(b))
			d.stats.changed(name)
		}
	case left.obj == nil:
		rightContent, err := asYaml(right.obj)
		if err != nil {
			return err
		}
		leaderComment := "object doesn't exist on the server"
		if right.obj.GetName() == "" {
			leaderComment += " (generated name)"
		}
		rightContent = addLeader(rightContent, leaderComment)
		b, err := diff.Strings("", rightContent, fileOpts)
		if err != nil {
			return err
		}
		fmt.Fprintln(d.w, string(b))
		d.stats.added(name)
	default:
		leftContent, err := asYaml(left.obj)
		if err != nil {
			return err
		}
		leftContent = addLeader(leftContent, "object doesn't exist locally")
		b, err := diff.Strings(leftContent, "", fileOpts)
		if err != nil {
			return err
		}
		fmt.Fprintln(d.w, string(b))
		d.stats.deleted(name)
	}
	return nil
}

// diff diffs the supplied object with its remote version and writes output to its writer.
// The local version is found by downcasting the supplied metadata to a local object.
// This cast should succeed for all but the deletion use case.
func (d *differ) diff(ob model.K8sMeta) error {
	name, leftName, rightName := d.names(ob)

	var remoteObject *unstructured.Unstructured
	var err error

	if ob.GetName() != "" {
		remoteObject, err = d.client.Get(ob)
		if err != nil && err != remote.ErrNotFound && err.Error() != "server type not found" { // *sigh*
			d.stats.errors(name)
			sio.Errorf("error fetching %s, %v\n", name, err)
			return err
		}
	}

	fixup := func(u *unstructured.Unstructured) *unstructured.Unstructured {
		if u == nil {
			return u
		}
		if !d.showSecrets {
			u, _ = types.HideSensitiveInfo(u)
		}
		d.ignores.preprocess(u)
		return u
	}

	var left, right *unstructured.Unstructured
	if remoteObject != nil {
		var source string
		left, source = remote.GetPristineVersionForDiff(remoteObject)
		leftName += " (source: " + source + ")"
	}
	left = fixup(left)

	if r, ok := ob.(model.K8sObject); ok {
		right = fixup(r.ToUnstructured())
	}
	return d.writeDiff(name, namedUn{name: leftName, obj: left}, namedUn{name: rightName, obj: right})
}

// diffLocal adapts the diff method to run as a parallel worker.
func (d *differ) diffLocal(ob model.K8sLocalObject) error {
	return d.diff(ob)
}

type diffCommandConfig struct {
	*config
	showDeletions bool
	showSecrets   bool
	parallel      int
	contextLines  int
	di            diffIgnores
	filterFunc    func() (filterParams, error)
	exitNonZero   bool
}

func doDiff(args []string, config diffCommandConfig) error {
	if len(args) != 1 {
		return newUsageError("exactly one environment required")
	}

	env := args[0]
	if env == model.Baseline {
		return newUsageError("cannot diff baseline environment, use a real environment")
	}
	fp, err := config.filterFunc()
	if err != nil {
		return err
	}

	client, err := config.Client(env)
	if err != nil {
		return err
	}

	objects, err := filteredObjects(config.config, env, client.ObjectKey, fp)
	if err != nil {
		return err
	}

	var lister lister = &stubLister{}
	var retainObjects []model.K8sLocalObject
	if config.showDeletions {
		lister, retainObjects, err = startRemoteList(env, config.config, client, fp)
		if err != nil {
			return err
		}
	}

	objects = objsort.Sort(objects, sortConfig(client.IsNamespaced))

	// since the 0 value of context is turned to 3 by the diff library,
	// special case to turn 0 into a negative number so that zero means zero.
	if config.contextLines == 0 {
		config.contextLines = -1
	}
	opts := diff.Options{Context: config.contextLines, Colorize: config.Colorize()}

	w := &lockWriter{Writer: config.Stdout()}
	d := &differ{
		w:           w,
		client:      client,
		opts:        opts,
		ignores:     config.di,
		showSecrets: config.showSecrets,
		verbose:     config.Verbosity(),
	}
	dErr := runInParallel(objects, d.diffLocal, config.parallel)

	var listErr error
	if dErr == nil {
		extra, err := lister.deletions(retainObjects, fp.Includes)
		if err != nil {
			listErr = err
		} else {
			for _, ob := range extra {
				if err := d.diff(ob); err != nil {
					return err
				}
			}
		}
	}

	d.stats.done()
	printStats(d.w, &d.stats)
	numDiffs := len(d.stats.Additions) + len(d.stats.Changes) + len(d.stats.Deletions)

	switch {
	case dErr != nil:
		return dErr
	case listErr != nil:
		return listErr
	case numDiffs > 0:
		if config.exitNonZero {
			return fmt.Errorf("%d object(s) different", numDiffs)
		}
		sio.Noticef("%d object(s) different\n", numDiffs)
		return nil
	default:
		return nil
	}
}

func newDiffCommand(cp configProvider) *cobra.Command {
	cmd := &cobra.Command{
		Use:     "diff <environment>",
		Short:   "diff one or more components against objects in a Kubernetes cluster",
		Example: diffExamples(),
	}

	config := diffCommandConfig{
		filterFunc: addFilterParams(cmd, true),
	}
	cmd.Flags().BoolVar(&config.showDeletions, "show-deletes", true, "include deletions in diff")
	cmd.Flags().IntVar(&config.contextLines, "context", 3, "context lines for diff")
	cmd.Flags().IntVar(&config.parallel, "parallel", 5, "number of parallel routines to run")
	cmd.Flags().BoolVarP(&config.showSecrets, "show-secrets", "S", false, "do not obfuscate secret values in the diff")
	cmd.Flags().BoolVar(&config.di.allAnnotations, "ignore-all-annotations", false, "remove all annotations from objects before diff")
	cmd.Flags().StringArrayVar(&config.di.annotationNames, "ignore-annotation", nil, "remove specific annotation from objects before diff")
	cmd.Flags().BoolVar(&config.di.allLabels, "ignore-all-labels", false, "remove all labels from objects before diff")
	cmd.Flags().StringArrayVar(&config.di.labelNames, "ignore-label", nil, "remove specific label from objects before diff")
	cmd.Flags().BoolVar(&config.exitNonZero, "error-exit", true, "exit with non-zero status code when diffs present")

	cmd.RunE = func(c *cobra.Command, args []string) error {
		config.config = cp()
		return wrapError(doDiff(args, config))
	}
	return cmd
}
