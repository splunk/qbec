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

package pristine

import (
	"bytes"
	"compress/gzip"
	"encoding/base64"
	"encoding/json"

	"github.com/pkg/errors"
	"github.com/splunk/qbec/internal/model"
	"github.com/splunk/qbec/internal/sio"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

// this file contains all pristine data processing. Pristine data is the original configuration
// written to the remote object as an annotation so that changes are be properly submitted via
// a 3-way merge that ensures that data written by the server is not overwritten on updates.
// Absence of pristine data is not an error; in this case we generate a pristine object that only
// has basic metadata.

// zipData returns a base64 encoded gzipped version of the JSON serialization of the supplied object.
func zipData(data map[string]interface{}) (string, error) {
	var buf bytes.Buffer
	gz := gzip.NewWriter(&buf)
	if err := json.NewEncoder(gz).Encode(data); err != nil {
		return "", err
	}
	if err := gz.Flush(); err != nil {
		return "", err
	}
	if err := gz.Close(); err != nil {
		return "", err
	}
	return base64.StdEncoding.EncodeToString(buf.Bytes()), nil
}

// unzipData is the exact reverse of a zipData operation.
func unzipData(s string) (map[string]interface{}, error) {
	b, err := base64.StdEncoding.DecodeString(s)
	if err != nil {
		return nil, err
	}

	r := bytes.NewReader(b)
	zr, err := gzip.NewReader(r)
	if err != nil {
		return nil, err
	}
	defer zr.Close()

	var data map[string]interface{}
	if err := json.NewDecoder(zr).Decode(&data); err != nil {
		return nil, err
	}
	return data, nil
}

type PristineReader interface {
	GetPristine(annotations map[string]string, obj *unstructured.Unstructured) (pristine *unstructured.Unstructured, source string)
}

type PristineReadWriter interface {
	PristineReader
	CreateFromPristine(obj model.K8sLocalObject) (model.K8sLocalObject, error)
}

type QbecPristine struct{}

func (k QbecPristine) GetPristine(annotations map[string]string, _ *unstructured.Unstructured) (*unstructured.Unstructured, string) {
	serialized := annotations[model.QbecNames.PristineAnnotation]
	if serialized == "" {
		return nil, ""
	}
	m, err := unzipData(serialized)
	if err != nil {
		sio.Warnln("unable to unzip pristine annotation", err)
		return nil, ""
	}
	return &unstructured.Unstructured{Object: m}, "qbec annotation"
}

func (k QbecPristine) CreateFromPristine(pristine model.K8sLocalObject) (model.K8sLocalObject, error) {
	b, err := json.Marshal(pristine)
	if err != nil {
		return nil, errors.Wrap(err, "pristine JSON marshal")
	}
	var annotated unstructured.Unstructured // duplicate object of pristine to start with
	if err := json.Unmarshal(b, &annotated); err != nil {
		return nil, errors.Wrap(err, "pristine JSON unmarshal")
	}
	zipped, err := zipData(pristine.ToUnstructured().Object)
	if err != nil {
		return nil, errors.Wrap(err, "zip data")
	}
	annotations := annotated.GetAnnotations()
	if annotations == nil {
		annotations = map[string]string{}
	}
	annotations[model.QbecNames.PristineAnnotation] = zipped
	annotated.SetAnnotations(annotations)
	return model.NewK8sLocalObject(annotated.Object, model.LocalAttrs{
		App:       pristine.Application(),
		Tag:       pristine.Tag(),
		Component: pristine.Component(),
		Env:       pristine.Environment(),
	}), nil
}

const kubectlLastConfig = "kubectl.kubernetes.io/last-applied-configuration"

type KubectlPristine struct{}

func (k KubectlPristine) GetPristine(annotations map[string]string, _ *unstructured.Unstructured) (*unstructured.Unstructured, string) {
	serialized := annotations[kubectlLastConfig]
	if serialized == "" {
		return nil, ""
	}
	var data map[string]interface{}
	if err := json.Unmarshal([]byte(serialized), &data); err != nil {
		sio.Warnln("unable to unmarshal pristine kubectl annotation", err)
		return nil, ""
	}
	// now set the annotation back into the pristine object so it is deleted when qbec tries to apply it
	ret := &unstructured.Unstructured{Object: data}
	anns := ret.GetAnnotations()
	if anns == nil {
		anns = map[string]string{}
	}
	anns[kubectlLastConfig] = serialized
	ret.SetAnnotations(anns)
	return ret, "kubectl annotation"
}

type fallbackPristine struct{}

func (f fallbackPristine) GetPristine(annotations map[string]string, orig *unstructured.Unstructured) (*unstructured.Unstructured, string) {
	delete(annotations, "deployment.kubernetes.io/revision")
	orig.SetDeletionTimestamp(nil)
	orig.SetCreationTimestamp(metav1.Time{})
	unstructured.RemoveNestedField(orig.Object, "metadata", "resourceVersion")
	unstructured.RemoveNestedField(orig.Object, "metadata", "selfLink")
	unstructured.RemoveNestedField(orig.Object, "metadata", "uid")
	unstructured.RemoveNestedField(orig.Object, "metadata", "generation")
	unstructured.RemoveNestedField(orig.Object, "status")
	orig.SetAnnotations(annotations)
	return orig, "fallback - live object with some attributes removed"
}

func GetPristineVersion(obj *unstructured.Unstructured, includeFallback bool) (*unstructured.Unstructured, string) {
	pristineReaders := []PristineReader{QbecPristine{}, KubectlPristine{}}
	if includeFallback {
		pristineReaders = append(pristineReaders, fallbackPristine{})
	}
	annotations := obj.GetAnnotations()
	if annotations == nil {
		annotations = map[string]string{}
	}
	for _, p := range pristineReaders {
		out, str := p.GetPristine(annotations, obj)
		if out != nil {
			return out, str
		}
	}
	return nil, ""
}

// GetPristineVersionForDiff interrogates annotations and extracts the pristine version of the supplied
// live object. If no annotations are found, it halfheartedly deletes known runtime information that is
// set on the server and returns the supplied object with those attributes removed.
func GetPristineVersionForDiff(obj *unstructured.Unstructured) (*unstructured.Unstructured, string) {
	return GetPristineVersion(obj, true)
}
