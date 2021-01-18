package exechttp

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestExecBasic(t *testing.T) {
	a := assert.New(t)
	ds, err := FromURL("exec-http://replay?exe=go&arg=run&arg=./testdata/replay/main.go")
	require.NoError(t, err)
	defer ds.Close()
	a.Equal("replay", ds.Name())
	err = ds.Start(map[string]interface{}{
		"extFoo": "extBar",
	})
	require.NoError(t, err)
	str, err := ds.Resolve("/foo/bar")
	require.NoError(t, err)
	var data struct {
		Env   map[string]string `json:"ENV"`
		Input map[string]string `json:"STDIN"`
	}
	err = json.Unmarshal([]byte(str), &data)
	require.NoError(t, err)
	a.Equal("replay", data.Env["DATA_SOURCE_NAME"])
	a.Equal("/foo/bar", data.Env["DATA_SOURCE_PATH"])
	a.EqualValues(map[string]string{"extFoo": "extBar"}, data.Input)
}

func TestExecNegative(t *testing.T) {
	tests := []struct {
		name          string
		url           string
		path          string
		startAsserter func(t *testing.T, err error)
		asserter      func(t *testing.T, resolved string, err error)
	}{
		{
			name: "bad-exe",
			url:  "exec-http://userdb?exe=non-existent",
			startAsserter: func(t *testing.T, err error) {
				require.Error(t, err)
				assert.Contains(t, err.Error(), `exec: "non-existent"`)
			},
		},
		{
			name: "exe-err",
			url:  "exec-http://replay?exe=go&arg=run&arg=./testdata/replay/main.go",
			path: "/fail",
			asserter: func(t *testing.T, resolved string, err error) {
				require.Error(t, err)
				assert.Contains(t, err.Error(), "GET /fail returned 400 (body=no data for you\n)")
			},
		},
		{
			name: "exe-slow",
			url:  "exec-http://replay?exe=go&arg=run&arg=./testdata/replay/main.go&request-timeout=100ms",
			path: "/slow",
			asserter: func(t *testing.T, resolved string, err error) {
				require.Error(t, err)
				assert.Contains(t, err.Error(), `context deadline exceeded`)
			},
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			ds, err := FromURL(test.url)
			require.NoError(t, err)
			err = ds.Start(map[string]interface{}{})
			if test.startAsserter == nil {
				require.NoError(t, err)
			} else {
				test.startAsserter(t, err)
			}
			if err != nil {
				return
			}
			defer ds.Close()
			p := test.path
			if p == "" {
				p = "/"
			}
			ret, err := ds.Resolve(p)
			if test.asserter != nil {
				test.asserter(t, ret, err)
			}
		})
	}
}

func TestExecParseErrors(t *testing.T) {
	tests := []struct {
		name     string
		url      string
		asserter func(t *testing.T, err error)
	}{
		{
			name: "bad-scheme",
			url:  "exec-foo://listing?exe=ls",
			asserter: func(t *testing.T, err error) {
				require.Error(t, err)
				assert.Contains(t, err.Error(), `unsupported scheme: want 'exec-http' got 'exec-foo'`)
			},
		},
		{
			name: "no-exe",
			url:  "exec-http://userdb",
			asserter: func(t *testing.T, err error) {
				require.Error(t, err)
				assert.Contains(t, err.Error(), `URL 'exec-http://userdb' must have a 'exe' parameter`)
			},
		},
		{
			name: "bad-timeout",
			url:  "exec-http://userdb?exe=ls&init-timeout=abc",
			asserter: func(t *testing.T, err error) {
				require.Error(t, err)
				assert.Contains(t, err.Error(), `invalid param init-timeout for exec-http://userdb?exe=ls&init-timeout=abc`)
			},
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			_, err := FromURL(test.url)
			test.asserter(t, err)
		})
	}
}

func TestNewWithBadConfig(t *testing.T) {
	_, err := New("foo", Config{})
	require.Error(t, err)
	assert.Equal(t, "data source foo: executable not specified", err.Error())
}
