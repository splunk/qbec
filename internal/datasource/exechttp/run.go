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
	"context"
	"fmt"
	"io/ioutil"
	"net"
	"net/http"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/pkg/errors"
	"github.com/splunk/qbec/internal/datasource/dsutils"
)

const bodyErrTruncate = 256

type run struct {
	ec     *dsutils.ExecContext
	port   int
	config Config
	cmd    *exec.Cmd
	client *http.Client
}

func (r *run) url(path string) string {
	if !strings.HasPrefix(path, "/") {
		path = "/" + path
	}
	return fmt.Sprintf("http://127.0.0.1:%d%s", r.port, path)
}

func (r *run) start() error {
	cmd := r.ec.BaseCommand(context.Background(), r.config.Args)
	cmd.Env = append(cmd.Env, fmt.Sprintf("DATA_SOURCE_PORT=%d", r.port)) // TODO: do ADDR instead to allow for Unix domain sockets in the future
	// TODO: something better than this
	cmd.Stdout = os.Stderr
	setCommandAttrs(cmd)
	if err := cmd.Start(); err != nil {
		return errors.Wrapf(err, "start executable %s", r.ec.Executable())
	}
	r.cmd = cmd
	dialer := net.Dialer{
		Timeout: r.config.ConnectTimeout,
	}
	r.client = &http.Client{
		Timeout: r.config.RequestTimeout,
		Transport: &http.Transport{
			DialContext: dialer.DialContext,
		},
	}
	return r.waitForReady()
}

func (r *run) waitForReady() error {
	endTime := time.Now().Add(r.config.InitTimeout)
	for {
		code, _, err := r.doHTTP(r.config.PingPath)
		if err == nil && code == http.StatusOK {
			return nil
		}
		time.Sleep(200 * time.Millisecond)
		if time.Now().After(endTime) {
			return fmt.Errorf("init timeout %v", r.config.InitTimeout)
		}
	}
}

func (r *run) doHTTP(path string) (statusCode int, body []byte, err error) {
	res, err := r.client.Get(r.url(path))
	if err != nil {
		return 0, nil, err
	}
	defer res.Body.Close()
	b, err := ioutil.ReadAll(res.Body)
	if err != nil {
		return 0, nil, errors.Wrap(err, "body read")
	}
	return res.StatusCode, b, nil
}

func (r *run) resolve(path string) (string, error) {
	statusCode, body, err := r.doHTTP(path)
	if err != nil {
		return "", err
	}
	if statusCode != http.StatusOK {
		more := ""
		if len(body) > bodyErrTruncate {
			body = body[:bodyErrTruncate]
			more = " ..."
		}
		return "", fmt.Errorf("GET %s returned %d (body=%s%s)", path, statusCode, body, more)
	}
	return string(body), nil
}

func (r *run) stop() error {
	var err error
	if r.cmd != nil {
		err = stopCommand(r.cmd)
	}
	_ = r.ec.Close()
	return err
}
