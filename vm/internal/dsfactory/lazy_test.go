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

package dsfactory

import (
	"fmt"
	"testing"

	"github.com/splunk/qbec/vm/datasource"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type delegate struct {
	initErr      error
	resolveError error
	closeError   error
}

func (d *delegate) Init(_ datasource.ConfigProvider) error {
	return d.initErr
}

func (d *delegate) Name() string {
	return "me"
}

func (d *delegate) Resolve(path string) (string, error) {
	return fmt.Sprintf("resolved:%s", path), d.resolveError
}

func (d *delegate) Close() error {
	return d.closeError
}

var _ datasource.WithLifecycle = &delegate{}

var cp = func(name string) (string, error) {
	return "", nil
}

func TestLazySuccess(t *testing.T) {
	d := &delegate{}
	l := makeLazy(d)
	assert.Equal(t, "me", l.Name())
	err := l.Init(cp)
	require.NoError(t, err)
	x, err := l.Resolve("/foo")
	require.NoError(t, err)
	assert.Equal(t, "resolved:/foo", x)
}

func TestLazyPropagatesInitError(t *testing.T) {
	d := &delegate{initErr: fmt.Errorf("init error")}
	l := makeLazy(d)
	err := l.Init(cp)
	require.NoError(t, err)
	_, err = l.Resolve("/foo")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "init error")
}

func TestLazyPropagatesResolveError(t *testing.T) {
	d := &delegate{resolveError: fmt.Errorf("resolve error")}
	l := makeLazy(d)
	err := l.Init(cp)
	require.NoError(t, err)
	_, err = l.Resolve("/foo")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "resolve error")
}

func TestLazyPropagatesCloseError(t *testing.T) {
	d := &delegate{closeError: fmt.Errorf("close error")}
	l := makeLazy(d)
	err := l.Init(cp)
	require.NoError(t, err)
	_, err = l.Resolve("/foo")
	require.NoError(t, err)
	err = l.Close()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "close error")
}
