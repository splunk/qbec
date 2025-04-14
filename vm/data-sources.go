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

package vm

import (
	"fmt"
	"io"

	"github.com/pkg/errors"
	"github.com/splunk/qbec/vm/datasource"
	"github.com/splunk/qbec/vm/internal/ds/factory"
)

type multiCloser struct {
	closers []io.Closer
}

func (m *multiCloser) add(c io.Closer) {
	m.closers = append(m.closers, c)
}

func (m *multiCloser) Close() error {
	var errs []error
	for _, c := range m.closers {
		if err := c.Close(); err != nil {
			errs = append(errs, err)
		}
	}
	switch len(errs) {
	case 0:
		return nil
	case 1:
		return errs[0]
	default:
		return fmt.Errorf("%d close errors: %v", len(errs), errs)
	}
}

// CreateDataSources returns the data source implementations for the supplied URIs. It also returns an io.Closer that should
// be called at the point when the data sources are no longer in use. It guarantees that the returned closer will be non-nil
// even when there are errors.
func CreateDataSources(input []string, cp datasource.ConfigProvider) (sources []datasource.DataSource, closer io.Closer, _ error) {
	closer = &multiCloser{}
	for _, uri := range input {
		src, err := factory.Create(uri)
		if err != nil {
			return nil, closer, errors.Wrapf(err, "create data source %s", uri)
		}
		err = src.Init(cp)
		if err != nil {
			return nil, closer, errors.Wrapf(err, "init data source %s", uri)
		}
		sources = append(sources, src)
	}
	return sources, closer, nil
}

// ConfigProviderFromVariables returns a simple config provider for data sources based on a static set of variables
// that will be defined for the VM.
func ConfigProviderFromVariables(vs VariableSet) datasource.ConfigProvider {
	return func(name string) (string, error) {
		jvm := New(Config{})
		return jvm.EvalCode(
			fmt.Sprintf("<%s>", name),
			MakeCode(fmt.Sprintf(`std.extVar('%s')`, name)),
			vs,
		)
	}
}
