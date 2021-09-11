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

// Package exec provides a data source implementation that can execute external commands and return
// its standard output for import or importstr use.
package exec

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"time"

	"github.com/pkg/errors"
	"github.com/splunk/qbec/vm/datasource"
	"github.com/splunk/qbec/vm/internal/ds"
)

// Scheme is scheme supported by this data source
const (
	Scheme = "exec"
)

// Config is the configuration of the data source.
type Config struct {
	Command    string            `json:"command"`              // the executable that is run
	Args       []string          `json:"args,omitempty"`       // arguments to be passed to the command
	Env        map[string]string `json:"env,omitempty"`        // environment for the command
	Stdin      string            `json:"stdin,omitempty"`      // standard input to pass to the command
	Timeout    string            `json:"timeout,omitempty"`    // command timeout as a duration string
	InheritEnv bool              `json:"inheritEnv,omitempty"` // Inherit env from the parent(qbec) process

	timeout time.Duration // internal representation
}

func findExecutable(cmd string) (string, error) {
	if !filepath.IsAbs(cmd) {
		p, err := filepath.Abs(cmd)
		if err == nil {
			stat, err := os.Stat(cmd)
			if err == nil {
				if m := stat.Mode(); !m.IsDir() && m&0111 != 0 {
					return p, nil
				}
			}
		}
	}
	return exec.LookPath(cmd)
}

func (c *Config) assertValid() error {
	if c.Command == "" {
		return fmt.Errorf("command not specified")
	}
	if c.Timeout != "" {
		t, err := time.ParseDuration(c.Timeout)
		if err != nil {
			return fmt.Errorf("invalid timeout '%s': %v", c.Timeout, err)
		}
		c.timeout = t
	}
	exe, err := findExecutable(c.Command)
	if err != nil {
		return fmt.Errorf("invalid command '%s': %v", c.Command, err)
	}
	c.Command = exe
	return nil
}

func (c *Config) initDefaults() {
	if c.timeout == 0 {
		c.timeout = time.Minute
	}
}

type execSource struct {
	name      string
	configVar string
	runner    *runner
}

// New creates a new exec data source
func New(name string, configVar string) ds.DataSourceWithLifecycle {
	return &execSource{
		name:      name,
		configVar: configVar,
	}
}

// Name implements the interface method
func (d *execSource) Name() string {
	return d.name
}

// Init implements the interface method.
func (d *execSource) Init(p datasource.ConfigProvider) (fErr error) {
	defer func() {
		fErr = errors.Wrapf(fErr, "init data source %s", d.name) // nil wraps as nil
	}()
	cfgJSON, err := p(d.configVar)
	if err != nil {
		return err
	}
	var c Config
	err = json.Unmarshal([]byte(cfgJSON), &c)
	if err != nil {
		return err
	}
	err = c.assertValid()
	if err != nil {
		return err
	}
	c.initDefaults()
	d.runner = newRunner(&c)
	return nil
}

// Resolve implements the interface method.
func (d *execSource) Resolve(path string) (string, error) {
	return d.runner.runWithEnv(map[string]string{
		"__DS_NAME__": d.name,
		"__DS_PATH__": path,
	})
}

// Close implements the interface method.
func (d *execSource) Close() error {
	return d.runner.close()
}
