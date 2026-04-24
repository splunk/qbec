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
			file: "both-applied.yaml",
			asserter: func(t *testing.T, obj *unstructured.Unstructured, source string) {
				a := assert.New(t)
				a.Equal("kubectl annotation", source)
				a.NotNil(obj)
				a.NotEqual("", obj.GetAnnotations()[kubectlLastConfig])
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
