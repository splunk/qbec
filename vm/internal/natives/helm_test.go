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

package natives

import (
	"encoding/json"
	"sort"
	"testing"

	"github.com/google/go-jsonnet"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestHelmOptions(t *testing.T) {
	a := assert.New(t)
	var h helmOptions
	a.Nil(h.toArgs())
	h = helmOptions{
		Execute:     []string{"a.yaml", "b.yaml"},
		KubeVersion: "1.10",
		Name:        "foo",
		Namespace:   "foobar",
		ThisFile:    "/path/to/my.jsonnet",
		Verbose:     true,
	}
	a.EqualValues([]string{
		"--execute", "a.yaml",
		"--execute", "b.yaml",
		"--kube-version", "1.10",
		"--name", "foo",
		"--namespace", "foobar",
	}, h.toArgs())
}

type cmOrSecret struct {
	APIVersion string `json:"apiVersion"`
	Kind       string `json:"kind"`
	Metadata   struct {
		Namespace string `json:"namespace"`
		Name      string `json:"name"`
	}
	Data map[string]string `json:"data"`
}

func TestHelmSimpleExpand(t *testing.T) {
	a := assert.New(t)
	jvm := jsonnet.MakeVM()
	Register(jvm)
	code, err := jvm.EvaluateFile("./testdata/consumer.jsonnet")
	require.Nil(t, err)

	var output []cmOrSecret
	err = json.Unmarshal([]byte(code), &output)
	require.Nil(t, err)

	require.Equal(t, 2, len(output))

	sort.Slice(output, func(i, j int) bool {
		return output[i].Kind < output[j].Kind
	})

	ob := output[0]
	a.Equal("ConfigMap", ob.Kind)
	a.Equal("my-ns", ob.Metadata.Namespace)
	a.Equal("my-name", ob.Metadata.Name)
	a.Equal("barbar", ob.Data["foo"])
	a.Equal("baz", ob.Data["bar"])

	ob = output[1]
	a.Equal("Secret", ob.Kind)
	a.Equal("my-ns", ob.Metadata.Namespace)
	a.Equal("my-name", ob.Metadata.Name)
	a.Equal("Y2hhbmdlbWUK", ob.Data["secret"])
}

func TestHelmBadRelative(t *testing.T) {
	a := assert.New(t)
	jvm := jsonnet.MakeVM()
	Register(jvm)
	_, err := jvm.EvaluateFile("./testdata/bad-relative.jsonnet")
	require.NotNil(t, err)
	a.Contains(err.Error(), "exit status 1")
}
