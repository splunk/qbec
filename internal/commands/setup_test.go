package commands

import (
	"bytes"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"path"
	"testing"

	"github.com/spf13/cobra"
	"github.com/splunk/qbec/internal/testutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestVersionCommand(t *testing.T) {
	s := newScaffold(t)
	defer s.reset()
	err := s.executeCommand("version")
	require.NoError(t, err)
	out := s.stdout()
	a := assert.New(t)
	a.Contains(out, "qbec version: "+version)
	a.Contains(out, "jsonnet version: "+jsonnetVersion)
	a.Contains(out, "client-go version: "+clientGoVersion)
	a.Contains(out, "go version: "+goVersion)
	a.Contains(out, "commit: "+commit)
}

func TestVersionCommandJSON(t *testing.T) {
	s := newScaffold(t)
	defer s.reset()
	err := s.executeCommand("version", "--json")
	require.NoError(t, err)
	var out map[string]string
	err = s.jsonOutput(&out)
	require.NoError(t, err)
	a := assert.New(t)
	a.Equal(version, out["qbec"])
	a.Equal(goVersion, out["go"])
	a.Equal(clientGoVersion, out["client-go"])
	a.Equal(commit, out["commit"])
	a.Equal(jsonnetVersion, out["jsonnet"])
}

func TestOptionsCommand(t *testing.T) {
	s := newScaffold(t)
	defer s.reset()
	err := s.executeCommand("options")
	require.NoError(t, err)
	a := assert.New(t)
	expectedStrings := []string{
		"--app-tag",
		"--colors",
		"--env-file",
		"--eval-concurrency",
		"--force:k8s-context",
		"--k8s:as",
		"--root",
		"--strict-vars",
		"--verbose",
		"--vm:ext-str",
		"--yes",
	}
	for _, str := range expectedStrings {
		a.Contains(s.stdout(), str)
	}
}

func TestSetupNoFail(t *testing.T) {
	assert.NotPanics(t, func() { Setup(&cobra.Command{}) })
}

func Handler(w http.ResponseWriter, r *http.Request) {
	_, _ = fmt.Fprintf(w, "Received %s\n", path.Base(r.URL.Path))
}

type HandlerTransport struct{ h http.Handler }

var payload = []byte(`
apiVersion: qbec.io/v1alpha1
kind: EnvironmentMap
spec:
  environments:
    prod2:
      server: https://prod-server
      includes:
      - service2
      properties:
        envType: prod`)

func (t HandlerTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	resp := &http.Response{
		Status:  "200 OK",
		StatusCode: 200,
		Body:    ioutil.NopCloser(bytes.NewBuffer(payload)),
		Request: req,
	}
	return resp, nil
}

func TestDownloadEnv(t *testing.T) {
	s := newScaffold(t)
	defer s.reset()
	httpClient.Transport = HandlerTransport{http.HandlerFunc(Handler)}
	err := s.executeCommand("env", "list", "--env-file", "http://localhost:5000/service/env-file")
	require.NoError(t, err)
}

func TestSetupEnvironments(t *testing.T) {
	tests := []struct {
		name   string
		fn     func(t *testing.T, s *scaffold)
		envMap map[string]string
	}{
		{
			name: "explicit root",
			fn: func(t *testing.T, s *scaffold) {
				err := s.executeCommand("--root", "testdata", "env", "list")
				require.NoError(t, err)
				out := s.stdout()
				assert.Contains(t, out, "dev")
				assert.Contains(t, out, "minikube")
			},
		},
		{
			name:   "env root",
			envMap: map[string]string{"QBEC_ROOT": "testdata"},
			fn: func(t *testing.T, s *scaffold) {
				err := s.executeCommand("env", "list")
				require.NoError(t, err)
				out := s.stdout()
				assert.Contains(t, out, "dev")
				assert.Contains(t, out, "minikube")
			},
		},
		{
			name: "bad root",
			fn: func(t *testing.T, s *scaffold) {
				err := s.executeCommand("env", "list")
				require.NotNil(t, err)
				assert.Contains(t, err.Error(), "unable to find source root")
			},
		},
		{
			name:   "env file",
			envMap: map[string]string{"QBEC_ROOT": "testdata"},
			fn: func(t *testing.T, s *scaffold) {
				err := s.executeCommand("env", "list", "-E", "testdata/extra-env.yaml")
				require.NoError(t, err)
				out := s.stdout()
				assert.Contains(t, out, "dev")
				assert.Contains(t, out, "minikube")
				assert.Contains(t, out, "prod")
			},
		},
		{
			name:   "env file from env var",
			envMap: map[string]string{"QBEC_ROOT": "testdata", "QBEC_ENV_FILE": "testdata/extra-env.yaml"},
			fn: func(t *testing.T, s *scaffold) {
				err := s.executeCommand("env", "list")
				require.NoError(t, err)
				out := s.stdout()
				assert.Contains(t, out, "dev")
				assert.Contains(t, out, "minikube")
				assert.Contains(t, out, "prod")
			},
		},
		{
			name:   "bad env file",
			envMap: map[string]string{"QBEC_ROOT": "testdata"},
			fn: func(t *testing.T, s *scaffold) {
				err := s.executeCommand("env", "list", "-E", "testdata/extra-env2.yaml")
				require.NotNil(t, err)
				assert.Contains(t, err.Error(), testutil.FileNotFoundMessage)
			},
		},
		{
			name: "force current context",
			envMap: map[string]string{
				"QBEC_ROOT":  "testdata",
				"KUBECONFIG": "../../../examples/test-app/kubeconfig.yaml",
			},
			fn: func(t *testing.T, s *scaffold) {
				err := s.executeCommand("env", "vars",
					"--force:k8s-context=__current__",
					"--force:k8s-namespace=__current__",
					"-o", "json",
					"dev")
				require.NoError(t, err)
				out := map[string]string{}
				err = s.jsonOutput(&out)
				require.NoError(t, err)
				assert.Equal(t, "prod", out["context"])
				assert.Equal(t, "barbaz", out["namespace"])
				assert.Equal(t, "prod", out["cluster"])
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			oldMap := map[string]string{}
			var unsetKeys []string
			for k, v := range test.envMap {
				oldV, ok := os.LookupEnv(k)
				if ok {
					oldMap[k] = oldV
				} else {
					unsetKeys = append(unsetKeys, k)
				}
				_ = os.Setenv(k, v)
			}
			envReset := func() {
				for k, v := range oldMap {
					_ = os.Setenv(k, v)
				}
				for _, k := range unsetKeys {
					_ = os.Unsetenv(k)
				}
			}
			s := newCustomScaffold(t, ".")
			defer s.reset()
			defer envReset()
			test.fn(t, s)
		})

	}
}
