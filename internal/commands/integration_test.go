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
}

func TestIntegrationLazyCustomResources(t *testing.T) {
	dir := "testdata/projects/lazy-resources"
	ns, reset := newNamespace(t)
	defer reset()

	extra := []string{"--vm:ext-str=suffix=" + fmt.Sprint(time.Now().Unix())}
	var err1, err2 error
	var wg sync.WaitGroup
	wg.Add(2)

	s1 := newIntegrationScaffold(t, ns, dir)
	defer s1.reset()
	go func() {
		defer wg.Done()
		err1 = s1.executeCommand(append(extra, "apply", "-C", "crds", "local")...)
		require.NoError(t, err1)
	}()
	go func() {
		defer wg.Done()
		time.Sleep(5 * time.Second)
		s2 := s1.sub()
		defer s2.reset()
		err2 = s2.executeCommand(append(extra, "apply", "-c", "crds", "local")...)
		require.NoError(t, err2)
	}()
	wg.Wait()
	_ = s1.executeCommand(append(extra, "delete", "local")...)
}
