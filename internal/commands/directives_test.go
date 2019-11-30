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
	"testing"

	"github.com/splunk/qbec/internal/model"
	"github.com/splunk/qbec/internal/sio"
	"github.com/stretchr/testify/assert"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

func k8sMetaWithAnnotations(kind, namespace, name string, anns map[string]interface{}) model.K8sMeta {
	meta := map[string]interface{}{
		"name":        name,
		"annotations": anns,
	}
	if namespace != "" {
		meta["namespace"] = namespace
	}
	return model.NewK8sObject(map[string]interface{}{
		"apiVersion": "v1",
		"kind":       kind,
		"metadata":   meta,
	})
}

func TestDirectivesIsSet(t *testing.T) {
	tests := []struct {
		name     string
		anns     map[string]interface{}
		expected bool
		warning  string
	}{
		{
			"nil-annotations", nil, false, "",
		},
		{
			"empty-annotations", map[string]interface{}{}, false, "",
		},
		{
			"nomatch-annotations", map[string]interface{}{"x": "always"}, false, "",
		},
		{
			"match-annotations", map[string]interface{}{"x": "never"}, true, "",
		},
		{
			"bad-annotations", map[string]interface{}{"x": "garbage"}, false, "ignored annotation x=garbage does not have one of the allowed values: always, never",
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			var b bytes.Buffer
			orig := sio.Output
			defer func() {
				sio.Output = orig
			}()
			sio.Output = &b
			ret := isSet(k8sMetaWithAnnotations("ConfigMap", "default", "bar", test.anns), "x", "never", []string{"always"})
			assert.EqualValues(t, test.expected, ret)
			if test.warning != "" {
				assert.Contains(t, b.String(), test.warning)
			}
		})
	}
}

func TestDirectivesUpdatePolicy(t *testing.T) {
	up := newUpdatePolicy()
	a := assert.New(t)
	ret := up.disableUpdate(k8sMetaWithAnnotations("ConfigMap", "foo", "bar", nil))
	a.False(ret)
	ret = up.disableUpdate(k8sMetaWithAnnotations("ConfigMap", "foo", "bar", map[string]interface{}{
		"directives.qbec.io/update-policy": "never",
	}))
	a.True(ret)
}

func TestDirectivesDeletePolicy(t *testing.T) {
	dp := newDeletePolicy(func(gvk schema.GroupVersionKind) (bool, error) {
		return gvk.Kind == "ConfigMap", nil
	}, "foobar")
	a := assert.New(t)
	a.True(dp.disableDelete(k8sMetaWithAnnotations("Namespace", "", "default", nil)))
	a.True(dp.disableDelete(k8sMetaWithAnnotations("Namespace", "", "kube-system", nil)))
	a.False(dp.disableDelete(k8sMetaWithAnnotations("Namespace", "", "foobar", nil)))
	a.False(dp.disableDelete(k8sMetaWithAnnotations("ConfigMap", "default", "foobar", nil)))

	disableAnns := map[string]interface{}{
		"directives.qbec.io/delete-policy": "never",
	}
	cmNoNs := k8sMetaWithAnnotations("ConfigMap", "", "cm1", disableAnns)
	a.True(dp.disableDelete(cmNoNs))
	a.True(dp.disableDelete(k8sMetaWithAnnotations("Namespace", "", "foobar", nil)))

	cmNs := k8sMetaWithAnnotations("ConfigMap", "xxx", "cm1", disableAnns)
	a.True(dp.disableDelete(cmNs))
	a.True(dp.disableDelete(k8sMetaWithAnnotations("Namespace", "", "xxx", nil)))

	clusterNs := k8sMetaWithAnnotations("ClusterObj", "yyy", "cobj1", disableAnns)
	a.True(dp.disableDelete(clusterNs))
	a.False(dp.disableDelete(k8sMetaWithAnnotations("Namespace", "", "yyy", nil)))
}
