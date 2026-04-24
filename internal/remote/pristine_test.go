// Copyright 2025 Splunk Inc.
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

package remote

import (
	"encoding/base64"
	"io/ioutil"
	"path/filepath"
	"testing"

	"github.com/splunk/qbec/internal/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"sigs.k8s.io/yaml"
)

func TestZipRoundTrip(t *testing.T) {
	text := `
Lorem ipsum dolor sit amet, consectetur adipiscing elit, sed do eiusmod tempor incididunt ut labore et dolore magna 
aliqua. Tellus pellentesque eu tincidunt tortor aliquam nulla facilisi cras fermentum. Vitae tortor condimentum lacinia 
quis vel eros donec. Sem viverra aliquet eget sit amet tellus cras. Faucibus nisl tincidunt eget nullam non nisi est. 
Amet massa vitae tortor condimentum lacinia quis vel eros donec. In nulla posuere sollicitudin aliquam ultrices. 
Dolor morbi non arcu risus. Netus et malesuada fames ac turpis egestas. Sit amet consectetur adipiscing elit 
pellentesque habitant morbi tristique. Vestibulum morbi blandit cursus risus at ultrices. Integer vitae justo eget 
magna. Lectus nulla at volutpat diam ut venenatis tellus in metus. Tellus pellentesque eu tincidunt tortor aliquam 
nulla facilisi. Blandit libero volutpat sed cras ornare arcu. Turpis nunc eget lorem dolor sed viverra ipsum nunc 
aliquet. Et sollicitudin ac orci phasellus egestas tellus rutrum tellus pellentesque. Nascetur ridiculus mus mauris 
vitae ultricies leo integer malesuada. Quam adipiscing vitae proin sagittis. Faucibus interdum posuere lorem ipsum.
`
	data := map[string]interface{}{
		"foo":  "bar",
		"text": text,
	}
	zipped, err := zipData(data)
	require.Nil(t, err)
	ret, err := unzipData(zipped)
	require.Nil(t, err)
	a := assert.New(t)
	a.Equal(2, len(ret))
	a.Equal("bar", ret["foo"])
	a.Equal(text, ret["text"])
}

func TestUnzipNegative(t *testing.T) {
	a := assert.New(t)

	_, err := unzipData("xxx")
	require.NotNil(t, err)
	a.Contains(err.Error(), "illegal base64 data")

	_, err = unzipData(base64.StdEncoding.EncodeToString([]byte("xxx")))
	require.NotNil(t, err)
	a.Contains(err.Error(), "unexpected EOF")
}

func loadFile(t *testing.T, file string) *unstructured.Unstructured {
	b, err := ioutil.ReadFile(filepath.Join("testdata", "pristine", file))
	require.Nil(t, err)
	var data map[string]interface{}
	err = yaml.Unmarshal(b, &data)
	require.Nil(t, err)
	return &unstructured.Unstructured{Object: data}
}

func testPristineReader(t *testing.T, useFallback bool) {
	tests := []struct {
		name     string
		file     string
		mod      func(obj *unstructured.Unstructured)
		asserter func(t *testing.T, obj *unstructured.Unstructured, source string)
	}{
		{
			file: "input.yaml",
			asserter: func(t *testing.T, obj *unstructured.Unstructured, source string) {
				a := assert.New(t)
				if !useFallback {
					a.Equal("", source)
					a.Nil(obj)
					return
				}
				a.Equal("fallback - live object with some attributes removed", source)
				a.NotNil(obj)

			},
		},
		{
			file: "kc-created.yaml",
			asserter: func(t *testing.T, obj *unstructured.Unstructured, source string) {
				a := assert.New(t)
				if !useFallback {
					a.Equal("", source)
					a.Nil(obj)
					return
				}
				a.Equal("fallback - live object with some attributes removed", source)
				a.NotNil(obj)
				a.Equal("", obj.GetResourceVersion())
				a.Equal("", obj.GetSelfLink())
				a.EqualValues("", obj.GetUID())
				a.EqualValues(0, obj.GetGeneration())
				a.Nil(obj.Object["status"])
			},
		},
		{
			file: "kc-applied.yaml",
			asserter: func(t *testing.T, obj *unstructured.Unstructured, source string) {
				a := assert.New(t)
				a.Equal("kubectl annotation", source)
				a.NotNil(obj)
				a.NotEqual("", obj.GetAnnotations()[kubectlLastConfig])
			},
		},
		{
			name: "kc-bad.yaml",
			file: "kc-applied.yaml",
			mod: func(obj *unstructured.Unstructured) {
				anns := obj.GetAnnotations()
				anns[kubectlLastConfig] += "XXX"
				obj.SetAnnotations(anns)
			},
			asserter: func(t *testing.T, obj *unstructured.Unstructured, source string) {
				a := assert.New(t)
				if !useFallback {
					a.Equal("", source)
					a.Nil(obj)
					return
				}
				a.Equal("fallback - live object with some attributes removed", source)
				a.NotNil(obj)
			},
		},
		{
			file: "qbec-applied.yaml",
			asserter: func(t *testing.T, obj *unstructured.Unstructured, source string) {
				a := assert.New(t)
				a.Equal("qbec annotation", source)
				a.NotNil(obj)
				a.Equal("", obj.GetAnnotations()[kubectlLastConfig])
				a.Equal("", obj.GetAnnotations()[model.QbecNames.PristineAnnotation])
			},
		},
		{
			name: "qbec-bad.yaml",
			file: "qbec-applied.yaml",
			mod: func(obj *unstructured.Unstructured) {
				anns := obj.GetAnnotations()
				anns[model.QbecNames.PristineAnnotation] += "XXX"
				obj.SetAnnotations(anns)
			},
			asserter: func(t *testing.T, obj *unstructured.Unstructured, source string) {
				a := assert.New(t)
				if !useFallback {
					a.Equal("", source)
					a.Nil(obj)
					return
				}
				a.Equal("fallback - live object with some attributes removed", source)
				a.NotNil(obj)
			},
		},
	}

	for _, test := range tests {
		name := test.name
		if name == "" {
			name = test.file
		}
		t.Run(name, func(t *testing.T) {
			obj := loadFile(t, test.file)
			if test.mod != nil {
				test.mod(obj)
			}
			var pristine *unstructured.Unstructured
			var source string
			if useFallback {
				pristine, source = GetPristineVersionForDiff(obj)
			} else {
				pristine, source = getPristineVersion(obj, false)
			}
			test.asserter(t, pristine, source)
		})
	}
}

func TestPristineReaderFallback(t *testing.T) {
	testPristineReader(t, true)
}

func TestPristineReaderNoFallback(t *testing.T) {
	testPristineReader(t, false)
}

func TestCreateFromPristine(t *testing.T) {
	un := loadFile(t, "input.yaml")
	p := qbecPristine{}
	obj := model.NewK8sLocalObject(un.Object, model.LocalAttrs{App: "app", Tag: "", Component: "comp1", Env: "dev"})
	ret, err := p.createFromPristine(obj)
	require.Nil(t, err)
	a := assert.New(t)
	eLabels := obj.ToUnstructured().GetLabels()
	aLabels := ret.ToUnstructured().GetLabels()
	a.EqualValues(eLabels, aLabels)
	eAnnotations := obj.ToUnstructured().GetAnnotations()
	aAnnotations := ret.ToUnstructured().GetAnnotations()
	pValue := aAnnotations[model.QbecNames.PristineAnnotation]
	delete(aAnnotations, model.QbecNames.PristineAnnotation)
	a.EqualValues(eAnnotations, aAnnotations)
	pObj, err := unzipData(pValue)
	require.Nil(t, err)
	a.EqualValues(un.Object, pObj)
}

func TestPristineReaderManagedFields(t *testing.T) {
	obj := newConfigMapWithData("default", "ssa-config", map[string]interface{}{
		"foo":   "bar",
		"stale": "remove-me",
	}).ToUnstructured()
	obj.SetManagedFields([]metav1.ManagedFieldsEntry{
		{
			Manager:    ssaFieldManager,
			Operation:  metav1.ManagedFieldsOperationApply,
			FieldsType: "FieldsV1",
			FieldsV1: &metav1.FieldsV1{
				Raw: []byte(`{"f:data":{"f:foo":{},"f:stale":{}}}`),
			},
		},
	})

	pristine, source := getPristineVersion(obj, false)
	require.NotNil(t, pristine)
	assert.Equal(t, "managed fields", source)
	assert.Equal(t, "ssa-config", pristine.GetName())
	assert.Empty(t, pristine.GetNamespace())
	data, found, err := unstructured.NestedStringMap(pristine.Object, "data")
	require.NoError(t, err)
	require.True(t, found)
	assert.Equal(t, "bar", data["foo"])
	assert.Equal(t, "remove-me", data["stale"])
}

func TestManagedFieldsPristinePreservesIdentityWhenProjectingMetadata(t *testing.T) {
	desired := stripApplyHistoryAnnotations(newConfigMap("default", "ssa-config"))
	annotations := desired.ToUnstructured().GetAnnotations()
	annotations[model.QbecNames.Directives.ApplyStrategy] = string(model.ApplyStrategyServer)
	desired.ToUnstructured().SetAnnotations(annotations)

	serverObj := desired.ToUnstructured().DeepCopy()
	serverObj.SetManagedFields([]metav1.ManagedFieldsEntry{
		{
			Manager:    ssaFieldManager,
			Operation:  metav1.ManagedFieldsOperationApply,
			FieldsType: "FieldsV1",
			FieldsV1: &metav1.FieldsV1{
				Raw: []byte(`{"f:data":{"f:foo":{}},"f:metadata":{"f:annotations":{"f:directives.qbec.io~1apply-strategy":{},"f:qbec.io~1component":{}},"f:labels":{"f:qbec.io~1application":{},"f:qbec.io~1environment":{}}}}`),
			},
		},
	})

	pristine, err := pristineFromManagedFields(serverObj, ssaFieldManager)
	require.NoError(t, err)
	require.NotNil(t, pristine)
	assert.Equal(t, "ssa-config", pristine.GetName())
	assert.Equal(t, "ConfigMap", pristine.GetKind())
	assert.Equal(t, "v1", pristine.GetAPIVersion())
	assert.Empty(t, pristine.GetNamespace())
	assert.Equal(t, "comp", pristine.GetAnnotations()[model.QbecNames.ComponentAnnotation])
	assert.Equal(t, string(model.ApplyStrategyServer), pristine.GetAnnotations()[model.QbecNames.Directives.ApplyStrategy])
	assert.Equal(t, "app", pristine.GetLabels()[model.QbecNames.ApplicationLabel])
	assert.Equal(t, "env", pristine.GetLabels()[model.QbecNames.EnvironmentLabel])

	p := patcher{cfgProvider: pristineBytesForClientSideApply}
	result, err := p.getPatchContents(serverObj, desired)
	require.NoError(t, err)
	assert.Equal(t, identicalObjects, result.SkipReason)
}

func TestClientSideApplyUsesManagedFieldsPristineForDeletion(t *testing.T) {
	serverObj := newConfigMapWithData("default", "ssa-config", map[string]interface{}{
		"foo":   "bar",
		"stale": "remove-me",
	}).ToUnstructured()
	serverObj.SetManagedFields([]metav1.ManagedFieldsEntry{
		{
			Manager:    ssaFieldManager,
			Operation:  metav1.ManagedFieldsOperationApply,
			FieldsType: "FieldsV1",
			FieldsV1: &metav1.FieldsV1{
				Raw: []byte(`{"f:data":{"f:foo":{},"f:stale":{}}}`),
			},
		},
	})

	p := patcher{cfgProvider: pristineBytesForClientSideApply}
	result, err := p.getPatchContents(serverObj, newConfigMap("default", "ssa-config"))
	require.NoError(t, err)
	assert.Empty(t, result.SkipReason)
	assert.Contains(t, string(result.patch), `"stale":null`)
}

func TestClientSideApplyUsesManagedFieldsPristineForMetadataDeletion(t *testing.T) {
	desired := newConfigMap("default", "ssa-config")
	annotations := desired.ToUnstructured().GetAnnotations()
	delete(annotations, model.QbecNames.ComponentAnnotation)
	desired.ToUnstructured().SetAnnotations(annotations)

	serverObj := newConfigMap("default", "ssa-config").ToUnstructured()
	serverObj.SetManagedFields([]metav1.ManagedFieldsEntry{
		{
			Manager:    ssaFieldManager,
			Operation:  metav1.ManagedFieldsOperationApply,
			FieldsType: "FieldsV1",
			FieldsV1: &metav1.FieldsV1{
				Raw: []byte(`{"f:data":{"f:foo":{}},"f:metadata":{"f:annotations":{"f:qbec.io~1component":{}},"f:labels":{"f:qbec.io~1application":{},"f:qbec.io~1environment":{}}}}`),
			},
		},
	})

	p := patcher{cfgProvider: pristineBytesForClientSideApply}
	result, err := p.getPatchContents(serverObj, desired)
	require.NoError(t, err)
	assert.Empty(t, result.SkipReason)
	assert.Contains(t, string(result.patch), `"qbec.io/component":null`)

	pristineBytes, err := pristineBytesForClientSideApply(serverObj)
	require.NoError(t, err)
	assert.Contains(t, string(pristineBytes), `"qbec.io/component"`)
}

func TestClientSideApplyUsesManagedFieldsPristineForDirectiveDeletion(t *testing.T) {
	desired := newConfigMap("default", "ssa-config")
	annotations := desired.ToUnstructured().GetAnnotations()
	delete(annotations, model.QbecNames.Directives.ApplyStrategy)
	desired.ToUnstructured().SetAnnotations(annotations)

	serverObj := newConfigMap("default", "ssa-config").ToUnstructured()
	annotations = serverObj.GetAnnotations()
	annotations[model.QbecNames.Directives.ApplyStrategy] = string(model.ApplyStrategyServer)
	serverObj.SetAnnotations(annotations)
	serverObj.SetManagedFields([]metav1.ManagedFieldsEntry{
		{
			Manager:    ssaFieldManager,
			Operation:  metav1.ManagedFieldsOperationApply,
			FieldsType: "FieldsV1",
			FieldsV1: &metav1.FieldsV1{
				Raw: []byte(`{"f:data":{"f:foo":{}},"f:metadata":{"f:annotations":{"f:qbec.io~1component":{}},"f:labels":{"f:qbec.io~1application":{},"f:qbec.io~1environment":{}}}}`),
			},
		},
	})

	p := patcher{cfgProvider: pristineBytesForClientSideApply}
	result, err := p.getPatchContents(serverObj, desired)
	require.NoError(t, err)
	assert.Empty(t, result.SkipReason)
	assert.Contains(t, string(result.patch), `"directives.qbec.io/apply-strategy":null`)

	pristineBytes, err := pristineBytesForClientSideApply(serverObj)
	require.NoError(t, err)
	assert.Contains(t, string(pristineBytes), `"directives.qbec.io/apply-strategy":"server"`)
}

func TestClientSideApplyPristineSkipsInClusterDirectives(t *testing.T) {
	desired := newConfigMap("default", "ssa-config")
	serverObj := desired.ToUnstructured().DeepCopy()
	annotations := serverObj.GetAnnotations()
	annotations[model.QbecNames.Directives.DeletePolicy] = "never"
	annotations[model.QbecNames.Directives.UpdatePolicy] = "never"
	serverObj.SetAnnotations(annotations)
	serverObj.SetManagedFields([]metav1.ManagedFieldsEntry{
		{
			Manager:    ssaFieldManager,
			Operation:  metav1.ManagedFieldsOperationApply,
			FieldsType: "FieldsV1",
			FieldsV1: &metav1.FieldsV1{
				Raw: []byte(`{"f:data":{"f:foo":{}}}`),
			},
		},
	})

	pristineBytes, err := pristineBytesForClientSideApply(serverObj)
	require.NoError(t, err)
	assert.NotContains(t, string(pristineBytes), model.QbecNames.Directives.DeletePolicy)
	assert.NotContains(t, string(pristineBytes), model.QbecNames.Directives.UpdatePolicy)

	p := patcher{cfgProvider: pristineBytesForClientSideApply}
	result, err := p.getPatchContents(serverObj, desired)
	require.NoError(t, err)
	assert.NotContains(t, string(result.patch), `"directives.qbec.io/delete-policy":null`)
	assert.NotContains(t, string(result.patch), `"directives.qbec.io/update-policy":null`)
}

func TestClientSideApplySyntheticPristineOmitsLiveNamespace(t *testing.T) {
	tests := []struct {
		name string
		mod  func(*unstructured.Unstructured)
	}{
		{
			name: "without managed fields",
		},
		{
			name: "from managed fields",
			mod: func(serverObj *unstructured.Unstructured) {
				serverObj.SetManagedFields([]metav1.ManagedFieldsEntry{
					{
						Manager:    ssaFieldManager,
						Operation:  metav1.ManagedFieldsOperationApply,
						FieldsType: "FieldsV1",
						FieldsV1: &metav1.FieldsV1{
							Raw: []byte(`{"f:data":{"f:foo":{}}}`),
						},
					},
				})
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			desired := newConfigMapWithoutNamespace("ssa-config")
			serverObj := newConfigMap("default", "ssa-config").ToUnstructured()
			if test.mod != nil {
				test.mod(serverObj)
			}

			pristineBytes, err := pristineBytesForClientSideApply(serverObj)
			require.NoError(t, err)
			assert.NotContains(t, string(pristineBytes), `"namespace"`)

			p := patcher{cfgProvider: pristineBytesForClientSideApply}
			result, err := p.getPatchContents(serverObj, desired)
			require.NoError(t, err)
			assert.Equal(t, identicalObjects, result.SkipReason)
			assert.NotContains(t, string(result.patch), `"namespace":null`)
		})
	}
}

func TestManagedFieldsPristinePreservesAssociativeListKeys(t *testing.T) {
	desired := newDeployment("default", "ssa-deployment")
	serverObj := desired.ToUnstructured().DeepCopy()
	serverObj.SetManagedFields([]metav1.ManagedFieldsEntry{
		{
			Manager:    ssaFieldManager,
			Operation:  metav1.ManagedFieldsOperationApply,
			FieldsType: "FieldsV1",
			FieldsV1: &metav1.FieldsV1{
				Raw: []byte(`{"f:spec":{"f:template":{"f:spec":{"f:containers":{"k:{\"name\":\"app\"}":{"f:image":{}}}}}}}`),
			},
		},
	})

	pristine, err := pristineFromManagedFields(serverObj, ssaFieldManager)
	require.NoError(t, err)
	containers, found, err := unstructured.NestedSlice(pristine.Object, "spec", "template", "spec", "containers")
	require.NoError(t, err)
	require.True(t, found)
	require.Len(t, containers, 1)
	container, ok := containers[0].(map[string]interface{})
	require.True(t, ok)
	assert.Equal(t, "app", container["name"])
	assert.Equal(t, "nginx", container["image"])

	p := patcher{cfgProvider: pristineBytesForClientSideApply}
	_, err = p.getPatchContents(serverObj, desired)
	require.NoError(t, err)
}

func TestClientServerClientRoundTripNoop(t *testing.T) {
	desired := newConfigMap("default", "ssa-config")
	clientApplied, err := qbecPristine{}.createFromPristine(desired)
	require.NoError(t, err)

	serverObj := desired.ToUnstructured()
	serverObj.SetManagedFields([]metav1.ManagedFieldsEntry{
		{
			Manager:    ssaFieldManager,
			Operation:  metav1.ManagedFieldsOperationApply,
			FieldsType: "FieldsV1",
			FieldsV1: &metav1.FieldsV1{
				Raw: []byte(`{"f:data":{"f:foo":{}},"f:metadata":{"f:annotations":{"f:qbec.io~1component":{}},"f:labels":{"f:qbec.io~1application":{},"f:qbec.io~1environment":{}}}}`),
			},
		},
	})

	sanitized := stripApplyHistoryAnnotations(clientApplied)
	assert.NotContains(t, sanitized.GetAnnotations(), model.QbecNames.PristineAnnotation)
	assert.NotContains(t, sanitized.GetAnnotations(), kubectlLastConfig)

	p := patcher{cfgProvider: pristineBytesForClientSideApply}
	result, err := p.getPatchContents(serverObj, desired)
	require.NoError(t, err)
	assert.Equal(t, identicalObjects, result.SkipReason)

	pristineBytes, err := pristineBytesForClientSideApply(serverObj)
	require.NoError(t, err)
	assert.NotContains(t, string(pristineBytes), model.QbecNames.PristineAnnotation)
}

func newDeployment(namespace, name string) model.K8sLocalObject {
	return model.NewK8sLocalObject(map[string]interface{}{
		"apiVersion": "apps/v1",
		"kind":       "Deployment",
		"metadata": map[string]interface{}{
			"name":      name,
			"namespace": namespace,
		},
		"spec": map[string]interface{}{
			"selector": map[string]interface{}{
				"matchLabels": map[string]interface{}{
					"app": "demo",
				},
			},
			"template": map[string]interface{}{
				"metadata": map[string]interface{}{
					"labels": map[string]interface{}{
						"app": "demo",
					},
				},
				"spec": map[string]interface{}{
					"containers": []interface{}{
						map[string]interface{}{
							"name":  "app",
							"image": "nginx",
						},
					},
				},
			},
		},
	}, model.LocalAttrs{App: "app", Component: "comp", Env: "env"})
}
