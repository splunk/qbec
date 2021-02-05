package cmd

import (
	"bytes"
	"io/ioutil"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestProfileSuccess(t *testing.T) {
	defer func() { cleanup = &closers{} }()
	tmpDir, err := ioutil.TempDir("", "")
	require.NoError(t, err)
	defer os.RemoveAll(tmpDir)
	cpuFile := filepath.Join(tmpDir, "cpu.prof")
	memFile := filepath.Join(tmpDir, "mem.prof")
	_ = getContext(t, Options{}, []string{
		"--pprof:cpu=" + cpuFile,
		"--pprof:memory=" + memFile,
	})
	// run some cycles create some garbage
	for i := 0; i < 10000; i++ {
		_ = bytes.NewBuffer(nil)
	}
	err = Close()
	require.NoError(t, err)
	s, err := os.Stat(cpuFile)
	require.NoError(t, err)
	assert.True(t, s.Size() > 0)
	s, err = os.Stat(memFile)
	require.NoError(t, err)
	assert.True(t, s.Size() > 0)
}

func TestProfileInitFail(t *testing.T) {
	p := &profiler{cpuProfile: "non-existent-path/foo.prof"}
	err := p.init()
	require.Error(t, err)
}

func TestProfileInitFail2(t *testing.T) {
	p := &profiler{memoryProfile: "non-existent-path/foo.prof"}
	err := p.init()
	require.Error(t, err)
}
