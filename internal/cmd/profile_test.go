/*
   Copyright 2021 Splunk Inc.

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

package cmd

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestProfileSuccess(t *testing.T) {
	defer func() { cleanup = &closers{} }()
	tmpDir, err := os.MkdirTemp("", "")
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
