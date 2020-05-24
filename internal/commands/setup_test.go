package commands

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestVersionCommand(t *testing.T) {
	s := newScaffold(t)
	defer s.reset()
	err := s.executeCommand("version")
	require.NoError(t, err)
	out := s.stdout()
	a := assert.New(t)
	a.Contains(out, "qbec version: "+version)
	a.Contains(out, "jsonnet version: "+jsonnetVersion)
	a.Contains(out, "client-go version: "+clientGoVersion)
	a.Contains(out, "go version: "+goVersion)
	a.Contains(out, "commit: "+commit)
}

func TestVersionCommandJSON(t *testing.T) {
	s := newScaffold(t)
	defer s.reset()
	err := s.executeCommand("version", "--json")
	require.NoError(t, err)
	var out map[string]string
	err = s.jsonOutput(&out)
	require.NoError(t, err)
	a := assert.New(t)
	a.Equal(version, out["qbec"])
	a.Equal(goVersion, out["go"])
	a.Equal(clientGoVersion, out["client-go"])
	a.Equal(commit, out["commit"])
	a.Equal(jsonnetVersion, out["jsonnet"])
}

func TestOptionsCommand(t *testing.T) {
	s := newScaffold(t)
	defer s.reset()
	err := s.executeCommand("options")
	require.NoError(t, err)
	a := assert.New(t)
	expectedStrings := []string{
		"--app-tag",
		"--colors",
		"--env-file",
		"--eval-concurrency",
		"--force:k8s-context",
		"--k8s:as",
		"--root",
		"--strict-vars",
		"--verbose",
		"--vm:ext-str",
		"--yes",
	}
	for _, str := range expectedStrings {
		a.Contains(s.stdout(), str)
	}
}
