/*
   Copyright 2019 Splunk Inc.

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

package commands

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	yamllib "github.com/ghodss/yaml"
	"github.com/pkg/errors"
	"github.com/spf13/cobra"
	"github.com/splunk/qbec/internal/model"
	"github.com/splunk/qbec/internal/remote"
	"github.com/splunk/qbec/internal/remote/k8smeta"
	"github.com/splunk/qbec/internal/sio"
	"github.com/splunk/qbec/internal/vm"
	"github.com/stretchr/testify/require"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/util/yaml"
)

type client struct {
	nsFunc        func(kind schema.GroupVersionKind) (bool, error)
	getFunc       func(obj model.K8sMeta) (*unstructured.Unstructured, error)
	syncFunc      func(obj model.K8sLocalObject, opts remote.SyncOptions) (*remote.SyncResult, error)
	validatorFunc func(gvk schema.GroupVersionKind) (k8smeta.Validator, error)
	listExtraFunc func(ignore []model.K8sQbecMeta, scope remote.ListQueryConfig) ([]model.K8sQbecMeta, error)
	deleteFunc    func(obj model.K8sMeta, dryRun bool) (*remote.SyncResult, error)
	objectKeyFunc func(obj model.K8sMeta) string
}

func (c *client) DisplayName(o model.K8sMeta) string {
	return fmt.Sprintf("%s:%s:%s", o.GetKind(), o.GetNamespace(), o.GetName())
}

func (c *client) IsNamespaced(kind schema.GroupVersionKind) (bool, error) {
	if c.nsFunc != nil {
		return c.nsFunc(kind)
	}
	if kind.Kind == "PodSecurityPolicy" || kind.Kind == "Namespace" {
		return false, nil
	}
	return true, nil
}

func (c *client) Get(obj model.K8sMeta) (*unstructured.Unstructured, error) {
	if c.getFunc != nil {
		return c.getFunc(obj)
	}
	return nil, errors.New("not implemented")
}

func (c *client) Sync(obj model.K8sLocalObject, opts remote.SyncOptions) (*remote.SyncResult, error) {
	if c.syncFunc != nil {
		return c.syncFunc(obj, opts)
	}
	return nil, errors.New("not implemented")
}

func (c *client) ValidatorFor(gvk schema.GroupVersionKind) (k8smeta.Validator, error) {
	if c.validatorFunc != nil {
		return c.validatorFunc(gvk)
	}
	return nil, errors.New("not implemented")
}

func (c *client) ListExtraObjects(ignore []model.K8sQbecMeta, scope remote.ListQueryConfig) ([]model.K8sQbecMeta, error) {
	if c.listExtraFunc != nil {
		return c.listExtraFunc(ignore, scope)
	}
	return nil, errors.New("not implemented")
}

func (c *client) Delete(obj model.K8sMeta, dryRun bool) (*remote.SyncResult, error) {
	if c.deleteFunc != nil {
		return c.deleteFunc(obj, dryRun)
	}
	return nil, errors.New("not implemented")
}

func (c *client) ObjectKey(obj model.K8sMeta) string {
	if c.objectKeyFunc != nil {
		return c.objectKeyFunc(obj)
	}
	return fmt.Sprintf("%s:%s:%s:%s", obj.GetObjectKind().GroupVersionKind().Group, obj.GetKind(), obj.GetNamespace(), obj.GetName())
}

func setPwd(t *testing.T, dir string) func() {
	wd, err := os.Getwd()
	require.Nil(t, err)
	p, err := filepath.Abs(dir)
	require.Nil(t, err)
	err = os.Chdir(p)
	require.Nil(t, err)
	return func() {
		err = os.Chdir(wd)
		require.Nil(t, err)
	}
}

type scaffold struct {
	t          *testing.T
	client     *client
	outCapture *bytes.Buffer
	errCapture *bytes.Buffer
	reset      func()
	cmd        *cobra.Command
}

func (s *scaffold) output() io.Writer {
	return s.outCapture
}

func (s *scaffold) stdout() string {
	return s.outCapture.String()
}

func (s *scaffold) stderr() string {
	return s.errCapture.String()
}

func (s *scaffold) yamlOutput() ([]interface{}, error) {
	var ret []interface{}
	data := s.outCapture.Bytes()
	d := yaml.NewYAMLToJSONDecoder(bytes.NewReader(data))
	for {
		var doc interface{}
		if err := d.Decode(&doc); err != nil {
			if err == io.EOF {
				break
			}
			return nil, err
		}
		ret = append(ret, doc)
	}
	return ret, nil
}

func (s *scaffold) jsonOutput(data interface{}) error {
	return json.Unmarshal(s.outCapture.Bytes(), &data)
}

func (s *scaffold) executeCommand(args ...string) error {
	s.cmd.SetArgs(args)
	defer func() {
		if os.Getenv("QBEC_VERBOSE") != "" {
			l := log.New(os.Stderr, "", 0)
			l.Println("Command:", args)
			l.Println("Output:\n" + s.stdout())
			l.Println("Error:\n" + s.stderr())
		}
	}()
	return s.cmd.Execute()
}

func (s *scaffold) testMatch(str string, r *regexp.Regexp) bool {
	lines := strings.Split(str, "\n")
	for _, l := range lines {
		if r.MatchString(l) {
			return true
		}
	}
	return false
}

func (s *scaffold) assertOutputLineMatch(r *regexp.Regexp) {
	b := s.testMatch(s.stdout(), r)
	if !b {
		s.t.Errorf("[unexpected] no output line matches: %v", r)
	}
}

func (s *scaffold) assertOutputLineNoMatch(r *regexp.Regexp) {
	b := s.testMatch(s.stdout(), r)
	if b {
		s.t.Errorf("[unexpected] output line matches: %v", r)
	}
}

func (s *scaffold) assertErrorLineMatch(r *regexp.Regexp) {
	b := s.testMatch(s.stderr(), r)
	if !b {
		s.t.Errorf("[unexpected] no error line matches: %v", r)
	}
}

func (s *scaffold) assertErrorLineNoMatch(r *regexp.Regexp) {
	b := s.testMatch(s.stderr(), r)
	if b {
		s.t.Errorf("[unexpected] error line matches: %v", r)
	}
}

func (s *scaffold) outputStats() map[string]interface{} {
	out := s.stdout()
	pos := strings.LastIndex(out, "---")
	require.True(s.t, pos >= 0)
	statsStr := out[pos:]
	var data struct {
		Stats map[string]interface{} `json:"stats"`
	}
	err := yamllib.Unmarshal([]byte(statsStr), &data)
	require.Nil(s.t, err)
	return data.Stats
}

func newScaffold(t *testing.T) *scaffold {
	reset := setPwd(t, "../../examples/test-app")
	app, err := model.NewApp("qbec.yaml", "")
	require.Nil(t, err)
	out := bytes.NewBuffer(nil)

	c := &client{}
	clientProvider := func(env string) (Client, error) { return c, nil }

	cp := ConfigFactory{
		Stdout:      out,
		SkipConfirm: true,
		Colors:      false,
	}
	config, err := cp.internalConfig(app, vm.Config{}, clientProvider)
	cmd := &cobra.Command{
		Use: "qbec-test",
	}
	Setup(cmd, func() *Config { return config })
	cmd.SetOutput(out)
	cmd.SilenceErrors = true
	cmd.SilenceUsage = true
	s := &scaffold{
		t:          t,
		client:     c,
		outCapture: out,
		errCapture: bytes.NewBuffer(nil),
		cmd:        cmd,
	}
	oldOut := sio.Output
	oldColors := sio.EnableColors
	sio.Output = s.errCapture
	sio.EnableColors = false
	s.reset = func() {
		reset()
		sio.Output = oldOut
		sio.EnableColors = oldColors
	}
	return s
}
