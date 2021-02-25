package filematcher_test

import (
	"fmt"
	"path/filepath"
	"testing"

	"github.com/splunk/qbec/internal/filematcher"
	"github.com/stretchr/testify/assert"
)

func TestMatch(t *testing.T) {
	cwd, err := filepath.Abs(".")
	assert.Nil(t, err)
	var tests = []struct {
		pattern       string
		expectedMatch bool
		expectedFiles []string
	}{
		{"testdata/*/env.yaml", false, nil},
		{"testdata/env.yaml", true, []string{filepath.Join(cwd, "testdata/env.yaml")}},
		{"testdata/non-existentenv.yaml", false, nil},
		{"testdata/*.yaml", true, []string{filepath.Join(cwd, "testdata/.env.yaml"), filepath.Join(cwd, "testdata/1env.yaml"), filepath.Join(cwd, "testdata/env.yaml"), filepath.Join(cwd, "testdata/env1.yaml")}},
		{"testdata", true, []string{filepath.Join(cwd, "testdata")}},
		{"testdata/testDirForGlobPatterns/*", true, []string{filepath.Join(cwd, "testdata/testDirForGlobPatterns/.keep"), filepath.Join(cwd, "testdata/testDirForGlobPatterns/childDir")}},
	}
	for i, test := range tests {
		t.Run(fmt.Sprintf("%d", i), func(t *testing.T) {
			files, err := filematcher.Match(test.pattern)
			assert.Equal(t, test.expectedMatch, err == nil)
			assert.Equal(t, test.expectedFiles, files)
		})
	}

}
