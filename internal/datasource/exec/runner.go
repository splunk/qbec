package exec

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
