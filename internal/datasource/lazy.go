package datasource

import (
	"sync"

	"github.com/splunk/qbec/internal/datasource/api"
)

type lazySource struct {
	delegate api.DataSource
	provider api.ConfigProvider
	l        sync.Mutex
	once     sync.Once
	initErr  error
}

func makeLazy(delegate api.DataSource) api.DataSource {
	return &lazySource{
		delegate: delegate,
	}
}

func (l *lazySource) Init(c api.ConfigProvider) error {
	l.provider = c
	return nil
}

func (l *lazySource) Name() string {
	return l.delegate.Name()
}

func (l *lazySource) initOnce() error {
	l.l.Lock()
	defer l.l.Unlock()
	l.once.Do(func() {
		l.initErr = l.delegate.Init(l.provider)
	})
	return l.initErr
}

func (l *lazySource) Resolve(path string) (string, error) {
	if err := l.initOnce(); err != nil {
		return "", err
	}
	return l.delegate.Resolve(path)
}

func (l *lazySource) Close() error {
	return l.delegate.Close()
}

var _ api.DataSource = &lazySource{}
