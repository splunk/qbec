// Copyright 2021 Splunk Inc.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

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
	"github.com/tidwall/pretty"
	"gopkg.in/yaml.v3"
)

func format(in []byte, filename string) ([]byte, error) {
	if getFileType(filename) == "yaml" {
		return formatYaml(in)
	}
	if getFileType(filename) == "jsonnet" {
		return formatJsonnet(in)
	}
	if getFileType(filename) == "json" {
		return formatJSON(in)
	}
	return nil, fmt.Errorf("unknown file type for file %q", filename)
}

func isYamlFile(f os.FileInfo) bool {
	name := f.Name()
	return !f.IsDir() && !strings.HasPrefix(name, ".") && getFileType(name) == "yaml"
}

func isJsonnetFile(f os.FileInfo) bool {
	name := f.Name()
	return !f.IsDir() && !strings.HasPrefix(name, ".") && getFileType(name) == "jsonnet"
}

func isJSONFile(f os.FileInfo) bool {
	name := f.Name()
	return !f.IsDir() && !strings.HasPrefix(name, ".") && getFileType(name) == "json"
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

func formatJsonnet(in []byte) ([]byte, error) {
	var ret, err = formatter.Format("", string(in), formatter.DefaultOptions())
	if err != nil {
		return nil, err
	}
	return []byte(ret), nil
}

func formatJSON(in []byte) ([]byte, error) {
	var j interface{}
	decoder := json.NewDecoder(bytes.NewReader(in))
	decoder.UseNumber()
	defaultOptions := pretty.DefaultOptions
	// Make array values to spread across lines
	defaultOptions.Width = -1
	//Validate input json
	var err = decoder.Decode(&j)
	return pretty.PrettyOptions(in, defaultOptions), err
}

const separator = "---\n"
const yamlSeparator = "\n---"

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
