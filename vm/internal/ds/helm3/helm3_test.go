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

package helm3

import (
	"encoding/json"
	"io/ioutil"
	"net/url"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestTemplateOptionsEmpty(t *testing.T) {
	tc := TemplateOptions{}
	parts := tc.toCommandLine()
	require.Equal(t, 0, len(parts))
}

func TestTemplateOptionsFull(t *testing.T) {
	tc := TemplateOptions{
		Namespace:                "default",
		CreateNamespace:          true,
		APIVersions:              []string{"1.2", "2.1"},
		DependencyUpdate:         true,
		Description:              "foo bar",
		Devel:                    true,
		DisableOpenAPIValidation: true,
		GenerateName:             true,
		IncludeCRDs:              true,
		IsUpgrade:                true,
		SkipCRDs:                 true,
		SkipTests:                true,
		InsecureSkipTLSVerify:    true,
		KubeVersion:              "1.16",
		NameTemplate:             "gentemp",
		NoHooks:                  true,
		PassCredentials:          true,
		Password:                 "foobar",
		RenderSubchartNotes:      true,
		Replace:                  true,
		Repo:                     "r1",
		ShowOnly:                 []string{"foo", "bar"},
		Username:                 "me",
		Validate:                 true,
		Verify:                   true,
		Version:                  "1.2.3",
	}
	ret := tc.toCommandLine()
	a := assert.New(t)
	a.Contains(ret, "--namespace=default")
	a.Contains(ret, "--api-versions=1.2")
	a.Contains(ret, "--api-versions=2.1")
	a.Contains(ret, "--kube-version=1.16")
	a.Contains(ret, "--name-template=gentemp")
	a.Contains(ret, "--password=foobar")
	a.Contains(ret, "--repo=r1")
	a.Contains(ret, "--show-only=foo")
	a.Contains(ret, "--show-only=bar")
	a.Contains(ret, "--username=me")
	a.Contains(ret, "--version=1.2.3")
	for _, s := range []string{
		"create-namespace",
		"dependency-update",
		"devel",
		"disable-openapi-validation",
		"generate-name",
		"include-crds",
		"is-upgrade",
		"skip-crds",
		"skip-tests",
		"insecure-skip-tls-verify",
		"no-hooks",
		"pass-credentials",
		"render-subchart-notes",
		"replace",
		"validate",
		"verify",
	} {
		a.Contains(ret, "--"+s)
	}

	str := tc.toDisplay()
	a.Contains(str, "--namespace=default")
	a.Contains(str, "--api-versions=1.2")
	a.Contains(str, "--api-versions=2.1")
	a.Contains(str, "--kube-version=1.16")
	a.Contains(str, "--name-template=gentemp")
	a.Contains(str, "--password=<REDACTED>")
	a.Contains(str, "--repo=r1")
	a.Contains(str, "--show-only=foo")
	a.Contains(str, "--show-only=bar")
	a.Contains(str, "--username=me")
	a.Contains(str, "--version=1.2.3")
	for _, s := range []string{
		"create-namespace",
		"dependency-update",
		"devel",
		"disable-openapi-validation",
		"generate-name",
		"include-crds",
		"is-upgrade",
		"skip-crds",
		"skip-tests",
		"insecure-skip-tls-verify",
		"no-hooks",
		"pass-credentials",
		"render-subchart-notes",
		"replace",
		"validate",
		"verify",
	} {
		a.Contains(str, "--"+s)
	}
}

func TestRunTemplate(t *testing.T) {

	mockConfigProvider := func(varName string) (string, error) {
		if varName == "config-var-name" {
			return `{"command": "helm", "timeout": "5s"}`, nil
		}
		return "", nil
	}

	mockTemplateConfig := TemplateConfig{
		Name: "mock-release",
		Options: TemplateOptions{
			Repo:      "https://charts.bitnami.com/bitnami",
			Namespace: "foobar",
			Version:   "10.1.0",
		},
		Values: map[string]interface{}{
			"key": "value",
		},
	}

	helm3Src := &helm3Source{configVar: "config-var-name"}
	err := helm3Src.Init(mockConfigProvider)
	require.NoError(t, err)
	mockURL := &url.URL{
		Scheme: "http",
		Host:   "",
		Path:   "apache",
	}
	templated, err := helm3Src.runTemplate(mockURL, mockTemplateConfig)
	b, err := json.MarshalIndent(templated, "", "  ")
	require.NoError(t, err)
	filePath := filepath.Join("testdata", "apache.json")
	fileContent, err := ioutil.ReadFile(filePath)
	require.NoError(t, err)
	require.Equal(t, fileContent, b)
}

func TestInitDefaults(t *testing.T) {
	cfg := Config{}
	cfg.initDefaults()
	require.Equal(t, cfg.Command, "helm")
	require.Equal(t, cfg.Timeout, "")
	require.Equal(t, cfg.timeout, time.Minute)
}

func TestFindExecutable(t *testing.T) {
	cmd, err := findExecutable("helm")
	require.NoError(t, err)
	require.FileExists(t, cmd)
}

func TestClose(t *testing.T) {
	helm3Src := &helm3Source{configVar: "config-var-name"}
	err := helm3Src.Close()
	require.NoError(t, err)
}

func TestName(t *testing.T) {
	mockConfigProvider := func(varName string) (string, error) {
		if varName == "config-var-name" {
			return `{"command": "helm", "timeout": "5s"}`, nil
		}
		return "", nil
	}

	helm3Src := &helm3Source{name: "baz", configVar: "config-var-name"}
	err := helm3Src.Init(mockConfigProvider)
	require.NoError(t, err)
	name := helm3Src.Name()
	require.Equal(t, name, "baz")
}
