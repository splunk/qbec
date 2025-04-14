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

package objsort

import (
	"errors"
	"fmt"
	"testing"

	"github.com/splunk/qbec/internal/model"
	"github.com/stretchr/testify/assert"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

type data struct {
	component  string
	apiVersion string
	kind       string
	name       string
	namespace  string
}

func object(d data) model.K8sLocalObject {
	return model.NewK8sLocalObject(map[string]interface{}{
		"apiVersion": d.apiVersion,
		"kind":       d.kind,
		"metadata": map[string]interface{}{
			"namespace": d.namespace,
			"name":      d.name,
		},
	}, model.LocalAttrs{App: "app1", Tag: "", Component: d.component, Env: "dev"})
}

func TestBasicSort(t *testing.T) {
	inputs := []model.K8sLocalObject{
		object(data{"c0", "v1", "BarBaz", "c1-secret", ""}),
		object(data{"c1", "v1", "Secret", "c1-secret", ""}),
		object(data{"c1", "v1", "Namespace", "c1-ns", ""}),
		object(data{"c1", "v1", "ServiceAccount", "c1-sa", "c1-ns"}),
		object(data{"cluster", "v1", "PodSecurityPolicy", "cluster-psp", ""}),
		object(data{"c2", "extensions/v1beta1", "Deployment", "deploy1", "c1-ns"}),
		object(data{"c2", "extensions/v1beta1", "FooBar", "fb1", "c1-ns"}),
		object(data{"c3", "rbac.authorization.k8s.io/v1beta1", "RoleBinding", "rb1", "c1-ns"}),
		object(data{"c3", "rbac.authorization.k8s.io/v1beta1", "ClusterRole", "cr", ""}),
	}
	sorted := Sort(inputs, Config{
		OrderingProvider: func(item model.K8sQbecMeta) int {
			if item.GetName() == "c1-ns" {
				return 1
			}
			return 0
		},
		NamespacedIndicator: func(gvk schema.GroupVersionKind) (bool, error) {
			if gvk.Kind == "PodSecurityPolicy" || gvk.Kind == "FooBar" || gvk.Kind == "ClusterRoleBinding" {
				return false, nil
			}
			if gvk.Kind == "BarBaz" {
				return false, errors.New("no indicator for BarBaz")
			}
			return true, nil
		},
	})
	var results []string
	for _, s := range sorted {
		results = append(results, fmt.Sprintf("%s:%s:%s", s.GetKind(), s.GetName(), s.GetNamespace()))
	}
	expected := []string{
		"Namespace:c1-ns:",
		"FooBar:fb1:c1-ns",
		"PodSecurityPolicy:cluster-psp:",
		"ServiceAccount:c1-sa:c1-ns",
		"Secret:c1-secret:",
		"ClusterRole:cr:",
		"RoleBinding:rb1:c1-ns",
		"Deployment:deploy1:c1-ns",
		"BarBaz:c1-secret:",
	}
	assert.EqualValues(t, expected, results)
}

func TestBasicSortMeta(t *testing.T) {
	inputs := []model.K8sQbecMeta{
		object(data{"c0", "v1", "BarBaz", "c1-secret", ""}),
		object(data{"c1", "v1", "Secret", "c1-secret", ""}),
		object(data{"c1", "v1", "Namespace", "c1-ns", ""}),
		object(data{"c1", "v1", "ServiceAccount", "c1-sa", "c1-ns"}),
		object(data{"cluster", "v1", "PodSecurityPolicy", "cluster-psp", ""}),
		object(data{"c2", "extensions/v1beta1", "Deployment", "deploy1", "c1-ns"}),
		object(data{"c2", "extensions/v1beta1", "FooBar", "fb1", "c1-ns"}),
		object(data{"c3", "rbac.authorization.k8s.io/v1beta1", "RoleBinding", "rb1", "c1-ns"}),
		object(data{"c3", "rbac.authorization.k8s.io/v1beta1", "ClusterRole", "cr", ""}),
	}
	sorted := SortMeta(inputs, Config{
		OrderingProvider: func(item model.K8sQbecMeta) int {
			if item.GetName() == "c1-ns" {
				return 1
			}
			return 0
		},
		NamespacedIndicator: func(gvk schema.GroupVersionKind) (bool, error) {
			if gvk.Kind == "PodSecurityPolicy" || gvk.Kind == "FooBar" || gvk.Kind == "ClusterRoleBinding" {
				return false, nil
			}
			if gvk.Kind == "BarBaz" {
				return false, errors.New("no indicator for BarBaz")
			}
			return true, nil
		},
	})
	var results []string
	for _, s := range sorted {
		results = append(results, fmt.Sprintf("%s:%s:%s", s.GetKind(), s.GetName(), s.GetNamespace()))
	}
	expected := []string{
		"Namespace:c1-ns:",
		"FooBar:fb1:c1-ns",
		"PodSecurityPolicy:cluster-psp:",
		"ServiceAccount:c1-sa:c1-ns",
		"Secret:c1-secret:",
		"ClusterRole:cr:",
		"RoleBinding:rb1:c1-ns",
		"Deployment:deploy1:c1-ns",
		"BarBaz:c1-secret:",
	}
	assert.EqualValues(t, expected, results)
}
