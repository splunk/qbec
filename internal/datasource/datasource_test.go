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
