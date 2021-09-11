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

package vm

import (
	"fmt"
	"io"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type errCloser struct {
	err error
}

func newErrCloser(err error) io.Closer {
	return errCloser{err: err}
}

func (e errCloser) Close() error {
	return e.err
}

func TestMultiCloser(t *testing.T) {
	mc := &multiCloser{}
	mc.add(newErrCloser(nil))
	err := mc.Close()
	require.NoError(t, err)

	mc.add(newErrCloser(fmt.Errorf("foo")))
	err = mc.Close()
	require.Error(t, err)
	assert.Equal(t, "foo", err.Error())

	mc.add(newErrCloser(fmt.Errorf("bar")))
	err = mc.Close()
	require.Error(t, err)
	assert.Equal(t, "2 close errors: [foo bar]", err.Error())
}

func TestConfigProviderFromVariables(t *testing.T) {
	vs := VariableSet{}.WithVars(NewVar("foo", "bar"), NewCodeVar("boo", "true"))
	cp := ConfigProviderFromVariables(vs)
	val, err := cp("foo")
	require.NoError(t, err)
	assert.Equal(t, `"bar"`+"\n", val)

	val, err = cp("boo")
	require.NoError(t, err)
	assert.Equal(t, `true`+"\n", val)

	_, err = cp("bar")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "RUNTIME ERROR: Undefined external variable: bar")
}

func TestCreateDataSourcesSuccess(t *testing.T) {
	// note that this test relies on the fact that `Init` on a data source returned by the factory is lazy and does
	// not, in fact, call the Init on the exec data source. Init failures simply cannot happen eagerly in the current implementation.
	_, closer, err := CreateDataSources([]string{"exec://foo?configVar=foo"}, ConfigProviderFromVariables(VariableSet{}))
	require.NoError(t, err)
	require.NotNil(t, closer)
}

func TestCreateDataSourcesError(t *testing.T) {
	_, closer, err := CreateDataSources([]string{"exec://foo"}, ConfigProviderFromVariables(VariableSet{}))
	require.Error(t, err)
	assert.Equal(t, "create data source exec://foo: data source 'exec://foo' must have a configVar param", err.Error())
	require.NotNil(t, closer)
}
