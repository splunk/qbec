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

package factory

import (
	"sync"

	"github.com/splunk/qbec/vm/datasource"
	"github.com/splunk/qbec/vm/internal/ds"
)

// lazySource wraps a data source and defers initialization of its delegate until the first call to Resolve.
// This allows data sources to be initialized before computed variables are, such that code in computed
// variables can also refer to data sources.
type lazySource struct {
	delegate ds.DataSourceWithLifecycle
	provider datasource.ConfigProvider
	l        sync.Mutex
	once     sync.Once
	initErr  error
}

func makeLazy(delegate ds.DataSourceWithLifecycle) ds.DataSourceWithLifecycle {
	return &lazySource{
		delegate: delegate,
	}
}

func (l *lazySource) Init(c datasource.ConfigProvider) error {
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

var _ ds.DataSourceWithLifecycle = &lazySource{}
