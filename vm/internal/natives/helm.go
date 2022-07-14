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

package natives

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/ghodss/yaml"
	"github.com/pkg/errors"
)

// helmOptions are options that can be passed to the helm template command as well
// as a `thisFile` option that the caller needs to set from `std.thisFile` to make
// relative references to charts work correctly.
type helmOptions struct {
	Execute      []string `json:"execute"`      // --execute option
	KubeVersion  string   `json:"kubeVersion"`  // --kube-version option
	Name         string   `json:"name"`         // --name option
	NameTemplate string   `json:"nameTemplate"` // --name-template option
	Namespace    string   `json:"namespace"`    // --namespace option
	ThisFile     string   `json:"thisFile"`     // use supplied file as current file to resolve relative refs, should be set to std.thisFile
	Verbose      bool     `json:"verbose"`      // print helm template command before executing it
	//IsUpgrade    bool     `json:"isUpgrade"` // --is-upgrade option, defer adding this until implications are known,
	ExtraArgs []string `json:"extraArgs"`
}

// toArgs converts options to a slice of command-line args.
func (h helmOptions) toArgs() []string {
	var ret []string
	if len(h.Execute) > 0 {
		for _, e := range h.Execute {
			ret = append(ret, "--execute", e)
		}
	}
	if h.KubeVersion != "" {
		ret = append(ret, "--kube-version", h.KubeVersion)
	}
	if h.Name != "" {
		ret = append(ret, "--name", h.Name)
	}
	if h.NameTemplate != "" {
		ret = append(ret, "--name-template", h.NameTemplate)
	}
	if h.Namespace != "" {
		ret = append(ret, "--namespace", h.Namespace)
	}
	//if h.IsUpgrade {
	//	ret = append(ret, "--is-upgrade")
	//}
	if len(h.ExtraArgs) > 0 {
		for _, a := range h.ExtraArgs {
			ret = append(ret, a)
		}
	}
	return ret
}

// expandHelmTemplate produces an array of objects parsed from the output of running `helm template` with
// the supplied values and helm options.
func expandHelmTemplate(chart string, values map[string]interface{}, options helmOptions) (out []interface{}, finalErr error) {
	// run command from the directory containing current file or the OS temp dir if `thisFile` not specified. That is,
	// explicitly fail to resolve relative refs unless the calling file is specified; don't let them work by happenstance.
	workDir := os.TempDir()
	if options.ThisFile != "" {
		dir := filepath.Dir(options.ThisFile)
		if !filepath.IsAbs(dir) {
			wd, err := os.Getwd()
			if err != nil {
				return nil, errors.Wrap(err, "get working directory")
			}
			dir = filepath.Join(wd, dir)
		}
		workDir = dir
	}

	valueBytes, err := yaml.Marshal(values)
	if err != nil {
		return nil, errors.Wrap(err, "marshal values to YAML")
	}

	args := append([]string{"template", chart}, options.toArgs()...)
	args = append(args, "--values", "-")

	var stdout bytes.Buffer
	cmd := exec.Command("helm", args...)
	cmd.Stdin = bytes.NewBuffer(valueBytes)
	cmd.Stdout = &stdout
	cmd.Stderr = os.Stderr
	cmd.Dir = workDir

	if options.Verbose {
		fmt.Fprintf(os.Stderr, "[helm template] cd %s && helm %s\n", workDir, strings.Join(args, " "))
	}

	if err := cmd.Run(); err != nil {
		if options.ThisFile == "" {
			fmt.Fprintln(os.Stderr, "[WARN] helm template command failed, you may need to set the 'thisFile' option to make relative chart paths work")
		}
		return nil, errors.Wrap(err, "run helm template command")
	}

	return ParseYAMLDocuments(bytes.NewReader(stdout.Bytes()))
}
