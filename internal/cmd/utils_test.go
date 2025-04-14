// Copyright 2025 Splunk Inc.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package cmd

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/spf13/cobra"
	"github.com/stretchr/testify/require"
)

func setPwd(t *testing.T, dir string) func() {
	wd, err := os.Getwd()
	require.NoError(t, err)
	p, err := filepath.Abs(dir)
	require.NoError(t, err)
	err = os.Chdir(p)
	require.NoError(t, err)
	return func() {
		err = os.Chdir(wd)
		require.NoError(t, err)
	}
}

func getContext(t *testing.T, opts Options, args []string) Context {
	var ctx Context
	var ctxMaker func() (Context, error)
	root := &cobra.Command{
		Use:   "qbec-test",
		Short: "qbec test tool",
		RunE: func(c *cobra.Command, args []string) error {
			var err error
			ctx, err = ctxMaker()
			return err
		},
	}
	ctxMaker = NewContext(root, opts)
	root.SetArgs(args)
	err := root.Execute()
	require.NoError(t, err)
	return ctx
}

func getBadContext(t *testing.T, opts Options, args []string) error {
	var ctxMaker func() (Context, error)
	root := &cobra.Command{
		Use:   "qbec-test",
		Short: "qbec test tool",
		RunE: func(c *cobra.Command, args []string) error {
			_, err := ctxMaker()
			return err
		},
	}
	ctxMaker = NewContext(root, opts)
	root.SetArgs(args)
	err := root.Execute()
	require.Error(t, err)
	return err
}
