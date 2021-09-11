package vm

import (
	"fmt"
	"io"

	"github.com/pkg/errors"
	"github.com/splunk/qbec/vm/datasource"
	"github.com/splunk/qbec/vm/internal/dsfactory"
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
		return fmt.Errorf("%d close errrors: %v", len(errs), errs)
	}
}

func CreateDataSources(input []string, cp datasource.ConfigProvider) (sources []datasource.DataSource, closer io.Closer, _ error) {
	closer = &multiCloser{}
	for _, uri := range input {
		src, err := dsfactory.Create(uri)
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
