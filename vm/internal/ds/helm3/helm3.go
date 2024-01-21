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

// Package helm3 provides a data source implementation that can extract k8s objects out of helm3 charts
package helm3

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"time"

	"github.com/iancoleman/strcase"
	"github.com/pkg/errors"
	"github.com/splunk/qbec/internal/sio"
	"github.com/splunk/qbec/vm/datasource"
	"github.com/splunk/qbec/vm/internal/ds"
	"github.com/splunk/qbec/vm/internal/natives"
)

// Scheme is the scheme supported by this data source
const (
	Scheme         = "helm3"
	configVarParam = "config-from"
)

var defaultNamespaceVar = "qbec.io/defaultNs" // TODO: make this blank and have qbec set it

// SetDefaultNamespaceVar sets the name of the external variable that is always set to the default namespace to use.
func SetDefaultNamespaceVar(s string) {
	defaultNamespaceVar = s
}

// Config is the configuration of the data source. TODO: add version check, SHA check options etc.
type Config struct {
	Command string        `json:"command"`           // the executable that is run, default is "helm"
	Timeout string        `json:"timeout,omitempty"` // command timeout as a duration string
	timeout time.Duration // internal representation
}

// TemplateOptions are a subset of command line arguments that can be passed to `helm template`
type TemplateOptions struct {
	Namespace                string   `json:"namespace,omitempty"`
	CreateNamespace          bool     `json:"createNamespace,omitempty"`
	APIVersions              []string `json:"apiVersions,omitempty"`
	DependencyUpdate         bool     `json:"dependencyUpdate,omitempty"`
	Description              string   `json:"description"`
	Devel                    bool     `json:"devel"`
	DisableOpenAPIValidation bool     `json:"disableOpenapiValidation,omitempty"`
	GenerateName             bool     `json:"generateName,omitempty"`
	IncludeCRDs              bool     `json:"includeCrds,omitempty"`
	IsUpgrade                bool     `json:"isUpgrade,omitempty"`
	SkipCRDs                 bool     `json:"skipCrds,omitempty"`
	SkipTests                bool     `json:"skipTests,omitempty"`
	InsecureSkipTLSVerify    bool     `json:"insecureSkipTlsVerify,omitempty"`
	KubeVersion              string   `json:"kubeVersion,omitempty"`
	NameTemplate             string   `json:"nameTemplate,omitempty"`
	NoHooks                  bool     `json:"noHooks,omitempty"`
	PassCredentials          bool     `json:"passCredentials,omitempty"`
	Password                 string   `json:"password,omitempty" fld:"secure"`
	RenderSubchartNotes      bool     `json:"renderSubchartNotes,omitempty"`
	Replace                  bool     `json:"replace,omitempty"`
	Repo                     string   `json:"repo,omitempty"`
	ShowOnly                 []string `json:"showOnly,omitempty"`
	Username                 string   `json:"username,omitempty"`
	Validate                 bool     `json:"validate,omitempty"`
	Verify                   bool     `json:"verify,omitempty"`
	Version                  string   `json:"version,omitempty"`
	/*
		// first pass: omit all options that refer to files since we need to figure out what the exact
		// semantics of relative paths are and its corresponding implications of qbec root etc.
				  --ca-file string               verify certificates of HTTPS-enabled servers using this CA bundle
				  --cert-file string             identify HTTPS client using this SSL certificate file
				  --key-file string              identify HTTPS client using this SSL key file
				  --keyring string               location of public keys used for verification (default "/Users/kanantheswaran/.gnupg/pubring.gpg")
				  --output-dir string            writes the executed templates to files in output-dir instead of stdout
				  --post-renderer postrenderer   the path to an executable to be used for post rendering. If it exists in $PATH, the binary will be used, otherwise it will try to look for the executable at the given path (default exec)
				  --release-name                 use release name in the output-dir path.
		// ignore values related stuff since we send JSON object via stdin
				  --set stringArray              set values on the command line (can specify multiple or separate values with commas: key1=val1,key2=val2)
				  --set-file stringArray         set values from respective files specified via the command line (can specify multiple or separate values with commas: key1=path1,key2=path2)
				  --set-string stringArray       set STRING values on the command line (can specify multiple or separate values with commas: key1=val1,key2=val2)
				  -f, --values strings               specify values in a YAML file or a URL (can specify multiple)
		// values not relevant for template expansion
				  --timeout duration             time to wait for any individual Kubernetes operation (like Jobs for hooks) (default 5m0s)
				  --wait                         if set, will wait until all Pods, PVCs, Services, and minimum number of Pods of a Deployment, StatefulSet, or ReplicaSet are in a ready state before marking the release as successful. It will wait for as long as --timeout
				  --wait-for-jobs                if set and --wait enabled, will wait until all Jobs have been completed before marking the release as successful. It will wait for as long as --timeout
		Global Flags:
		// we always set debug for better error messages, don't expose
			  --debug  enable verbose output
		// we _could_ set all the kube options based on what we know but this requires a dance between qbec knowledge and data source knowledge
			  --kube-apiserver string       the address and the port for the Kubernetes API server
			  --kube-as-group stringArray   group to impersonate for the operation, this flag can be repeated to specify multiple groups.
			  --kube-as-user string         username to impersonate for the operation
			  --kube-ca-file string         the certificate authority file for the Kubernetes API server connection
			  --kube-context string         name of the kubeconfig context to use
			  --kube-token string           bearer token used for authentication
			  --kubeconfig string           path to the kubeconfig file
		// more files to ignore for now
			  --registry-config string      path to the registry config file (default "")
			  --repository-cache string     path to the file containing cached repository
			  --repository-config string    path to the file containing repository names and URLs
	*/
}

func (o TemplateOptions) toInternalCommandLine(display bool) []string {
	t := reflect.TypeOf(o)
	v := reflect.ValueOf(o)
	var ret []string
	for i := 0; i < t.NumField(); i++ {
		fld := t.Field(i)
		val := v.FieldByName(fld.Name)
		tag := strings.Split(fld.Tag.Get("json"), ",")[0]
		secure := strings.Contains(fld.Tag.Get("fld"), "secure")
		option := fmt.Sprintf("--%s", strcase.ToKebab(tag))
		kv := func(v interface{}) {
			if secure && display {
				v = "<REDACTED>"
			}
			ret = append(ret, fmt.Sprintf("%s=%s", option, v))
		}
		switch {
		case fld.Type.Name() == "string":
			out := val.String()
			if out != "" {
				kv(out)
			}
		case fld.Type.Name() == "bool":
			out := val.Bool()
			if out {
				ret = append(ret, option)
			}
		case fld.Type.Kind() == reflect.Slice && fld.Type.Elem().Name() == "string":
			for j := 0; j < val.Len(); j++ {
				kv(val.Index(j).String())
			}
		default:
			panic("unsupported field type for field " + fld.Name)
		}
	}
	return ret
}

func (o TemplateOptions) toCommandLine() []string {
	return o.toInternalCommandLine(false)
}

func (o TemplateOptions) toDisplay() string {
	parts := o.toInternalCommandLine(true)
	return strings.Join(parts, " ") // TODO: improve me for quoting
}

// TemplateConfig is the configuration for the template command that includes options and values.
// TODO: make a schema for this and validate inputs before JSON unmarshal
type TemplateConfig struct {
	Name    string                 `json:"name,omitempty"`
	Options TemplateOptions        `json:"options,omitempty"`
	Values  map[string]interface{} `json:"values,omitempty"`
}

func findExecutable(cmd string) (string, error) {
	if !filepath.IsAbs(cmd) {
		p, err := filepath.Abs(cmd)
		if err == nil {
			stat, err := os.Stat(cmd)
			if err == nil {
				if m := stat.Mode(); !m.IsDir() && m&0111 != 0 {
					return p, nil
				}
			}
		}
	}
	return exec.LookPath(cmd)
}

func (c *Config) assertValid() error {
	if c.Command == "" {
		return fmt.Errorf("command not specified")
	}
	if c.Timeout != "" {
		t, err := time.ParseDuration(c.Timeout)
		if err != nil {
			return fmt.Errorf("invalid timeout '%s': %v", c.Timeout, err)
		}
		c.timeout = t
	}
	exe, err := findExecutable(c.Command)
	if err != nil {
		return fmt.Errorf("invalid command '%s': %v", c.Command, err)
	}
	c.Command = exe
	// TODO: support version/ SHA  checking etc.
	return nil
}

func (c *Config) initDefaults() {
	if c.Command == "" {
		c.Command = "helm"
	}
	if c.timeout == 0 {
		c.timeout = time.Minute
	}
}

type helm3Source struct {
	name      string
	configVar string
	cp        datasource.ConfigProvider
	config    Config
}

// New creates a new helm3 data source
func New(name string, configVar string) ds.DataSourceWithLifecycle {
	return &helm3Source{
		name:      name,
		configVar: configVar,
	}
}

// Name implements the interface method
func (d *helm3Source) Name() string {
	return d.name
}

// Init implements the interface method.
func (d *helm3Source) Init(p datasource.ConfigProvider) (fErr error) {
	defer func() {
		fErr = errors.Wrapf(fErr, "init data source %s", d.name) // nil wraps as nil
	}()
	cfgJSON, err := p(d.configVar)
	if err != nil {
		return err
	}
	var c Config
	err = json.Unmarshal([]byte(cfgJSON), &c)
	if err != nil {
		return err
	}
	c.initDefaults()
	err = c.assertValid()
	if err != nil {
		return err
	}
	d.cp = p
	d.config = c
	return nil
}

// Resolve implements the interface method.
func (d *helm3Source) Resolve(path string) (_ string, finalErr error) {
	u, err := url.Parse(path)
	if err != nil {
		return "", errors.Wrapf(err, "parse path %q", path)
	}
	configVar := u.Query().Get(configVarParam)
	if configVar == "" {
		return "", fmt.Errorf("%s query param not set in data source path %q", configVarParam, path)
	}
	u.Query().Del(configVarParam)
	str, err := d.cp(configVar)
	if err != nil {
		return "", errors.Wrapf(err, "get ext code variable %s", configVar)
	}
	var tc TemplateConfig
	err = json.Unmarshal([]byte(str), &tc)
	if err != nil {
		return "", errors.Wrapf(err, "json unmarshal of %s value", configVar)
	}
	if tc.Options.Namespace == "" {
		if defaultNamespaceVar == "" {
			return "", fmt.Errorf("namespace option not specified and no default value exists")
		}
		ns, err := d.cp(defaultNamespaceVar)
		if err != nil {
			return "", errors.Wrapf(err, "get default namespace from %s", defaultNamespaceVar)
		}
		tc.Options.Namespace = ns
	}
	out, err := d.runTemplate(u, tc)
	if err != nil {
		return "", err
	}
	b, err := json.Marshal(out)
	if err != nil {
		return "", errors.Wrap(err, "marshal output")
	}
	return string(b), nil
}

func (d *helm3Source) runTemplate(u *url.URL, tc TemplateConfig) (interface{}, error) {
	path := strings.TrimPrefix(u.Path, "/")
	chart := path
	if tc.Options.Repo == "" { // then assume path is a URL with https scheme
		parts := strings.SplitN(path, "/", 2) // first component of part is actually the host
		if len(parts) == 1 {
			return nil, fmt.Errorf("unable to extract host and path from %s", path)
		}
		u.Host = parts[0]
		u.Path = "/" + parts[1]
		u.Scheme = "https"
		chart = u.String()
	}
	b, err := json.Marshal(tc.Values)
	if err != nil {
		return "", errors.Wrap(err, "marshal values")
	}
	args := append([]string{"template", "--debug"}, tc.Options.toCommandLine()...)
	if tc.Name != "" { // TODO: figure out if omitting name is the correct strategy
		args = append(args, tc.Name)
	}
	args = append(args, chart)
	args = append(args, "--values", "-")

	ctx, cancel := context.WithTimeout(context.Background(), d.config.timeout)
	defer cancel()

	displayCommand := fmt.Sprintf("%s template %s %s %s", d.config.Command, tc.Options.toDisplay(), tc.Name, chart)
	sio.Debugln(displayCommand)

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	cmd := exec.CommandContext(ctx, d.config.Command, args...)
	cmd.Stdin = bytes.NewBuffer(b)
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	err = cmd.Run()
	if err != nil {
		sio.Warnf("%s\n%s\n", "debug output from helm", stdout.String())
		return "", fmt.Errorf("%s\n%s", err.Error(), stderr.String())
	}
	docs, err := natives.ParseYAMLDocuments(bytes.NewReader(stdout.Bytes()))
	if err != nil {
		return "", err
	}
	return docs, nil
}

// Close implements the interface method.
func (d *helm3Source) Close() error {
	return nil
}
