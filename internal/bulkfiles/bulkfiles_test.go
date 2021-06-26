package bulkfiles

import (
	"fmt"
	"io/fs"
	"os"
	"sort"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type proc struct {
	extensions []string
	errorsFor  []string
	seenFiles  []string
}

func (p *proc) Matches(path string, file fs.FileInfo, userSpecified bool) bool {
	if userSpecified {
		return true
	}
	for _, e := range p.extensions {
		if strings.HasSuffix(path, e) {
			return true
		}
	}
	return false
}
func (p *proc) Process(path string, file fs.FileInfo) error {
	p.seenFiles = append(p.seenFiles, path)
	if file.IsDir() {
		return fmt.Errorf("processor got a dir")
	}
	sort.Strings(p.seenFiles)
	for _, e := range p.errorsFor {
		if strings.HasSuffix(path, e) {
			return fmt.Errorf("error processing %s", path)
		}
	}
	return nil
}

func TestProcess(t *testing.T) {
	tests := []struct {
		name            string
		p               *proc
		specified       []string
		continueOnError bool
		assertFn        func(t *testing.T, p *proc, err error)
	}{
		{
			name: "happy",
			p: &proc{
				extensions: []string{".libsonnet", ".jsonnet"},
			},
			specified: []string{"testdata"},
			assertFn: func(t *testing.T, p *proc, err error) {
				require.NoError(t, err)
				assert.Equal(t, 3, len(p.seenFiles))
			},
		},
		{
			name: "happy-specified",
			p: &proc{
				extensions: []string{".libsonnet", ".jsonnet"},
			},
			specified: []string{"testdata", "testdata/0.txt"},
			assertFn: func(t *testing.T, p *proc, err error) {
				require.NoError(t, err)
				assert.Equal(t, 4, len(p.seenFiles))
			},
		},
		{
			name: "errors no continue",
			p: &proc{
				extensions: []string{".libsonnet", ".jsonnet"},
				errorsFor:  []string{"1.jsonnet", "2.jsonnet"},
			},
			specified: []string{"testdata"},
			assertFn: func(t *testing.T, p *proc, err error) {
				require.Error(t, err)
				assert.Equal(t, 1, len(p.seenFiles))
				assert.Contains(t, err.Error(), "error processing")
			},
		},
		{
			name: "errors continue",
			p: &proc{
				extensions: []string{".libsonnet", ".jsonnet"},
				errorsFor:  []string{"1.jsonnet", "2.jsonnet"},
			},
			specified:       []string{"testdata"},
			continueOnError: true,
			assertFn: func(t *testing.T, p *proc, err error) {
				require.Error(t, err)
				assert.Equal(t, 3, len(p.seenFiles))
				assert.Contains(t, err.Error(), "2 errors encountered")
			},
		},
		{
			name: "single error continue",
			p: &proc{
				extensions: []string{".libsonnet", ".jsonnet"},
				errorsFor:  []string{"1.jsonnet"},
			},
			specified:       []string{"testdata"},
			continueOnError: true,
			assertFn: func(t *testing.T, p *proc, err error) {
				require.Error(t, err)
				assert.Equal(t, 3, len(p.seenFiles))
				assert.Contains(t, err.Error(), "1 error encountered")
			},
		},
		{
			name: "bad args",
			p: &proc{
				extensions: []string{".libsonnet", ".jsonnet"},
			},
			specified:       []string{"not-present"},
			continueOnError: true,
			assertFn: func(t *testing.T, p *proc, err error) {
				require.Error(t, err)
				assert.Equal(t, 0, len(p.seenFiles))
				assert.IsType(t, &os.PathError{}, err)
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			err := Process(test.specified, Options{ContinueOnError: test.continueOnError}, test.p)
			test.assertFn(t, test.p, err)
		})
	}
}
