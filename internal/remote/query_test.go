package remote

import (
	"context"
	"fmt"
	"testing"

	"github.com/pkg/errors"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"
	dynamicfakeclient "k8s.io/client-go/dynamic/fake"
	faketesting "k8s.io/client-go/testing"
	cmdtesting "k8s.io/kubectl/pkg/cmd/testing"
	"k8s.io/kubectl/pkg/scheme"
)

func newUnstructuredList(apiVersion, kind string, continueVal int64, items ...*unstructured.Unstructured) *unstructured.UnstructuredList {
	continueValueToSet := ""
	// Set continue if there's more items to come
	if continueVal > 0 {
		continueValueToSet = fmt.Sprint(continueVal)
	}
	list := &unstructured.UnstructuredList{
		Object: map[string]interface{}{
			"apiVersion": apiVersion,
			"kind":       kind,
			"metadata": map[string]interface{}{
				"continue": continueValueToSet,
			},
		},
	}
	for i := range items {
		list.Items = append(list.Items, *items[i])
	}
	return list
}

func newUnstructured(apiVersion, kind, namespace, name string) *unstructured.Unstructured {
	return &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": apiVersion,
			"kind":       kind,
			"metadata": map[string]interface{}{
				"namespace": namespace,
				"name":      name,
				"labels": map[string]interface{}{
					"qbec.io/application": "app",
					"qbec.io/environment": "env",
				},
				"uid": "some-UID-value",
			},
		},
	}
}

func TestListPagination(t *testing.T) {
	tf := cmdtesting.NewTestFactory().WithNamespace("test")
	defer tf.Cleanup()

	listMapping := map[schema.GroupVersionResource]string{
		{Group: "", Version: "v1", Resource: "secrets"}: "SecretList",
	}
	tf.FakeDynamicClient = dynamicfakeclient.NewSimpleDynamicClientWithCustomListKinds(scheme.Scheme, listMapping)
	var uns []*unstructured.Unstructured
	var totalItemsInList = int64(3)
	for i := int64(0); i <= totalItemsInList; i++ {
		ns := "default"
		uns = append(uns, newUnstructured("v1", "Secret", ns, fmt.Sprintf("test-secret-%d", i)))
	}
	callIndex := int64(0)
	tf.FakeDynamicClient.PrependReactor("list", "secrets", func(action faketesting.Action) (handled bool, ret runtime.Object, err error) {
		if callIndex >= totalItemsInList {
			return true, nil, errors.New("unexpected call to list. list has been served already")
		}
		listWithContinue := newUnstructuredList("v1", "SecretList", totalItemsInList-callIndex-1, uns[callIndex])
		callIndex++
		return true, listWithContinue, nil
	})
	qc := queryConfig{
		scope: ListQueryConfig{
			Application:        "app",
			Tag:                "",
			Environment:        "env",
			KindFilter:         nil,
			ListQueryScope:     ListQueryScope{},
			ClusterScopedLists: true,
			Limit:              1,
		},
		resourceProvider: func(gvk schema.GroupVersionKind, namespace string) (dynamic.ResourceInterface, error) {
			return tf.FakeDynamicClient.Resource(schema.GroupVersionResource{Resource: "secrets", Version: "v1"}), nil
		},
		namespacedTypes: []schema.GroupVersionKind{},
		clusterTypes:    []schema.GroupVersionKind{},
		verbosity:       4,
	}
	ol := objectLister{qc}
	objs, err := ol.listObjectsOfType(context.TODO(), schema.GroupVersionKind{Version: "v1", Kind: "Secret"}, "default")
	if err != nil {
		t.Fatalf("unexpected err %v", err)
	}
	actual := len(objs)
	if int(totalItemsInList) != actual {
		// t.Logf("expected items to be %d but found %d. Change this to Fatal when https://github.com/kubernetes/kubernetes/issues/107277 is fixed", totalItemsInList, actual)
		t.Fatalf("expected items to be %d but found %d", totalItemsInList, actual)
	}
}
