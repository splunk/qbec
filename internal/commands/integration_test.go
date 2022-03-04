// Copyright 2021 Splunk Inc.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

//go:build integration
// +build integration

package commands

import (
	"fmt"
	"regexp"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestIntegrationBasic(t *testing.T) {
	dir := "testdata/projects/simple-service"
	ns, reset := newNamespace(t)
	defer reset()

	t.Run("show", func(t *testing.T) {
		s := newIntegrationScaffold(t, ns, dir)
		defer s.reset()
		err := s.executeCommand("show", "-O", "local")
		require.NoError(t, err)
		s.assertOutputLineMatch(regexp.MustCompile(`^simple-service\s+ConfigMap\s+nginx`))
		s.assertOutputLineMatch(regexp.MustCompile(`^simple-service\s+Service\s+nginx`))
		s.assertOutputLineMatch(regexp.MustCompile(`^simple-service\s+Deployment\s+nginx`))
	})
	t.Run("validate", func(t *testing.T) {
		s := newIntegrationScaffold(t, ns, dir)
		defer s.reset()
		err := s.executeCommand("validate", "local")
		require.NoError(t, err)
		s.assertOutputLineMatch(regexp.MustCompile(`✔ configmaps nginx`))
		s.assertOutputLineMatch(regexp.MustCompile(`✔ deployments nginx`))
		s.assertOutputLineMatch(regexp.MustCompile(`✔ services nginx`))
	})
	allAddsDiffTest := func(t *testing.T) {
		s := newIntegrationScaffold(t, ns, dir)
		defer s.reset()
		err := s.executeCommand("diff", "--error-exit=false", "local")
		require.NoError(t, err)
		stats := s.outputStats()
		a := assert.New(t)
		a.EqualValues(3, len(stats["additions"].([]interface{})))
	}
	t.Run("diff", allAddsDiffTest)
	t.Run("apply dryrun", func(t *testing.T) {
		s := newIntegrationScaffold(t, ns, dir)
		defer s.reset()
		err := s.executeCommand("apply", "-n", "local")
		require.NoError(t, err)
		stats := s.outputStats()
		a := assert.New(t)
		a.EqualValues(3, len(stats["created"].([]interface{})))
	})
	// ensure dryrun did not change state above
	t.Run("diff1", allAddsDiffTest)
	t.Run("apply", func(t *testing.T) {
		s := newIntegrationScaffold(t, ns, dir)
		defer s.reset()
		err := s.executeCommand("apply", "local", "--wait")
		require.NoError(t, err)
		stats := s.outputStats()
		a := assert.New(t)
		a.EqualValues(3, len(stats["created"].([]interface{})))
	})
	t.Run("diff2", func(t *testing.T) {
		s := newIntegrationScaffold(t, ns, dir)
		defer s.reset()
		err := s.executeCommand("diff", "--error-exit=false", "local")
		require.NoError(t, err)
		stats := s.outputStats()
		a := assert.New(t)
		a.EqualValues(3, stats["same"])
	})

	changeArgs := []string{"--vm:ext-code=replicas=2", "--vm:ext-str=cmContent=goodbye world"}
	t.Run("diff3", func(t *testing.T) {
		s := newIntegrationScaffold(t, ns, dir)
		defer s.reset()
		err := s.executeCommand(append(changeArgs, "diff", "--error-exit=false", "local")...)
		require.NoError(t, err)
		stats := s.outputStats()
		a := assert.New(t)
		a.EqualValues(1, stats["same"])
		a.EqualValues(2, len(stats["changes"].([]interface{})))
		s.assertOutputLineMatch(regexp.MustCompile(`-\s+replicas: 1$`))
		s.assertOutputLineMatch(regexp.MustCompile(`\+\s+replicas: 2$`))
		s.assertOutputLineMatch(regexp.MustCompile(`-\s+index.html: hello world$`))
		s.assertOutputLineMatch(regexp.MustCompile(`\+\s+index.html: goodbye world$`))
	})
	t.Run("apply2", func(t *testing.T) {
		s := newIntegrationScaffold(t, ns, dir)
		defer s.reset()
		err := s.executeCommand(append(changeArgs, "apply", "local", "--wait")...)
		require.NoError(t, err)
		stats := s.outputStats()
		a := assert.New(t)
		a.EqualValues(1, stats["same"])
		a.EqualValues(2, len(stats["updated"].([]interface{})))
	})
	t.Run("apply3", func(t *testing.T) {
		s := newIntegrationScaffold(t, ns, dir)
		defer s.reset()
		err := s.executeCommand(append(changeArgs, "apply", "local", "--wait", "--wait-all")...)
		require.NoError(t, err)
		stats := s.outputStats()
		a := assert.New(t)
		a.EqualValues(3, stats["same"])
		s.assertErrorLineMatch(regexp.MustCompile(`waiting for readiness of 1 objects`))
	})
}

func TestIntegrationLazyCustomResources(t *testing.T) {
	dir := "testdata/projects/lazy-resources"
	ns, reset := newNamespace(t)
	defer reset()

	extra := []string{"--vm:ext-str=suffix=" + fmt.Sprint(time.Now().Unix())}
	var err1, err2 error
	done := make(chan struct{}, 1)
	var wg sync.WaitGroup
	wg.Add(2)

	s1 := newIntegrationScaffold(t, ns, dir)
	defer s1.reset()
	defer s1.executeCommand(append(extra, "delete", "local")...)

	s2 := s1.sub()
	defer s2.reset()

	go func() {
		defer func() { close(done) }()
		err1 = s2.executeCommand(append(extra, "apply", "-C", "crds", "local")...)
		require.NoError(t, err1)
	}()
	time.Sleep(2 * time.Second)
	err2 = s1.executeCommand(append(extra, "apply", "-k", "customresourcedefinitions", "local")...)
	require.NoError(t, err2)
	<-done
}

func TestIntegrationWait(t *testing.T) {
	dir := "testdata/projects/wait"
	ns, reset := newNamespace(t)
	defer reset()

	t.Run("apply", func(t *testing.T) {
		s := newIntegrationScaffold(t, ns, dir)
		defer s.reset()
		err := s.executeCommand("apply", "local", "--wait-all")
		require.NoError(t, err)
		stats := s.outputStats()
		a := assert.New(t)
		a.EqualValues(1, len(stats["created"].([]interface{})))
		s.assertErrorLineMatch(regexp.MustCompile(`waiting for readiness of 1 objects`))
	})
	t.Run("apply-no-wait", func(t *testing.T) {
		s := newIntegrationScaffold(t, ns, dir)
		defer s.reset()
		err := s.executeCommand("apply", "local", "--wait-all", "--vm:ext-code=wait=false")
		require.NoError(t, err)
		stats := s.outputStats()
		a := assert.New(t)
		a.EqualValues(1, len(stats["updated"].([]interface{})))
		s.assertErrorLineMatch(regexp.MustCompile(`waiting for readiness of 0 objects`))
		s.assertErrorLineMatch(regexp.MustCompile(`: wait disabled by policy`))
	})
}

func TestIntegrationDiffPolicies(t *testing.T) {
	dir := "testdata/projects/policies"
	ns, reset := newNamespace(t)
	defer reset()

	t.Run("apply", func(t *testing.T) {
		s := newIntegrationScaffold(t, ns, dir)
		defer s.reset()
		err := s.executeCommand("apply", "local")
		require.NoError(t, err)
		stats := s.outputStats()
		a := assert.New(t)
		a.EqualValues(1, len(stats["updated"].([]interface{})))
		a.EqualValues(2, len(stats["created"].([]interface{})))
	})
	t.Run("diff", func(t *testing.T) {
		s := newIntegrationScaffold(t, ns, dir)
		defer s.reset()
		err := s.executeCommand("diff", "local", "--vm:ext-code=hide=true", "--vm:ext-str=foo=xxx --error-exit=false")
		require.NoError(t, err)
		stats := s.outputStats()
		sk := stats["skipped"]
		skipped, ok := sk.(map[string]interface{})
		require.True(t, ok)
		a := assert.New(t)
		a.EqualValues(1, len(skipped["updates"].([]interface{})))
		a.EqualValues(1, len(skipped["deletions"].([]interface{})))
	})
}

func TestIntegrationNamespaceFilters(t *testing.T) {
	dir := "testdata/projects/multi-ns"

	t.Run("show-first", func(t *testing.T) {
		s := newIntegrationScaffold(t, "", dir)
		defer s.reset()
		err := s.executeCommand("show", "-O", "-p", "first", "local")
		require.NoError(t, err)
		s.assertOutputLineMatch(regexp.MustCompile(`^first\s+ConfigMap\s+first-cm`))
		s.assertOutputLineNoMatch(regexp.MustCompile(`second`))
	})

	t.Run("show-second-and-cluster", func(t *testing.T) {
		s := newIntegrationScaffold(t, "", dir)
		defer s.reset()
		err := s.executeCommand("show", "-O", "-p", "second", "--include-cluster-objects", "local")
		require.NoError(t, err)
		s.assertOutputLineMatch(regexp.MustCompile(`^first\s+Namespace\s+first`))
		s.assertOutputLineMatch(regexp.MustCompile(`^second\s+Namespace\s+second`))
		s.assertOutputLineNoMatch(regexp.MustCompile(`^first\s+ConfigMap`))
		s.assertOutputLineMatch(regexp.MustCompile(`^second\s+ConfigMap\s+second-cm`))
		s.assertOutputLineMatch(regexp.MustCompile(`^second\s+Secret\s+second-secret`))
	})

	t.Run("only-cluster-filter", func(t *testing.T) {
		s := newIntegrationScaffold(t, "", dir)
		defer s.reset()
		err := s.executeCommand("show", "-O", "--include-cluster-objects=false", "local")
		require.NoError(t, err)
		s.assertOutputLineNoMatch(regexp.MustCompile(`Namespace`))
		s.assertOutputLineMatch(regexp.MustCompile(`^first\s+ConfigMap\s+first-cm`))
		s.assertOutputLineMatch(regexp.MustCompile(`^second\s+ConfigMap\s+second-cm`))
		s.assertOutputLineMatch(regexp.MustCompile(`^second\s+Secret\s+second-secret`))
	})

}
