package commands

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/google/go-jsonnet/formatter"
	"github.com/spf13/cobra"
	"github.com/tidwall/pretty"
	"gopkg.in/yaml.v3"
)

type fmtCommandConfig struct {
	*config
	check         bool
	write         bool
	formatTypes   map[string]bool
	formatJsonnet bool
	formatYaml    bool
	formatJson    bool
	files         []string
}

func doFmt(args []string, config *fmtCommandConfig) error {
	if config.check && config.write {
		return newUsageError(fmt.Sprintf("check and write are not supported together"))
	}
	if len(args) > 0 {
		config.files = args
	} else {
		config.files = []string{"."}
	}
	for _, path := range config.files {
		switch dir, err := os.Stat(path); {
		case err != nil:
			return err
		case dir.IsDir():
			return walkDir(config, path)
		default:
			if shouldFormat(config, dir) {
				if err := processFile(config, path, nil, config.Stdout()); err != nil {
					return err
				}
			}
		}
	}
	return nil
}

var (
	supportedTypes = []string{"json,jsonnet,yaml"}
)

func newFmtCommand(cp configProvider) *cobra.Command {
	cmd := &cobra.Command{
		Use:     "fmt",
		Short:   "format files",
		Example: fmtExamples(),
	}

	config := fmtCommandConfig{}
	var specifiedTypes []string
	cmd.Flags().BoolVarP(&config.check, "check-errors", "e", false, "check for unformatted files")
	cmd.Flags().BoolVarP(&config.write, "write", "w", false, "write result to (source) file instead of stdout")
	cmd.Flags().StringSliceVar(&specifiedTypes, "types", supportedTypes, "file types that should be formatted. Supports yaml,json,jsonnet.")
	cmd.Flags().BoolVar(&config.formatJsonnet, "jsonnet", true, "format jsonnet and libsonnet files")
	cmd.Flags().BoolVar(&config.formatJson, "json", true, "format json files")
	cmd.Flags().BoolVar(&config.formatYaml, "yaml", true, "format yaml files")
	cmd.RunE = func(c *cobra.Command, args []string) error {
		config.config = cp()
		return wrapError(doFmt(args, &config))
	}
	return cmd
}

func isYamlFile(f os.FileInfo) bool {
	name := f.Name()
	return !f.IsDir() && !strings.HasPrefix(name, ".") && getFileType(name) == "yaml"
}

func isJsonnetFile(f os.FileInfo) bool {
	name := f.Name()
	return !f.IsDir() && !strings.HasPrefix(name, ".") && getFileType(name) == "jsonnet"
}

func isJsonFile(f os.FileInfo) bool {
	name := f.Name()
	return !f.IsDir() && !strings.HasPrefix(name, ".") && getFileType(name) == "json"
}

func shouldFormat(config *fmtCommandConfig, f os.FileInfo) bool {
	if config.formatJsonnet && config.formatYaml {
		return isJsonnetFile(f) || isYamlFile(f)
	}
	if config.formatJsonnet {
		return isJsonnetFile(f)
	}
	if config.formatYaml {
		return isYamlFile(f)
	}
	if config.formatJson {
		return isJsonFile(f)
	}
	return false
}
func walkDir(config *fmtCommandConfig, path string) error {
	return filepath.Walk(path, fileVisitor(config))
}

func fileVisitor(config *fmtCommandConfig) filepath.WalkFunc {
	return func(path string, f os.FileInfo, err error) error {
		if err == nil && shouldFormat(config, f) {
			err = processFile(config, path, nil, config.Stdout())
		}
		// Don't complain if a file was deleted in the meantime (i.e.
		// the directory changed concurrently while running fmt).
		if err != nil && !os.IsNotExist(err) {
			return err
		}
		return nil
	}
}

func processFile(config *fmtCommandConfig, filename string, in io.Reader, out io.Writer) error {
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
		return fmt.Errorf("error formatting file %q: %v", filename, err)
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

const chmodSupported = runtime.GOOS != "windows"

// From https://golang.org/src/cmd/gofmt/gofmt.go
// backupFile writes data to a new file named filename<number> with permissions perm,
// with <number randomly chosen such that the file name is unique. backupFile returns
// the chosen file name.
func backupFile(filename string, data []byte, perm os.FileMode) (string, error) {
	// create backup file
	f, err := ioutil.TempFile(filepath.Dir(filename), filepath.Base(filename))
	if err != nil {
		return "", err
	}
	bakname := f.Name()
	if chmodSupported {
		err = f.Chmod(perm)
		if err != nil {
			f.Close()
			os.Remove(bakname)
			return bakname, err
		}
	}

	// write data to backup file
	_, err = f.Write(data)
	if err1 := f.Close(); err == nil {
		err = err1
	}

	return bakname, err
}

func format(in []byte, filename string) ([]byte, error) {
	if getFileType(filename) == "yaml" {
		return formatYaml(in)
	}
	if getFileType(filename) == "jsonnet" {
		return formatJsonnet(in)
	}
	if getFileType(filename) == "json" {
		return formatJson(in)
	}
	return nil, fmt.Errorf("unknown file type for file %q", filename)
}

const separator = "---\n"
const yamlSeparator = "\n---"

func formatYaml(in []byte) ([]byte, error) {
	scanner := bufio.NewScanner(bytes.NewReader(in))
	// the size of initial allocation for buffer 4k
	buf := make([]byte, 4*1024)
	// the maximum size used to buffer a token 5M
	scanner.Buffer(buf, 5*1024*1024)
	scanner.Split(splitYAMLDocument)
	var formatted []byte
	var i = 0
	for scanner.Scan() {
		err := scanner.Err()
		if err != nil {
			if err == io.EOF {
				break
			}
			return nil, err
		}
		var doc yaml.Node
		doc.Style = yaml.FlowStyle
		if err := yaml.Unmarshal(scanner.Bytes(), &doc); err != nil {
			return nil, err
		}
		var b bytes.Buffer
		e := yaml.NewEncoder(&b)
		e.SetIndent(2)
		if len(doc.Content) == 0 {
			// skip empty yaml files
			continue
		}
		err = e.Encode(doc.Content[0])
		//y, err := yaml.Marshal(doc.Content[0])
		if err != nil {
			return nil, err
		}
		y := b.Bytes()
		if i > 0 {
			formatted = append(append(formatted, []byte(separator)...), y...)
		} else {
			formatted = append(formatted, y...)
		}

		i++
	}
	return formatted, nil
}

func formatJsonnet(in []byte) ([]byte, error) {
	var ret, err = formatter.Format("", string(in), formatter.DefaultOptions())
	if err != nil {
		return nil, err
	}
	return []byte(ret), nil
}

func formatJson(in []byte) ([]byte, error) {
	var j interface{}
	decoder := json.NewDecoder(bytes.NewReader(in))
	decoder.UseNumber()
	//Validate input json
	defaultOptions := pretty.DefaultOptions
	// For array values to spread across lines
	defaultOptions.Width = 1
	var err = decoder.Decode(&j)
	return pretty.PrettyOptions(in, defaultOptions), err
}

func getFileType(filename string) string {
	if strings.HasSuffix(filename, ".yml") || strings.HasSuffix(filename, ".yaml") {
		return "yaml"
	}
	if strings.HasSuffix(filename, ".jsonnet") || strings.HasSuffix(filename, ".libsonnet") {
		return "jsonnet"
	}
	if strings.HasSuffix(filename, ".json") {
		return "json"
	}
	return ""
}

// splitYAMLDocument is a bufio.SplitFunc for splitting YAML streams into individual documents.
// Source: https://github.com/kubernetes/apimachinery/blob/master/pkg/util/yaml/decoder.go
func splitYAMLDocument(data []byte, atEOF bool) (advance int, token []byte, err error) {
	if atEOF && len(data) == 0 {
		return 0, nil, nil
	}
	sep := len([]byte(yamlSeparator))
	if i := bytes.Index(data, []byte(yamlSeparator)); i >= 0 {
		// We have a potential document terminator
		i += sep
		after := data[i:]
		if len(after) == 0 {
			// we can't read any more characters
			if atEOF {
				return len(data), data[:len(data)-sep], nil
			}
			return 0, nil, nil
		}
		if j := bytes.IndexByte(after, '\n'); j >= 0 {
			return i + j + 1, data[0 : i-sep], nil
		}
		return 0, nil, nil
	}
	// If we're at EOF, we have a final, non-terminated line. Return it.
	if atEOF {
		return len(data), data, nil
	}
	// Request more data.
	return 0, nil, nil
}
