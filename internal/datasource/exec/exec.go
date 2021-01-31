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

package exec

import (
	"bytes"
	"context"
	"fmt"
	"net/url"
	"time"

	"github.com/pkg/errors"
	"github.com/splunk/qbec/internal/datasource/api"
	"github.com/splunk/qbec/internal/datasource/dsutils"
)

// Scheme is scheme supported by this data source
const (
	Scheme = "exec"
)

// Config is the configuration of the data source.
type Config struct {
	Executable string        // the executable that is run, which should implement an HTTP server over localhost
	Args       []string      // static arguments to be passed to the executable
	Timeout    time.Duration // timeout for the server to be ready
}

func (c *Config) assertValid() error {
	if c.Executable == "" {
		return fmt.Errorf("executable not specified")
	}
	return nil
}

func (c *Config) initDefaults() {
	if c.Timeout == 0 {
		c.Timeout = time.Minute
	}
}

type execSource struct {
	name   string
	config Config
	ec     *dsutils.ExecContext
}

const (
	paramExe     = "exe"
	paramArg     = "arg"
	paramTimeout = "timeout"
)

func parseDuration(s string) (time.Duration, error) {
	if s == "" {
		return 0, nil
	}
	return time.ParseDuration(s)
}

// FromURL returns an exec data source from the supplied URL. The URL must be of the form:
//     exec://ds-name?exe=path/to/executable[&timeout=10s]
func FromURL(u string) (api.DataSource, error) {
	parsed, err := url.Parse(u)
	if err != nil {
		return nil, err
	}
	if parsed.Scheme != Scheme {
		return nil, fmt.Errorf("unsupported scheme: want '%s' got '%s'", Scheme, parsed.Scheme)
	}
	name := parsed.Hostname()
	q := parsed.Query()
	exe := q.Get(paramExe)
	if exe == "" {
		return nil, fmt.Errorf("URL '%s' must have a '%s' parameter", u, paramExe)
	}
	args := q[paramArg]
	cfg := Config{
		Executable: exe,
		Args:       args,
	}
	cfg.Timeout, err = parseDuration(q.Get(paramTimeout))
	if err != nil {
		return nil, errors.Wrapf(err, "invalid param %s for %s", paramTimeout, u)
	}
	return New(name, cfg)
}

// New creates an exec datasource with the given name and configuration.
func New(name string, config Config) (api.DataSource, error) {
	if err := config.assertValid(); err != nil {
		return nil, errors.Wrapf(err, "data source %s", name)
	}
	config.initDefaults()
	return &execSource{
		name:   name,
		config: config,
	}, nil
}

// Name implements the interface method
func (d *execSource) Name() string {
	return d.name
}

// Start implements the interface method.
func (d *execSource) Start(vars map[string]interface{}) error {
	ec, err := dsutils.NewExecContext(d.name, d.config.Executable, vars)
	if err != nil {
		return errors.Wrap(err, "get execution context")
	}
	d.ec = ec
	return nil
}

// Resolve implements the interface method.
func (d *execSource) Resolve(path string) (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), d.config.Timeout)
	defer cancel()

	cmd := d.ec.BaseCommand(ctx, d.config.Args)
	cmd.Env = append(cmd.Env, fmt.Sprintf("DATA_SOURCE_PATH=%s", path))
	var capture bytes.Buffer
	cmd.Stdout = &capture

	if err := cmd.Run(); err != nil {
		return "", errors.Wrapf(err, "run '%s'", d.ec.Executable())
	}
	return capture.String(), nil
}

// Close implements the interface method.
func (d *execSource) Close() error {
	return d.ec.Close()
}
