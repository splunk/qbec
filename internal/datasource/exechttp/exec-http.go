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

package exechttp

import (
	"fmt"
	"net/url"
	"time"

	"github.com/phayes/freeport"
	"github.com/pkg/errors"
	"github.com/splunk/qbec/internal/datasource/api"
	"github.com/splunk/qbec/internal/datasource/dsutils"
)

// Scheme is scheme supported by this data source
const (
	Scheme = "exec-http"
)

// Config is the configuration of the data source.
type Config struct {
	Executable     string        // the executable that is run, which should implement an HTTP server over localhost
	Args           []string      // static arguments to the passed to the executable
	PingPath       string        // the path that should return a 200 to indicate that the server is ready to process requests
	ConnectTimeout time.Duration // timeout for connection requests to the server
	InitTimeout    time.Duration // timeout for the server to be ready
	RequestTimeout time.Duration // timeout for regular requests
}

func (c *Config) assertValid() error {
	if c.Executable == "" {
		return fmt.Errorf("executable not specified")
	}
	return nil
}

func (c *Config) initDefaults() {
	if c.PingPath == "" {
		c.PingPath = "/ping"
	}
	if c.ConnectTimeout == 0 {
		c.ConnectTimeout = 100 * time.Millisecond
	}
	if c.InitTimeout == 0 {
		c.InitTimeout = 10 * time.Second
	}
	if c.RequestTimeout == 0 {
		c.RequestTimeout = time.Second
	}
}

type execHTTPSource struct {
	name   string
	config Config
	run    *run
}

const (
	paramExe            = "exe"
	paramArg            = "arg"
	paramPingPath       = "ping-path"
	paramConnectTimeout = "connect-timeout"
	paramInitTimeout    = "init-timeout"
	paramRequestTimeout = "request-timeout"
)

func parseDuration(s string) (time.Duration, error) {
	if s == "" {
		return 0, nil
	}
	return time.ParseDuration(s)
}

// FromURL returns an exec HTTP data source from the supplied URL. The URL must be of the form:
//     exec-http://ds-name?exe=path/to/executable[&connect-timeout=100ms][&init-timeout=1s][&request-timeout=5s]
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
	cfg.PingPath = q.Get(paramPingPath)
	cfg.ConnectTimeout, err = parseDuration(q.Get(paramConnectTimeout))
	if err != nil {
		return nil, errors.Wrapf(err, "invalid param %s for %s", paramConnectTimeout, u)
	}
	cfg.InitTimeout, err = parseDuration(q.Get(paramInitTimeout))
	if err != nil {
		return nil, errors.Wrapf(err, "invalid param %s for %s", paramInitTimeout, u)
	}
	cfg.RequestTimeout, err = parseDuration(q.Get(paramRequestTimeout))
	if err != nil {
		return nil, errors.Wrapf(err, "invalid param %s for %s", paramRequestTimeout, u)
	}
	return New(name, cfg)
}

// New creates an exec-http datasource with the given name and configuration.
func New(name string, config Config) (api.DataSource, error) {
	if err := config.assertValid(); err != nil {
		return nil, errors.Wrapf(err, "data source %s", name)
	}
	config.initDefaults()
	return &execHTTPSource{
		name:   name,
		config: config,
	}, nil
}

// Name implements the interface method
func (d *execHTTPSource) Name() string {
	return d.name
}

// Start implements the interface method.
func (d *execHTTPSource) Start(vars map[string]interface{}) error {
	ec, err := dsutils.NewExecContext(d.name, d.config.Executable, vars)
	if err != nil {
		return errors.Wrap(err, "create execution context")
	}
	port, err := freeport.GetFreePort()
	if err != nil {
		return errors.Wrap(err, "get free port")
	}
	d.run = &run{
		ec:     ec,
		port:   port,
		config: d.config,
	}
	if err := d.run.start(); err != nil {
		return errors.Wrap(err, "start data source")
	}
	return nil
}

// Resolve implements the interface method.
func (d *execHTTPSource) Resolve(path string) (string, error) {
	return d.run.resolve(path)
}

// Close implements the interface method.
func (d *execHTTPSource) Close() error {
	// if Start hasn't been called, noop
	if d.run == nil {
		return nil
	}
	return d.run.stop()
}
