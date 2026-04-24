// Copyright 2025 Splunk Inc.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package remote

import (
	"context"
	"encoding/json"
	"fmt"
	"testing"

	openapi_v2 "github.com/google/gnostic-models/openapiv2"
	"github.com/splunk/qbec/internal/model"
	"github.com/splunk/qbec/internal/remote/k8smeta"
	qtypes "github.com/splunk/qbec/internal/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	apiTypes "k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/client-go/dynamic"
)

type staticPool struct {
	client dynamic.Interface
}

func (s staticPool) clientForGroupVersionKind(kind schema.GroupVersionKind) (dynamic.Interface, error) {
	return s.client, nil
}

type testDisco struct {
	groups *metav1.APIGroupList
	lists  map[string]*metav1.APIResourceList
}

func (d testDisco) ServerGroups() (*metav1.APIGroupList, error) {
	return d.groups, nil
}

func (d testDisco) ServerResourcesForGroupVersion(groupVersion string) (*metav1.APIResourceList, error) {
	return d.lists[groupVersion], nil
}

func (d testDisco) OpenAPISchema() (*openapi_v2.Document, error) {
	return nil, fmt.Errorf("not implemented")
}

type recorderDynamic struct {
	resource *recorderResource
}

func (r recorderDynamic) Resource(resource schema.GroupVersionResource) dynamic.NamespaceableResourceInterface {
	r.resource.gvr = resource
	return r.resource
}

type recordedPatch struct {
	name    string
	kind    apiTypes.PatchType
	data    []byte
	options metav1.PatchOptions
}

type recorderResource struct {
	gvr           schema.GroupVersionResource
	namespace     string
	patchName     string
	patchType     apiTypes.PatchType
	patchData     []byte
	patchOptions  metav1.PatchOptions
	patchCalls    []recordedPatch
	patchResponse *unstructured.Unstructured
	createObject  *unstructured.Unstructured
}

func (r *recorderResource) Namespace(namespace string) dynamic.ResourceInterface {
	r.namespace = namespace
	return r
}

func (r *recorderResource) Create(ctx context.Context, obj *unstructured.Unstructured, options metav1.CreateOptions, subresources ...string) (*unstructured.Unstructured, error) {
	r.createObject = obj.DeepCopy()
	return obj, nil
}

func (r *recorderResource) Update(ctx context.Context, obj *unstructured.Unstructured, options metav1.UpdateOptions, subresources ...string) (*unstructured.Unstructured, error) {
	panic("unexpected call to Update")
}

func (r *recorderResource) UpdateStatus(ctx context.Context, obj *unstructured.Unstructured, options metav1.UpdateOptions) (*unstructured.Unstructured, error) {
	panic("unexpected call to UpdateStatus")
}

func (r *recorderResource) Delete(ctx context.Context, name string, options metav1.DeleteOptions, subresources ...string) error {
	panic("unexpected call to Delete")
}

func (r *recorderResource) DeleteCollection(ctx context.Context, options metav1.DeleteOptions, listOptions metav1.ListOptions) error {
	panic("unexpected call to DeleteCollection")
}

func (r *recorderResource) Get(ctx context.Context, name string, options metav1.GetOptions, subresources ...string) (*unstructured.Unstructured, error) {
	panic("unexpected call to Get")
}

func (r *recorderResource) List(ctx context.Context, opts metav1.ListOptions) (*unstructured.UnstructuredList, error) {
	panic("unexpected call to List")
}

func (r *recorderResource) Watch(ctx context.Context, opts metav1.ListOptions) (watch.Interface, error) {
	panic("unexpected call to Watch")
}

func (r *recorderResource) Patch(ctx context.Context, name string, pt apiTypes.PatchType, data []byte, options metav1.PatchOptions, subresources ...string) (*unstructured.Unstructured, error) {
	r.patchName = name
	r.patchType = pt
	r.patchData = append([]byte(nil), data...)
	r.patchOptions = *options.DeepCopy()
	r.patchCalls = append(r.patchCalls, recordedPatch{
		name:    name,
		kind:    pt,
		data:    append([]byte(nil), data...),
		options: *options.DeepCopy(),
	})
	if r.patchResponse != nil {
		return r.patchResponse.DeepCopy(), nil
	}
	return &unstructured.Unstructured{}, nil
}

func (r *recorderResource) Apply(ctx context.Context, name string, obj *unstructured.Unstructured, options metav1.ApplyOptions, subresources ...string) (*unstructured.Unstructured, error) {
	panic("unexpected call to Apply")
}

func (r *recorderResource) ApplyStatus(ctx context.Context, name string, obj *unstructured.Unstructured, options metav1.ApplyOptions) (*unstructured.Unstructured, error) {
	panic("unexpected call to ApplyStatus")
}

func newConfigMap(namespace, name string) model.K8sLocalObject {
	return model.NewK8sLocalObject(map[string]interface{}{
		"apiVersion": "v1",
		"kind":       "ConfigMap",
		"metadata": map[string]interface{}{
			"name":      name,
			"namespace": namespace,
		},
		"data": map[string]interface{}{
			"foo": "bar",
		},
	}, model.LocalAttrs{App: "app", Component: "comp", Env: "env"})
}

func newConfigMapWithData(namespace, name string, data map[string]interface{}) model.K8sLocalObject {
	return model.NewK8sLocalObject(map[string]interface{}{
		"apiVersion": "v1",
		"kind":       "ConfigMap",
		"metadata": map[string]interface{}{
			"name":      name,
			"namespace": namespace,
		},
		"data": data,
	}, model.LocalAttrs{App: "app", Component: "comp", Env: "env"})
}

func newConfigMapWithoutNamespace(name string) model.K8sLocalObject {
	obj := newConfigMap("", name)
	unstructured.RemoveNestedField(obj.ToUnstructured().Object, "metadata", "namespace")
	return obj
}

func newGenerateNameConfigMap(namespace, generateName string) model.K8sLocalObject {
	return model.NewK8sLocalObject(map[string]interface{}{
		"apiVersion": "v1",
		"kind":       "ConfigMap",
		"metadata": map[string]interface{}{
			"generateName": generateName,
			"namespace":    namespace,
		},
		"data": map[string]interface{}{
			"foo": "bar",
		},
	}, model.LocalAttrs{App: "app", Component: "comp", Env: "env"})
}

func newSecret(namespace, name, value string) model.K8sLocalObject {
	return model.NewK8sLocalObject(map[string]interface{}{
		"apiVersion": "v1",
		"kind":       "Secret",
		"metadata": map[string]interface{}{
			"name":      name,
			"namespace": namespace,
		},
		"data": map[string]interface{}{
			"token": value,
		},
	}, model.LocalAttrs{App: "app", Component: "comp", Env: "env"})
}

func newServerSideApplyClient(t *testing.T, response *unstructured.Unstructured) (*Client, *recorderResource) {
	t.Helper()
	recorder := &recorderResource{patchResponse: response}
	resources, err := k8smeta.NewResources(testDisco{
		groups: &metav1.APIGroupList{
			Groups: []metav1.APIGroup{
				{
					Name: "",
					Versions: []metav1.GroupVersionForDiscovery{
						{GroupVersion: "v1", Version: "v1"},
					},
					PreferredVersion: metav1.GroupVersionForDiscovery{GroupVersion: "v1", Version: "v1"},
				},
			},
		},
		lists: map[string]*metav1.APIResourceList{
			"v1": {
				GroupVersion: "v1",
				APIResources: []metav1.APIResource{
					{Name: "configmaps", Kind: "ConfigMap", Namespaced: true, Verbs: metav1.Verbs([]string{"create", "delete", "get", "list", "patch"})},
					{Name: "secrets", Kind: "Secret", Namespaced: true, Verbs: metav1.Verbs([]string{"create", "delete", "get", "list", "patch"})},
				},
			},
		},
	}, k8smeta.ResourceOpts{})
	require.NoError(t, err)
	return &Client{
		resources: resources,
		schema:    k8smeta.NewServerSchema(testDisco{}),
		pool:      staticPool{client: recorderDynamic{resource: recorder}},
		defaultNs: "default",
	}, recorder
}

func TestMaybeCreateServerSideApplyUsesPatchOptions(t *testing.T) {
	obj := newConfigMap("default", "ssa-config")
	client, recorder := newServerSideApplyClient(t, obj.ToUnstructured())
	result, err := client.maybeCreate(context.Background(), obj, SyncOptions{
		ApplyStrategy:  model.ApplyStrategyServer,
		ForceConflicts: true,
	})
	require.NoError(t, err)
	assert.Equal(t, opCreate, result.Operation)
	assert.Equal(t, apiTypes.ApplyPatchType, recorder.patchType)
	assert.Equal(t, "ssa-config", recorder.patchName)
	assert.Equal(t, ssaFieldManager, recorder.patchOptions.FieldManager)
	assert.Nil(t, recorder.patchOptions.DryRun)
	require.NotNil(t, recorder.patchOptions.Force)
	assert.True(t, *recorder.patchOptions.Force)
	assert.Equal(t, schema.GroupVersionResource{Version: "v1", Resource: "configmaps"}, recorder.gvr)
	assert.Equal(t, "default", recorder.namespace)
}

func TestMaybeCreateServerSideApplyFallsBackForGenerateName(t *testing.T) {
	client, recorder := newServerSideApplyClient(t, nil)
	obj := newGenerateNameConfigMap("default", "ssa-config-")
	result, err := client.maybeCreate(context.Background(), obj, SyncOptions{
		ApplyStrategy: model.ApplyStrategyServer,
	})
	require.NoError(t, err)
	assert.Equal(t, opCreate, result.Operation)
	assert.Equal(t, "", recorder.patchName)
	require.NotNil(t, recorder.createObject)
	assert.Equal(t, "ssa-config-", recorder.createObject.GetGenerateName())
}

func TestMaybeCreateServerSideApplyDryRunFallsBackToLocalCreate(t *testing.T) {
	client, recorder := newServerSideApplyClient(t, nil)
	obj := newConfigMap("default", "ssa-config")
	result, err := client.maybeCreate(context.Background(), obj, SyncOptions{
		DryRun:        true,
		ApplyStrategy: model.ApplyStrategyServer,
	})
	require.NoError(t, err)
	assert.Equal(t, opCreate, result.Operation)
	assert.Equal(t, "local", result.Source)
	assert.Nil(t, recorder.createObject)
	assert.Empty(t, recorder.patchName)
	assert.Nil(t, recorder.patchData)
}

func TestMaybeUpdateServerSideApplyDetectsIdenticalObjects(t *testing.T) {
	existing := newConfigMap("default", "ssa-config").ToUnstructured()
	client, recorder := newServerSideApplyClient(t, existing.DeepCopy())
	result, err := client.maybeUpdate(context.Background(), newConfigMap("default", "ssa-config"), existing.DeepCopy(), SyncOptions{
		ApplyStrategy:   model.ApplyStrategyServer,
		DisableUpdateFn: func(model.K8sMeta) bool { return false },
	}, internalSyncOptions{})
	require.NoError(t, err)
	assert.Equal(t, identicalObjects, result.SkipReason)
	assert.Equal(t, apiTypes.ApplyPatchType, recorder.patchType)
}

func TestMaybeUpdateSecretDryRunBypassesServerSideApply(t *testing.T) {
	existing := newSecret("default", "ssa-secret", "dmFsdWU=").ToUnstructured()
	client, recorder := newServerSideApplyClient(t, existing.DeepCopy())
	localObj, changed := qtypes.HideSensitiveLocalInfo(newSecret("default", "ssa-secret", "dmFsdWU="))
	require.True(t, changed)
	result, err := client.maybeUpdate(context.Background(), localObj, existing.DeepCopy(), SyncOptions{
		DryRun:        true,
		ApplyStrategy: model.ApplyStrategyServer,
		DisableUpdateFn: func(model.K8sMeta) bool {
			return false
		},
	}, internalSyncOptions{secretDryRun: true})
	require.NoError(t, err)
	assert.NotEqual(t, apiTypes.ApplyPatchType, result.Kind)
	assert.Empty(t, recorder.patchName)
	assert.Nil(t, recorder.patchData)
}

func TestMaybeUpdateServerSideApplyIgnoresControllerOwnedChanges(t *testing.T) {
	existing := newConfigMap("default", "ssa-config").ToUnstructured()
	out := existing.DeepCopy()
	require.NoError(t, unstructured.SetNestedField(out.Object, map[string]interface{}{"state": "ready"}, "status"))
	annotations := out.GetAnnotations()
	if annotations == nil {
		annotations = map[string]string{}
	}
	annotations["controller.example.com/hash"] = "12345"
	out.SetAnnotations(annotations)
	out.SetFinalizers([]string{"controller.example.com/finalizer"})

	client, _ := newServerSideApplyClient(t, out)
	result, err := client.maybeUpdate(context.Background(), newConfigMap("default", "ssa-config"), existing.DeepCopy(), SyncOptions{
		ApplyStrategy:   model.ApplyStrategyServer,
		DisableUpdateFn: func(model.K8sMeta) bool { return false },
	}, internalSyncOptions{})
	require.NoError(t, err)
	assert.Equal(t, identicalObjects, result.SkipReason)
}

func TestMaybeUpdateServerSideApplyDetectsRemovedManagedFields(t *testing.T) {
	existing := newConfigMapWithData("default", "ssa-config", map[string]interface{}{
		"foo":   "bar",
		"stale": "remove-me",
	}).ToUnstructured()
	existing.SetManagedFields([]metav1.ManagedFieldsEntry{
		{
			Manager:    ssaFieldManager,
			Operation:  metav1.ManagedFieldsOperationApply,
			FieldsType: "FieldsV1",
			FieldsV1: &metav1.FieldsV1{
				Raw: []byte(`{"f:data":{"f:foo":{},"f:stale":{}}}`),
			},
		},
	})
	out := newConfigMap("default", "ssa-config").ToUnstructured()

	client, recorder := newServerSideApplyClient(t, out)
	result, err := client.maybeUpdate(context.Background(), newConfigMap("default", "ssa-config"), existing.DeepCopy(), SyncOptions{
		ApplyStrategy:   model.ApplyStrategyServer,
		DisableUpdateFn: func(model.K8sMeta) bool { return false },
	}, internalSyncOptions{})
	require.NoError(t, err)
	assert.Empty(t, result.SkipReason)
	assert.Equal(t, opUpdate, result.Operation)
	assert.Equal(t, apiTypes.ApplyPatchType, recorder.patchType)
}

func TestMaybeUpdateServerSideApplyForcesLegacyClientSideMigration(t *testing.T) {
	existing := newConfigMap("default", "ssa-config").ToUnstructured()
	annotations := existing.GetAnnotations()
	annotations[model.QbecNames.PristineAnnotation] = "pristine"
	existing.SetAnnotations(annotations)

	out := newConfigMap("default", "ssa-config").ToUnstructured()
	client, recorder := newServerSideApplyClient(t, out)
	result, err := client.maybeUpdate(context.Background(), newConfigMap("default", "ssa-config"), existing.DeepCopy(), SyncOptions{
		ApplyStrategy:   model.ApplyStrategyServer,
		DisableUpdateFn: func(model.K8sMeta) bool { return false },
	}, internalSyncOptions{})
	require.NoError(t, err)
	assert.Equal(t, opUpdate, result.Operation)
	require.NotNil(t, recorder.patchOptions.Force)
	assert.True(t, *recorder.patchOptions.Force)
}

func TestMaybeUpdateServerSideApplyForcesKubectlMigration(t *testing.T) {
	existing := newConfigMap("default", "ssa-config").ToUnstructured()
	annotations := existing.GetAnnotations()
	annotations[kubectlLastConfig] = `{"apiVersion":"v1"}`
	existing.SetAnnotations(annotations)

	out := newConfigMap("default", "ssa-config").ToUnstructured()
	client, recorder := newServerSideApplyClient(t, out)
	result, err := client.maybeUpdate(context.Background(), newConfigMap("default", "ssa-config"), existing.DeepCopy(), SyncOptions{
		ApplyStrategy:   model.ApplyStrategyServer,
		DisableUpdateFn: func(model.K8sMeta) bool { return false },
	}, internalSyncOptions{})
	require.NoError(t, err)
	assert.Equal(t, opUpdate, result.Operation)
	require.NotNil(t, recorder.patchOptions.Force)
	assert.True(t, *recorder.patchOptions.Force)
}

func TestMaybeUpdateServerSideApplyCleansLegacyOwnedFields(t *testing.T) {
	legacy := newConfigMapWithData("default", "ssa-config", map[string]interface{}{
		"foo":   "bar",
		"stale": "remove-me",
	})
	clientApplied, err := qbecPristine{}.createFromPristine(legacy)
	require.NoError(t, err)
	existing := legacy.ToUnstructured().DeepCopy()
	existing.SetAnnotations(clientApplied.ToUnstructured().GetAnnotations())
	out := newConfigMap("default", "ssa-config").ToUnstructured()

	client, recorder := newServerSideApplyClient(t, out)
	result, err := client.maybeUpdate(context.Background(), newConfigMap("default", "ssa-config"), existing.DeepCopy(), SyncOptions{
		ApplyStrategy:   model.ApplyStrategyServer,
		DisableUpdateFn: func(model.K8sMeta) bool { return false },
	}, internalSyncOptions{})
	require.NoError(t, err)
	assert.Equal(t, opUpdate, result.Operation)
	require.Len(t, recorder.patchCalls, 2)

	cleanup := recorder.patchCalls[0]
	assert.NotEqual(t, apiTypes.ApplyPatchType, cleanup.kind)
	assert.Contains(t, string(cleanup.data), `"stale":null`)
	assert.NotContains(t, string(cleanup.data), `"`+model.QbecNames.PristineAnnotation+`":null`)

	ssa := recorder.patchCalls[1]
	assert.Equal(t, apiTypes.ApplyPatchType, ssa.kind)
	assert.Contains(t, string(ssa.data), `"`+model.QbecNames.PristineAnnotation+`":null`)
	require.NotNil(t, ssa.options.Force)
	assert.True(t, *ssa.options.Force)
}

func TestMaybeUpdateServerSideApplyTreatsEmptyDesiredCollectionAsUpdate(t *testing.T) {
	existing := newConfigMapWithData("default", "ssa-config", map[string]interface{}{
		"foo": "bar",
	}).ToUnstructured()
	desired := newConfigMapWithData("default", "ssa-config", map[string]interface{}{})
	out := desired.ToUnstructured()

	client, recorder := newServerSideApplyClient(t, out)
	result, err := client.maybeUpdate(context.Background(), desired, existing.DeepCopy(), SyncOptions{
		ApplyStrategy:   model.ApplyStrategyServer,
		DisableUpdateFn: func(model.K8sMeta) bool { return false },
	}, internalSyncOptions{})
	require.NoError(t, err)
	assert.Empty(t, result.SkipReason)
	assert.Equal(t, opUpdate, result.Operation)
	assert.Equal(t, apiTypes.ApplyPatchType, recorder.patchType)
}

func TestServerSideApplyStripsApplyHistoryAnnotations(t *testing.T) {
	obj := newConfigMap("default", "ssa-config")
	annotated := cloneLocalObject(obj)
	annotations := annotated.ToUnstructured().GetAnnotations()
	annotations[model.QbecNames.PristineAnnotation] = "pristine"
	annotations[kubectlLastConfig] = `{"apiVersion":"v1"}`
	annotated.ToUnstructured().SetAnnotations(annotations)

	client, recorder := newServerSideApplyClient(t, annotated.ToUnstructured())
	_, err := client.serverSideApply(context.Background(), annotated, nil, SyncOptions{}, opUpdate)
	require.NoError(t, err)

	var payload map[string]interface{}
	require.NoError(t, json.Unmarshal(recorder.patchData, &payload))
	gotAnnotations, found, err := unstructured.NestedStringMap(payload, "metadata", "annotations")
	require.NoError(t, err)
	require.True(t, found)
	assert.NotContains(t, gotAnnotations, model.QbecNames.PristineAnnotation)
	assert.NotContains(t, gotAnnotations, kubectlLastConfig)
	assert.Equal(t, obj.Component(), gotAnnotations[model.QbecNames.ComponentAnnotation])
}
