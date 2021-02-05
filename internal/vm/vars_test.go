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
	"math/rand"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/google/go-jsonnet"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestVMScratchVariableSet(t *testing.T) {
	a := assert.New(t)
	c := VariableSet{}
	a.Nil(c.Vars())
	a.Nil(c.TopLevelVars())

	exts := []Var{
		NewVar("ext-foo", "bar"),
		NewCodeVar("ext-code-foo", "true"),
	}
	tlas := []Var{
		NewVar("tla-foo", "bar"),
		NewVar("tls-bar", "baz"),
		NewCodeVar("tla-code-foo", "100"),
		NewCodeVar("tla-code-bar", "true"),
	}

	c = c.WithVars(exts...).
		WithTopLevelVars(tlas...)
	a.Equal(2, len(c.Vars()))
	a.Equal(4, len(c.TopLevelVars()))
	a.True(c.HasTopLevelVar("tla-foo"))
	a.True(c.HasTopLevelVar("tla-code-foo"))
	a.False(c.HasTopLevelVar("ext-foo"))
	a.False(c.HasTopLevelVar("ext-code-foo"))

	a.False(c.HasVar("tla-foo"))
	a.False(c.HasVar("tla-code-foo"))
	a.True(c.HasVar("ext-foo"))
	a.True(c.HasVar("ext-code-foo"))

	c = c.WithoutTopLevel()
	a.False(c.HasTopLevelVar("tla-foo"))
	a.False(c.HasTopLevelVar("tla-code-foo"))
}

func TestVMNoopVariableSet(t *testing.T) {
	c := VariableSet{}
	newC := c.WithoutTopLevel().WithVars().
		WithTopLevelVars()
	assert.Equal(t, &newC, &c)
}

func TestVMBadCodeVar(t *testing.T) {
	c := VariableSet{}.WithVars(NewCodeVar("foo", "{ foo: bar"))
	jvm := jsonnet.MakeVM()
	c.register(jvm)
	_, err := jvm.EvaluateAnonymousSnippet("foo.jsonnet", `std.extVar('foo')`)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "<extvar:foo>:1:11 Expected a comma before next field")
}

func TestVMBadTLACodeVar(t *testing.T) {
	c := VariableSet{}.WithTopLevelVars(NewCodeVar("foo", "{ foo: bar"))
	jvm := jsonnet.MakeVM()
	c.register(jvm)
	_, err := jvm.EvaluateAnonymousSnippet("foo.jsonnet", `function (foo) foo`)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "<top-level-arg:foo>:1:11 Expected a comma before next field")
}

func TestVMBadCodeVarNoRef(t *testing.T) {
	c := VariableSet{}.WithVars(NewCodeVar("foo", "{ foo: bar"))
	jvm := jsonnet.MakeVM()
	c.register(jvm)
	ret, err := jvm.EvaluateAnonymousSnippet("foo.jsonnet", `10`)
	require.NoError(t, err)
	assert.Equal(t, "10\n", ret)
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
