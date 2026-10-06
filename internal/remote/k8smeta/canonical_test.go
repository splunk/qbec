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

package k8smeta

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

// kyverno.io: the preferred version v2 does not contain ClusterPolicy, which is served by v1 and v2beta1.
func kindMissingInPreferredVersion() *disco {
	verbs := metav1.Verbs{"create", "delete", "get", "list"}
	res := func(name, kind string) metav1.APIResource {
		return metav1.APIResource{Name: name, Kind: kind, Verbs: verbs}
	}
	return &disco{
		Groups: &metav1.APIGroupList{Groups: []metav1.APIGroup{{
			Name: "kyverno.io",
			Versions: []metav1.GroupVersionForDiscovery{
				{GroupVersion: "kyverno.io/v2", Version: "v2"},
				{GroupVersion: "kyverno.io/v1", Version: "v1"},
				{GroupVersion: "kyverno.io/v2beta1", Version: "v2beta1"},
			},
			PreferredVersion: metav1.GroupVersionForDiscovery{GroupVersion: "kyverno.io/v2", Version: "v2"},
		}}},
		ResourceLists: map[string]*metav1.APIResourceList{
			"kyverno.io:v2":      {APIResources: []metav1.APIResource{res("cleanuppolicies", "CleanupPolicy")}},
			"kyverno.io:v1":      {APIResources: []metav1.APIResource{res("clusterpolicies", "ClusterPolicy")}},
			"kyverno.io:v2beta1": {APIResources: []metav1.APIResource{res("clusterpolicies", "ClusterPolicy"), res("cleanuppolicies", "CleanupPolicy")}},
		},
	}
}

func TestCanonicalKindMissingInPreferredVersion(t *testing.T) {
	r, err := NewResources(kindMissingInPreferredVersion(), ResourceOpts{})
	require.NoError(t, err)

	v1 := schema.GroupVersionKind{Group: "kyverno.io", Version: "v1", Kind: "ClusterPolicy"}
	v2beta1 := schema.GroupVersionKind{Group: "kyverno.io", Version: "v2beta1", Kind: "ClusterPolicy"}
	for _, gvk := range []schema.GroupVersionKind{v1, v2beta1} {
		canonical, err := r.CanonicalGroupVersionKind(gvk)
		require.NoError(t, err)
		assert.Equal(t, v1, canonical, "canonical of %v", gvk)
	}

	cleanup, err := r.CanonicalGroupVersionKind(schema.GroupVersionKind{Group: "kyverno.io", Version: "v2beta1", Kind: "CleanupPolicy"})
	require.NoError(t, err)
	assert.Equal(t, "v2", cleanup.Version)

	// map iteration order is random, the listed version must not be
	for i := 0; i < 100; i++ {
		assert.Equal(t, "v1", r.CanonicalResources()[v1.GroupKind()].Version)
	}
}
