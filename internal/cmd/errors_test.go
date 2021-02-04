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
