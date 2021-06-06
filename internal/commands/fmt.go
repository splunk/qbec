package commands

import (
	"bytes"
	"fmt"
	"io"
	"io/fs"
	"io/ioutil"
	"os"

	"github.com/spf13/cobra"
	"github.com/splunk/qbec/internal/bulkfiles"
	"github.com/splunk/qbec/internal/cmd"
	"github.com/splunk/qbec/internal/sio"
)

var (
	supportedTypes = []string{"json", "jsonnet", "yaml"}
)

type fmtCommandConfig struct {
	cmd.AppContext
	opts           bulkfiles.Options
	check          bool
	write          bool
	formatTypes    map[string]bool
	specifiedTypes []string
	files          []string
}

type processor struct {
	config *fmtCommandConfig
}

func (p *processor) Matches(path string, f fs.FileInfo, userSpecified bool) bool {
	return shouldFormat(p.config, path, f, userSpecified)
}

func (p *processor) Process(path string, f fs.FileInfo) error {
	return processFile(p.config, path, nil, p.config.Stdout())
}

func doFmt(args []string, config *fmtCommandConfig) error {
	if config.check && config.write {
		return cmd.NewUsageError(fmt.Sprintf("check and write are not supported together"))
	}
	if len(args) > 0 {
		config.files = args
	} else {
		config.files = []string{"."}
	}
	config.formatTypes = make(map[string]bool)
	isSupported := func(s string) bool {
		for _, t := range supportedTypes {
			if s == t {
				return true
			}
		}
		return false
	}
	for _, s := range config.specifiedTypes {
		if !isSupported(s) {
			return cmd.NewUsageError(fmt.Sprintf("%q is not a supported type", s))
		}
		config.formatTypes[s] = true
	}
	config.opts.ContinueOnError = config.check
	p := &processor{config: config}
	return bulkfiles.Process(config.files, config.opts, p)
}

func newFmtCommand(cp ctxProvider) *cobra.Command {
	c := &cobra.Command{
		Use:     "fmt",
		Short:   "format files",
		Example: fmtExamples(),
	}
	deprecationNotice := "qbec alpha fmt is deprecated. Use qbec fmt instead. qbec alpha fmt would be removed in the next release"

	config := fmtCommandConfig{}
	c.Flags().BoolVarP(&config.check, "check-errors", "e", false, "check for unformatted files")
	c.Flags().BoolVarP(&config.write, "write", "w", false, "write result to (source) file instead of stdout")
	c.Flags().StringSliceVarP(&config.specifiedTypes, "type", "t", []string{"jsonnet"}, "file types that should be formatted")
	c.RunE = func(c *cobra.Command, args []string) error {
		if c.Parent().Name() == "alpha" {
			sio.Warnln(deprecationNotice)
		}
		config.AppContext = cp()
		return cmd.WrapError(doFmt(args, &config))
	}
	return c
}

func shouldFormat(config *fmtCommandConfig, _ string, f os.FileInfo, userSpecified bool) bool {
	if isJsonnetFile(f) {
		return config.formatTypes["jsonnet"] || userSpecified
	}
	if isYamlFile(f) {
		return config.formatTypes["yaml"] || userSpecified
	}
	if isJSONFile(f) {
		return config.formatTypes["json"] || userSpecified
	}
	return false
}

func processFile(config *fmtCommandConfig, filename string, in io.Reader, out io.Writer) error {
	if out == nil {
		out = os.Stdout
	}
	var perm os.FileMode = 0644
	if in == nil {
		f, err := os.Open(filename)
		if err != nil {
			return err
		}
		defer f.Close()
		fi, err := f.Stat()
		if err != nil {
			return err
		}
		in = f
		perm = fi.Mode().Perm()
	}

	src, err := ioutil.ReadAll(in)
	if err != nil {
		return err
	}

	res, err := format(src, filename)
	if err != nil {
		return fmt.Errorf("%s: error formatting file %q", filename, err)
	}

	if !bytes.Equal(src, res) {
		// formatting has changed
		if config.check {
			return fmt.Errorf(filename)
		}
		if config.write {
			// make a temporary backup before overwriting original
			bakname, err := backupFile(filename+".", src, perm)
			if err != nil {
				return fmt.Errorf("error creating backup file %q: %v", filename+".", err)
			}
			err = ioutil.WriteFile(filename, res, perm)
			if err != nil {
				os.Rename(bakname, filename)

				return fmt.Errorf("error writing file %q: %v", filename, err)
			}
			err = os.Remove(bakname)
			if err != nil {
				return fmt.Errorf("error removing backup file %q: %v", bakname, err)
			}
		}
	}

	if !config.check && !config.write {
		_, err = out.Write(res)
	}

	return err
}
