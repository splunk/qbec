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
	"encoding/base64"
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
	"github.com/stretchr/testify/require"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/util/yaml"
	"k8s.io/client-go/dynamic"
)

type objectKey struct {
	gvk       schema.GroupVersionKind
	namespace string
	name      string
}

func (o objectKey) GroupVersionKind() schema.GroupVersionKind { return o.gvk }
func (o objectKey) GetKind() string                           { return o.gvk.Kind }
func (o objectKey) GetNamespace() string                      { return o.namespace }
func (o objectKey) GetName() string                           { return o.name }

type basicObject struct {
	objectKey
	app       string
	tag       string
	component string
	env       string
	anns      map[string]string
}

func (b *basicObject) Application() string               { return b.app }
func (b *basicObject) Tag() string                       { return b.tag }
func (b *basicObject) Component() string                 { return b.component }
func (b *basicObject) Environment() string               { return b.env }
func (b *basicObject) GetGenerateName() string           { return "" }
func (b *basicObject) GetAnnotations() map[string]string { return b.anns }

type coll struct {
	data map[objectKey]model.K8sQbecMeta
}

func (c *coll) add(objs ...*basicObject) {
	if c.data == nil {
		c.data = map[objectKey]model.K8sQbecMeta{}
	}
	for _, o := range objs {
		c.data[o.objectKey] = o
	}
}

func (c *coll) Remove(objs []model.K8sQbecMeta) error {
	removeMap := map[objectKey]bool{}
	for _, o := range objs {
		removeMap[objectKey{gvk: o.GroupVersionKind(), namespace: o.GetNamespace(), name: o.GetName()}] = true
	}
	retainedSet := map[objectKey]model.K8sQbecMeta{}
	for k, v := range c.data {
		if !removeMap[k] {
			retainedSet[k] = v
		}
	}
	c.data = retainedSet
	return nil
}

func (c *coll) ToList() []model.K8sQbecMeta {
	var ret []model.K8sQbecMeta
	for _, v := range c.data {
		ret = append(ret, v)
	}
	return ret
}

type client struct {
	nsFunc        func(kind schema.GroupVersionKind) (bool, error)
	getFunc       func(obj model.K8sMeta) (*unstructured.Unstructured, error)
	syncFunc      func(obj model.K8sLocalObject, opts remote.SyncOptions) (*remote.SyncResult, error)
	validatorFunc func(gvk schema.GroupVersionKind) (k8smeta.Validator, error)
	listFunc      func(scope remote.ListQueryConfig) (remote.Collection, error)
	deleteFunc    func(obj model.K8sMeta, opts remote.DeleteOptions) (*remote.SyncResult, error)
	objectKeyFunc func(obj model.K8sMeta) string
}

func (c *client) DisplayName(o model.K8sMeta) string {
	return fmt.Sprintf("%s:%s:%s", o.GetKind(), o.GetNamespace(), model.NameForDisplay(o))
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

func (c *client) ListObjects(scope remote.ListQueryConfig) (remote.Collection, error) {
	if c.listFunc != nil {
		return c.listFunc(scope)
	}
	return nil, errors.New("not implemented")
}

func (c *client) Delete(obj model.K8sMeta, opts remote.DeleteOptions) (*remote.SyncResult, error) {
	if c.deleteFunc != nil {
		return c.deleteFunc(obj, opts)
	}
	return nil, errors.New("not implemented")
}

func (c *client) ObjectKey(obj model.K8sMeta) string {
	if c.objectKeyFunc != nil {
		return c.objectKeyFunc(obj)
	}
	return fmt.Sprintf("%s:%s:%s:%s", obj.GroupVersionKind().Group, obj.GetKind(), obj.GetNamespace(), obj.GetName())
}

func (c *client) ResourceInterface(gvk schema.GroupVersionKind, namespace string) (dynamic.ResourceInterface, error) {
	return nil, fmt.Errorf("not implemented")
}

func setPwd(t *testing.T, dir string) func() {
	wd, err := os.Getwd()
	require.NoError(t, err)
	p, err := filepath.Abs(dir)
	require.NoError(t, err)
	err = os.Chdir(p)
	require.NoError(t, err)
	return func() {
		err = os.Chdir(wd)
		require.NoError(t, err)
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

func (s *scaffold) executeCommand(args ...string) (err error) {
	s.cmd.SetArgs(args)
	defer func() {
		if os.Getenv("QBEC_VERBOSE") != "" {
			l := log.New(os.Stderr, "", 0)
			l.Println("Command:", args)
			l.Println("Stdout:\n" + s.stdout())
			l.Println("Stderr:\n" + s.stderr())
			l.Println("Err:", err)
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

func newCustomScaffold(t *testing.T, dir string) *scaffold {
	if dir == "" {
		dir = "../../examples/test-app"
	}
	reset := setPwd(t, dir)
	out := bytes.NewBuffer(nil)

	c := &client{}
	clientProvider := func(env string) (kubeClient, error) { return c, nil }
	attrsProvider := func(env string) (*remote.KubeAttributes, error) {
		return &remote.KubeAttributes{
			Context:    "foo",
			ConfigFile: "kube.config",
			Namespace:  "my-ns",
			Cluster:    "dev.server.com",
		}, nil
	}
	cp := configFactory{
		stdout:      out,
		skipConfirm: true,
		colors:      false,
	}

	cmd := &cobra.Command{
		Use: "qbec-test",
	}
	doSetup(cmd, cp, clientProvider, attrsProvider)
	cmd.SetOut(out)
	cmd.SetErr(out)
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

func newScaffold(t *testing.T) *scaffold {
	return newCustomScaffold(t, "")
}

type dg struct {
	cmValue     string
	secretValue string
}

func (d *dg) get(obj model.K8sMeta) (*unstructured.Unstructured, error) {
	switch {
	case obj.GetName() == "svc2-cm":
		return &unstructured.Unstructured{
			Object: map[string]interface{}{
				"apiVersion": "v1",
				"kind":       "ConfigMap",
				"metadata": map[string]interface{}{
					"creationTimestamp": "xxx",
					"namespace":         "bar-system",
					"name":              "svc2-cm",
					"annotations": map[string]interface{}{
						"ann/foo": "bar",
						"ann/bar": "baz",
					},
				},
				"data": map[string]interface{}{
					"foo": d.cmValue,
				},
			},
		}, nil
	case obj.GetName() == "svc2-secret":
		return &unstructured.Unstructured{
			Object: map[string]interface{}{
				"apiVersion": "v1",
				"kind":       "Secret",
				"metadata": map[string]interface{}{
					"creationTimestamp": "xxx",
					"namespace":         "bar-system",
					"name":              "svc2-secret",
				},
				"data": map[string]interface{}{
					"foo": base64.StdEncoding.EncodeToString([]byte(d.secretValue)),
				},
			},
		}, nil
	case obj.GetName() == "svc2-previous-deploy":
		return &unstructured.Unstructured{
			Object: map[string]interface{}{
				"apiVersion": "apps/v1",
				"kind":       "Deployment",
				"metadata": map[string]interface{}{
					"creationTimestamp": "xxx",
					"namespace":         "bar-system",
					"name":              "svc2-previous-deploy",
				},
				"spec": map[string]interface{}{
					"foo": "bar",
				},
			},
		}, nil
	default:
		return nil, remote.ErrNotFound
	}

}

func stdLister(_ remote.ListQueryConfig) (remote.Collection, error) {
	c := &coll{}
	c.add(
		&basicObject{
			objectKey: objectKey{
				gvk:       schema.GroupVersionKind{Group: "apps", Version: "v1", Kind: "Deployment"},
				namespace: "bar-system",
				name:      "svc2-deploy",
			},
			component: "service1", // deliberate mismatch
			app:       "app",
			env:       "dev",
		},
		&basicObject{
			objectKey: objectKey{
				gvk:       schema.GroupVersionKind{Group: "apps", Version: "v1", Kind: "Deployment"},
				namespace: "bar-system",
				name:      "svc2-previous-deploy",
			},
			component: "service2",
			app:       "app",
			env:       "dev",
		},
	)
	return c, nil
}
