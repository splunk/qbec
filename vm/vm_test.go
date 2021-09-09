/*
   Copyright 2019 Splunk Inc.

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
	"encoding/json"
	"fmt"
	"math/rand"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/splunk/qbec/vm/datasource"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestVMEvalFile(t *testing.T) {
	vm := New(Config{LibPaths: []string{"testdata/vmlib"}})
	out, err := vm.EvalFile(
		"testdata/vmtest.jsonnet",
		VariableSet{}.WithVars(
			NewVar("foo", "fooVal"),
			NewCodeVar("bar", "true"),
		),
	)
	require.NoError(t, err)
	var data struct {
		Foo string `json:"foo"`
		Bar bool   `json:"bar"`
	}
	err = json.Unmarshal([]byte(out), &data)
	require.NoError(t, err)
	assert.Equal(t, "fooVal", data.Foo)
	assert.True(t, data.Bar)
}

func TestVMEvalCode(t *testing.T) {
	vm := New(Config{LibPaths: []string{"testdata/vmlib"}})
	out, err := vm.EvalCode(
		"fake.jsonnet",
		MakeCode(`
			import 'testdata/vmtest.jsonnet'
		`),
		VariableSet{}.WithVars(
			NewVar("foo", "fooVal"),
			NewCodeVar("bar", "true"),
		),
	)
	require.NoError(t, err)
	var data struct {
		Foo string `json:"foo"`
		Bar bool   `json:"bar"`
	}
	err = json.Unmarshal([]byte(out), &data)
	require.NoError(t, err)
	assert.Equal(t, "fooVal", data.Foo)
	assert.True(t, data.Bar)
}

func TestVMEvalNonExistentFile(t *testing.T) {
	vm := New(Config{})
	_, err := vm.EvalFile("testdata/does-not-exist.jsonnet", VariableSet{})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "testdata/does-not-exist.jsonnet: file not found")
}

func TestVMEvalDir(t *testing.T) {
	vm := New(Config{})
	_, err := vm.EvalFile("testdata", VariableSet{})
	require.Error(t, err)
	assert.Equal(t, err.Error(), "file 'testdata' was a directory")
}

func testExtVar(vm VM) error {
	val := fmt.Sprint(rand.Intn(10000000))
	ret, err := vm.EvalFile("testdata/parallel-ext-vars.jsonnet", VariableSet{}.WithVars(NewCodeVar("foo", val)))
	if err != nil {
		return err
	}
	ret = strings.TrimRight(ret, "\r\n")
	if ret != val {
		return fmt.Errorf("EXT: want '%s', got '%s'", val, ret)
	}
	return nil
}

func testTLAVar(vm VM) error {
	val := fmt.Sprint(rand.Intn(10000000))
	ret, err := vm.EvalFile("testdata/parallel-tla-vars.jsonnet", VariableSet{}.WithTopLevelVars(NewCodeVar("foo", val)))
	if err != nil {
		return err
	}
	ret = strings.TrimRight(ret, "\r\n")
	if ret != val {
		return fmt.Errorf("TLA: want '%s', got '%s'", val, ret)
	}
	return nil
}

func TestVMConcurrency(t *testing.T) {
	vm := New(Config{})
	concurrency := 20
	total := 20000

	rand.Seed(time.Now().Unix())
	queue := make(chan struct{}, total)
	for i := 0; i < total; i++ {
		queue <- struct{}{}
	}
	close(queue)

	done := make(chan struct{}) // barrier for early exit
	var once sync.Once
	var evalError error
	closeDone := func(err error) {
		evalError = err
		once.Do(func() { close(done) })
	}
	worker := func() {
		for {
			select {
			case <-done:
				return
			case _, ok := <-queue:
				if !ok {
					return
				}
			}
			var err error
			if rand.Intn(2) == 0 {
				err = testExtVar(vm)
			} else {
				err = testTLAVar(vm)
			}
			if err != nil {
				closeDone(err)
			}
		}
	}

	var wg sync.WaitGroup
	startCh := make(chan struct{}) // barrier for start
	for i := 0; i < concurrency; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-startCh
			worker()
		}()
	}
	close(startCh)
	wg.Wait()
	require.NoError(t, evalError)
}

type replay struct {
	name string
}

func (d *replay) Name() string {
	return d.name
}

func (d *replay) Resolve(path string) (string, error) {
	out := struct {
		Source string `json:"source"`
		Path   string `json:"path"`
	}{d.name, path}
	b, err := json.MarshalIndent(out, "", "  ")
	if err != nil {
		return "", err
	}
	return string(b), nil
}

func TestVMDataSource(t *testing.T) {
	jvm := New(Config{
		LibPaths:    []string{},
		DataSources: []datasource.DataSource{&replay{name: "replay"}},
	})
	jsonCode, err := jvm.EvalFile("testdata/data-sources/replay.jsonnet", VariableSet{})
	require.NoError(t, err)
	var data map[string]interface{}
	err = json.Unmarshal([]byte(jsonCode), &data)
	require.NoError(t, err)
	assert.Equal(t, "replay", data["source"])
	assert.Equal(t, "/foo/bar", data["path"])
}

func TestVMTwoDataSources(t *testing.T) {
	jvm := New(Config{
		LibPaths: []string{},
		DataSources: []datasource.DataSource{
			&replay{name: "replay"},
			&replay{name: "replay2"},
		},
	})
	jsonCode, err := jvm.EvalFile("testdata/data-sources/replay2.jsonnet", VariableSet{})
	require.NoError(t, err)
	var data []map[string]interface{}
	err = json.Unmarshal([]byte(jsonCode), &data)
	require.NoError(t, err)
	require.Equal(t, 2, len(data))
	assert.Equal(t, "replay", data[0]["source"])
	assert.Equal(t, "/foo/bar", data[0]["path"])
	assert.Equal(t, "replay2", data[1]["source"])
	assert.Equal(t, "/bar/baz", data[1]["path"])
}

func TestVMLint(t *testing.T) {
	vm := New(Config{})
	err := vm.LintCode("foo.jsonnet", MakeCode("{}"))
	require.NoError(t, err)
	err = vm.LintCode("foo.jsonnet", MakeCode("local foo=10; {}"))
	require.Error(t, err)
	assert.Contains(t, err.Error(), "foo.jsonnet:1:7-13 Unused variable: foo")
	err = vm.LintCode("foo.jsonnet", MakeCode("}"))
	require.Error(t, err)
	assert.Contains(t, err.Error(), "foo.jsonnet:1:1-2")
}
