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

package cmd

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestUsageError(t *testing.T) {
	ue := NewUsageError("foobar")
	a := assert.New(t)
	a.True(IsUsageError(ue))
	a.Equal("foobar", ue.Error())
}

func TestRuntimeError(t *testing.T) {
	re := NewRuntimeError(errors.New("foobar"))
	a := assert.New(t)
	a.True(IsRuntimeError(re))
	a.False(IsUsageError(re))
	a.Equal("foobar", re.Error())
}

func TestWrapError(t *testing.T) {
	ue := NewUsageError("foobar")
	a := assert.New(t)
	a.Nil(WrapError(nil))
	a.True(IsUsageError(WrapError(ue)))
	a.True(IsRuntimeError(WrapError(errors.New("foobar"))))
}
