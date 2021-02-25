package datasource

import (
	"fmt"
	"testing"

	"github.com/splunk/qbec/internal/datasource/api"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type delegate struct {
	initErr      error
	resolveError error
	closeError   error
}

func (d *delegate) Init(_ api.ConfigProvider) error {
	return d.initErr
}

func (d *delegate) Name() string {
	return "me"
}

func (d *delegate) Resolve(path string) (string, error) {
	return fmt.Sprintf("resolved:%s", path), d.resolveError
}

func (d *delegate) Close() error {
	return d.closeError
}

var _ api.DataSource = &delegate{}

var cp = func(name string) (string, error) {
	return "", nil
}

func TestLazySuccess(t *testing.T) {
	d := &delegate{}
	l := makeLazy(d)
	assert.Equal(t, "me", l.Name())
	err := l.Init(cp)
	require.NoError(t, err)
	x, err := l.Resolve("/foo")
	require.NoError(t, err)
	assert.Equal(t, "resolved:/foo", x)
}

func TestLazyPropagatesInitError(t *testing.T) {
	d := &delegate{initErr: fmt.Errorf("init error")}
	l := makeLazy(d)
	err := l.Init(cp)
	require.NoError(t, err)
	_, err = l.Resolve("/foo")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "init error")
}

func TestLazyPropagatesResolveError(t *testing.T) {
	d := &delegate{resolveError: fmt.Errorf("resolve error")}
	l := makeLazy(d)
	err := l.Init(cp)
	require.NoError(t, err)
	_, err = l.Resolve("/foo")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "resolve error")
}

func TestLazyPropagatesCloseError(t *testing.T) {
	d := &delegate{closeError: fmt.Errorf("close error")}
	l := makeLazy(d)
	err := l.Init(cp)
	require.NoError(t, err)
	_, err = l.Resolve("/foo")
	require.NoError(t, err)
	err = l.Close()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "close error")
}
