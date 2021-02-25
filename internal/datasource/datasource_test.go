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

package datasource

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDataSourceSuccess(t *testing.T) {
	ds, err := Create("exec://foo?configVar=bar")
	require.NoError(t, err)
	assert.IsType(t, &lazySource{}, ds)
}

func TestNegativeCases(t *testing.T) {
	tests := []struct {
		name string
		uri  string
		msg  string
	}{
		{
			name: "bad-url",
			uri:  "exec://bar\x00?configVar=test",
			msg:  "parse URL",
		},
		{
			name: "bad-kind",
			uri:  "exec-foo://bar?configVar=test",
			msg:  "unsupported scheme 'exec-foo",
		},
		{
			name: "no-name",
			uri:  "exec://?configVar=test",
			msg:  "does not have a name",
		},
		{
			name: "forgot-slash",
			uri:  "exec:foobar?configVar=test",
			msg:  "did you forget the '//' after the ':'",
		},
		{
			name: "no-cfg-var",
			uri:  "exec://foo?config=test",
			msg:  "must have a configVar param",
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			_, err := Create(test.uri)
			require.Error(t, err)
			assert.Contains(t, err.Error(), test.msg)
		})
	}
}
