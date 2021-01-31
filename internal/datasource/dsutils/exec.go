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

// Package dsutils provides common utilities for data sources.
package dsutils

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/pkg/errors"
)

// ExecContext is the context for command execution as implemented by exec data sources
type ExecContext struct {
	name       string
	executable string    // the resolved path of the executable to run
	workDir    string    // the work dir to be set for the command
	env        []string  // the environment variables for the command
	stderr     io.Writer // the stderr to be set
	tmpDir     string    // tmpdir created for the exe
	stdin      []byte    // stdin data to be passed to every command invocation
}

// Executable returns the resolved exe name.
func (e *ExecContext) Executable() string {
	return e.executable
}

// BaseCommand returns a command with working dir, stderr, stdin and env set.
func (e *ExecContext) BaseCommand(ctx context.Context, args []string) *exec.Cmd {
	cmd := exec.CommandContext(ctx, e.executable, args...)
	cmd.Dir = e.workDir
	cmd.Stderr = e.stderr
	cmd.Stdin = bytes.NewReader(e.stdin)
	cmd.Env = append(os.Environ(), e.env...)
	return cmd
}

// NewExecContext returns an exec context for the supplied data source name, executable and variables.
func NewExecContext(name string, exe string, vars map[string]interface{}) (*ExecContext, error) {
	self, err := os.Executable()
	if err != nil {
		return nil, errors.Wrap(err, "get self exe")
	}
	wd, err := os.Getwd()
	if err != nil {
		return nil, errors.Wrap(err, "get working directory")
	}
	exe, err = findExe(exe, wd)
	if err != nil {
		return nil, errors.Wrapf(err, "find exe '%s'", exe)
	}
	tmpdir, err := ioutil.TempDir("", "ds*")
	if err != nil {
		return nil, errors.Wrap(err, "create temp dir")
	}
	b, err := json.MarshalIndent(vars, "", "  ")
	if err != nil {
		return nil, errors.Wrap(err, "json marshal")
	}
	return &ExecContext{
		executable: exe,
		workDir:    wd,
		stdin:      b,
		env: []string{
			fmt.Sprintf("DATA_SOURCE_RUNNER=%s", self),
			fmt.Sprintf("DATA_SOURCE_NAME=%s", name),
			fmt.Sprintf("TMPDIR=%s", tmpdir),
		},
		stderr: os.Stderr, // TODO: do something better
		tmpDir: tmpdir,
	}, nil
}

// Close deletes the temporary directory created for command execution.
func (e *ExecContext) Close() error {
	return os.RemoveAll(e.tmpDir)
}

// findExe finds the supplied executable in the file system using exec.LookPath
// except that if a relative path is found for the exe, it first attempts to resolve
// it against the supplied dir. In effect, it behaves as though PATH always has . in the
// first position.
func findExe(exe string, wd string) (string, error) {
	if !filepath.IsAbs(exe) {
		file := filepath.Join(wd, exe)
		if p, err := exec.LookPath(file); err == nil {
			return p, nil
		}
	}
	return exec.LookPath(exe)
}
