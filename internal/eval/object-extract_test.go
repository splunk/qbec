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

package eval

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSimpleObject(t *testing.T) {
	obj := map[string]interface{}{
		"kind": "ConfigMap",
		"metadata": map[string]interface{}{
			"name": "foo",
		},
		"apiVersion": "v1",
		"data":       map[string]string{"foo": "bar"},
	}
	ret, err := walk(obj)
	require.Nil(t, err)
	require.Equal(t, 1, len(ret))
}

func TestSimpleObjectNoName(t *testing.T) {
	obj := map[string]interface{}{
		"kind":       "ConfigMap",
		"metadata":   map[string]interface{}{},
		"apiVersion": "v1",
		"data":       map[string]string{"foo": "bar"},
	}
	nested := map[string]interface{}{
		"apiVersion": "v1",
		"kind":       "List",
		"items":      []interface{}{obj},
	}
	_, err := walk(nested)
	require.NotNil(t, err)
	assert.Contains(t, err.Error(), "did not have a name")
	assert.Contains(t, err.Error(), `"$.items[0]"`)
	assert.Contains(t, err.Error(), `"foo": "bar"`)
}

func TestDeepObjectNesting(t *testing.T) {
	obj := map[string]interface{}{
		"kind":       "ConfigMap",
		"apiVersion": "v1",
		"metadata": map[string]interface{}{
			"name": "foo",
		},
		"data": map[string]string{"foo": "bar"},
	}
	nested := map[string]interface{}{
		"comp1": map[string]interface{}{
			"inner": []interface{}{
				map[string]interface{}{
					"apiVersion": "v1",
					"kind":       "List",
					"items": []interface{}{
						map[string]interface{}{
							"keyed": obj,
						},
					},
				},
			},
		},
		"comp2": nil,
	}
	ret, err := walk(nested)
	require.Nil(t, err)
	require.Equal(t, 1, len(ret))
	assert.Equal(t, obj, ret[0])
}
