package commands

import (
	"runtime"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLintCompressLines(t *testing.T) {
	tests := []struct {
		name   string
		input  string
		output []string
	}{
		{
			name:   "no empty",
			input:  "a\nb\nc\n",
			output: []string{"a", "b", "c", ""},
		},
		{
			name:   "emptiness",
			input:  "\n\na\n\n\n\nb\nc\n\n\n\n",
			output: []string{"", "a", "", "b", "c", ""},
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			actual := compressLines(test.input)
			assert.EqualValues(t, test.output, actual)
		})
	}
}

func TestLintBasic(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("import of a file called _.libsonnet doesn't seem to work in the linter in windows")
	}
	s := newScaffold(t)
	defer s.reset()
	err := s.executeCommand("alpha", "lint")
	require.NoError(t, err)
}

func TestLintWithApp(t *testing.T) {
	s := newCustomScaffold(t, "testdata/projects/lint-app")
	defer s.reset()
	err := s.executeCommand("alpha", "lint", "components", "lib")
	require.Error(t, err)
	assert.Equal(t, "3 errors encountered", err.Error())
}

func TestLintWithoutApp(t *testing.T) {
	s := newCustomScaffold(t, "testdata/projects/lint-app")
	defer s.reset()
	err := s.executeCommand("alpha", "lint", "lib/bar.libsonnet", "--app=false")
	require.Error(t, err)
	assert.Equal(t, "1 error encountered", err.Error())
}
