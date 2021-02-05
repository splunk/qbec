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
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type cl struct {
	err    error
	called bool
}

func (c *cl) Close() error {
	c.called = true
	return c.err
}

func TestCleanupSuccess(t *testing.T) {
	defer func() { cleanup = &closers{} }()
	c := &cl{err: nil}
	RegisterCleanupTask(c)
	err := Close()
	require.NoError(t, err)
	assert.True(t, c.called)
}

func TestCleanupError(t *testing.T) {
	defer func() { cleanup = &closers{} }()
	c1 := &cl{err: nil}
	c2 := &cl{err: fmt.Errorf("foobar")}
	c3 := &cl{err: fmt.Errorf("barbaz")}
	c4 := &cl{err: nil}
	RegisterCleanupTask(c1)
	RegisterCleanupTask(c2)
	RegisterCleanupTask(c3)
	RegisterCleanupTask(c4)
	err := Close()
	require.Error(t, err)
	a := assert.New(t)
	a.True(c1.called)
	a.True(c2.called)
	a.True(c3.called)
	a.True(c4.called)
	a.Equal("barbaz", err.Error())
}
