package commands

import (
	"fmt"
	"io/fs"
	"io/ioutil"
	"net/url"
	"strings"

	"github.com/pkg/errors"
	"github.com/spf13/cobra"
	"github.com/splunk/qbec/internal/bulkfiles"
	"github.com/splunk/qbec/internal/cmd"
	"github.com/splunk/qbec/internal/vm"
	"github.com/splunk/qbec/internal/vm/importers"
)

type lintCommandConfig struct {
	vm    vm.VM
	opts  bulkfiles.Options
	check bool
	files []string
}

type linter struct {
	config *lintCommandConfig
}

func (p *linter) Matches(path string, f fs.FileInfo, userSpecified bool) bool {
	return userSpecified || strings.HasSuffix(path, ".jsonnet") || strings.HasSuffix(path, ".libsonnet")
}

func (p *linter) Process(path string, f fs.FileInfo) error {
	b, err := ioutil.ReadFile(path)
	if err != nil {
		return err
	}
	return p.config.vm.LintCode(path, vm.MakeCode(string(b)))
}

func doLint(args []string, config *lintCommandConfig) error {
	if len(args) > 0 {
		config.files = args
	} else {
		config.files = []string{"."}
	}
	config.opts.ContinueOnError = config.check
	p := &linter{config: config}
	return bulkfiles.Process(config.files, config.opts, p)
}

type mockDs struct {
	name string
}

func (m mockDs) Name() string {
	return m.name
}

func (m mockDs) Resolve(path string) (string, error) {
	return "", nil
}

func createMockDatasource(u string) (importers.DataSource, error) {
	parsed, err := url.Parse(u)
	if err != nil {
		return nil, err
	}
	if parsed.Host == "" {
		return nil, fmt.Errorf("unable to find data source name")
	}
	return mockDs{name: parsed.Host}, nil
}

func newLintCommand(cp ctxProvider) *cobra.Command {
	c := &cobra.Command{
		Use:   "lint",
		Short: "lint files",
		//Example: fmtExamples(),
	}

	config := lintCommandConfig{}
	c.Flags().BoolVarP(&config.check, "check-errors", "e", false, "check for files that fail lint")
	c.RunE = func(c *cobra.Command, args []string) error {
		ac := cp()
		cfg := vm.Config{
			LibPaths: ac.App().LibPaths(),
		}
		for _, dsStr := range ac.App().DataSources() {
			ds, err := createMockDatasource(dsStr)
			if err != nil {
				return errors.Wrapf(err, "create mock data source for %s", dsStr)
			}
			cfg.DataSources = append(cfg.DataSources, ds)
		}
		config.vm = vm.New(cfg)
		return cmd.WrapError(doLint(args, &config))
	}
	return c
}
