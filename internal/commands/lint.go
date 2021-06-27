package commands

import (
	"fmt"
	"io/fs"
	"io/ioutil"
	"net/url"
	"strings"

	"github.com/pkg/errors"
	"github.com/spf13/cobra"
	"github.com/splunk/qbec/internal/cmd"
	"github.com/splunk/qbec/internal/fswalk"
	"github.com/splunk/qbec/internal/sio"
	"github.com/splunk/qbec/internal/vm"
	"github.com/splunk/qbec/internal/vm/importers"
)

type mockDs struct {
	name string
}

func (m mockDs) Name() string {
	return m.name
}

func (m mockDs) Resolve(_ string) (string, error) {
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

func compressLines(s string) []string {
	inLines := strings.Split(s, "\n")
	prevEmpty := false
	var lines []string
	for _, line := range inLines {
		if line == "" {
			if prevEmpty {
				continue
			} else {
				prevEmpty = true
			}
		} else {
			prevEmpty = false
		}
		lines = append(lines, line)
	}
	return lines
}

type lintCommandConfig struct {
	vm       vm.VM
	opts     fswalk.Options
	loadApp  bool
	failFast bool
	files    []string
}

type linter struct {
	config *lintCommandConfig
}

func (p *linter) Matches(path string, f fs.FileInfo, userSpecified bool) bool {
	return userSpecified || strings.HasSuffix(path, ".jsonnet") || strings.HasSuffix(path, ".libsonnet")
}

func (p *linter) printError(err error) {
	if err == nil || !p.config.opts.ContinueOnError {
		return
	}
	fmt.Println("---")
	for i, line := range compressLines(err.Error()) {
		if i == 0 {
			fmt.Println(sio.ErrorString(line))
		} else {
			fmt.Println(line)
		}
	}
}

func (p *linter) Process(path string, f fs.FileInfo) (outErr error) {
	defer func() {
		p.printError(outErr)
	}()
	b, err := ioutil.ReadFile(path)
	if err != nil {
		return err
	}
	return p.doLint(path, b)
}

func (p *linter) doLint(file string, code []byte) (outErr error) {
	return p.config.vm.LintCode(file, vm.MakeCode(string(code)))
}

func doLint(args []string, config *lintCommandConfig, ac cmd.AppContext) error {
	if len(args) > 0 {
		config.files = args
	} else {
		config.files = []string{"."}
	}
	var libPaths []string
	var dataSources []importers.DataSource
	if ac.App() != nil {
		libPaths = ac.App().LibPaths()
		for _, dsStr := range ac.App().DataSources() {
			ds, err := createMockDatasource(dsStr)
			if err != nil {
				return errors.Wrapf(err, "create mock data source for %s", dsStr)
			}
			dataSources = append(dataSources, ds)
		}
	}
	cfg := vm.Config{
		LibPaths:    libPaths,
		DataSources: dataSources,
	}
	config.vm = vm.New(cfg)
	config.opts.VerboseWalk = ac.Context.Verbosity() > 0
	config.opts.ContinueOnError = !config.failFast
	p := &linter{config: config}
	return fswalk.Process(config.files, config.opts, p)
}

func newLintCommand(cp ctxProvider) *cobra.Command {
	c := &cobra.Command{
		Use:     "lint",
		Short:   "lint files using jsonnet-lint",
		Example: lintExamples(),
	}

	config := lintCommandConfig{}
	c.Flags().BoolVar(&config.loadApp, "app", true, "assume a qbec root and load qbec.yaml for lib paths and data sources")
	c.Flags().BoolVar(&config.failFast, "fail-fast", false, "fail on first error, stop processing other files")

	c.RunE = func(c *cobra.Command, args []string) error {
		ac := cp()
		return cmd.WrapError(doLint(args, &config, ac))
	}
	return c
}
