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

package dsexec

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
)

type runner struct {
	c *Config
}

func newRunner(c *Config) *runner {
	return &runner{c: c}
}

func (r *runner) runWithEnv(e map[string]string) (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), r.c.timeout)
	defer cancel()

	cmd := exec.CommandContext(ctx, r.c.Command, r.c.Args...)
	var env []string
	if r.c.InheritEnv {
		env = os.Environ()
	}
	for k, v := range r.c.Env {
		env = append(env, fmt.Sprintf("%s=%s", k, v))
	}
	for k, v := range e {
		env = append(env, fmt.Sprintf("%s=%s", k, v))
	}
	cmd.Env = env

	var capture bytes.Buffer
	cmd.Stdin = bytes.NewReader([]byte(r.c.Stdin))
	cmd.Stdout = &capture
	cmd.Stderr = os.Stderr

	if err := cmd.Run(); err != nil {
		return "", err
	}
	return capture.String(), nil
}

func (r *runner) close() error {
	return nil
}
