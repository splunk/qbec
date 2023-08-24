// Copyright 2021 Splunk Inc.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package types

import (
	"encoding/json"
	"io/ioutil"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"testing"

	"github.com/splunk/qbec/internal/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

func TestObjectStatus(t *testing.T) {
	os := RolloutStatus{}
	os.withDesc("foo").withDone(true)
	assert.Equal(t, "foo", os.Description)
	assert.True(t, os.Done)
}

func load(t *testing.T, file string) *unstructured.Unstructured {
	b, err := ioutil.ReadFile(file)
	require.Nil(t, err)
	var data map[string]interface{}
	err = json.Unmarshal(b, &data)
	require.Nil(t, err)
	return &unstructured.Unstructured{Object: data}
}

func checkExpectedStatus(t *testing.T, data *unstructured.Unstructured, rev int64, fn RolloutStatusFunc) {
	a := assert.New(t)
	expectedDesc := data.GetAnnotations()["test/status"]
	expectedDone := data.GetAnnotations()["test/done"] == "true"
	expectedErr := data.GetAnnotations()["test/error"]

	status, err := fn(data, rev)
	if expectedErr != "" {
		require.NotNil(t, err)
		if strings.HasPrefix(expectedErr, "/") && strings.HasSuffix(expectedErr, "/") {
			e := regexp.MustCompile(expectedErr[1 : len(expectedErr)-1])
			a.Regexp(e, err.Error())
			return
		}
		a.Equal(expectedErr, err.Error())
		return
	}

	require.Nil(t, err)
	require.NotNil(t, status)
	a.Equal(expectedDesc, status.Description)
	a.Equal(expectedDone, status.Done)
}

func testDir(t *testing.T, dir string) {
	files, err := filepath.Glob(filepath.Join("testdata", dir, "*.json"))
	require.Nil(t, err)
	for _, file := range files {
		t.Run(file, func(t *testing.T) {
			un := load(t, file)
			var rev int64
			inputRevStr := un.GetAnnotations()["test-input/revision"]
			if inputRevStr != "" {
				var err error
				rev, err = strconv.ParseInt(inputRevStr, 10, 64)
				require.Nil(t, err)
			}
			statusFn := StatusFuncFor(model.NewK8sObject(un.Object))
			require.NotNil(t, statusFn)
			checkExpectedStatus(t, un, rev, statusFn)
		})
	}
}

func TestDeployStatus(t *testing.T) {
	testDir(t, "deploy")
}

func TestDaemonSetStatus(t *testing.T) {
	testDir(t, "daemonset")
}

func TestStatefulSetStatus(t *testing.T) {
	testDir(t, "statefulset")
}

func TestUnknownObject(t *testing.T) {
	obj := model.NewK8sObject(map[string]interface{}{
		"kind":       "foo",
		"apiversion": "apps/v1",
		"metadata": map[string]interface{}{
			"namespace": "foo",
			"name":      "foo",
		},
	})
	statusFn := StatusFuncFor(obj)
	require.Nil(t, statusFn)
}
