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

package remote

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/jonboulle/clockwork"
	"github.com/pkg/errors"
	"github.com/splunk/qbec/internal/model"
	"github.com/splunk/qbec/internal/sio"
	apiErrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/jsonmergepatch"
	"k8s.io/apimachinery/pkg/util/mergepatch"
	"k8s.io/apimachinery/pkg/util/strategicpatch"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/kube-openapi/pkg/util/proto"
	oapi "k8s.io/kube-openapi/pkg/util/proto"
)

// this file contains the patch code from kubectl, modified such that it does not pull in the whole world from
// those libraries with parts re-written for maintainer clarity and a new algorithm for detecting empty patches
// to reduce apply --dry-run noise.

const (
	// maxPatchRetry is the maximum number of conflicts retry for during a patch operation before returning failure
	maxPatchRetry = 5
	// backOffPeriod is the period to back off when apply patch resutls in error.
	backOffPeriod = 1 * time.Second
	// how many times we can retry before back off
	triesBeforeBackOff = 1
)

type resourceInterfaceProvider func(gvk schema.GroupVersionKind, namespace string) (dynamic.ResourceInterface, error)
type originalConfigurationProvider func(obj *unstructured.Unstructured) ([]byte, error)
type openAPILookup func(gvk schema.GroupVersionKind) proto.Schema

type patcher struct {
	provider      resourceInterfaceProvider
	cfgProvider   originalConfigurationProvider
	overwrite     bool
	backOff       clockwork.Clock
	openAPILookup openAPILookup
}

type serialized struct {
	server   []byte // the document as it exists on the server
	pristine []byte // the last applied document if known
	desired  []byte // the current document we want
}

func (p *patcher) getSerialized(serverObj *unstructured.Unstructured, desired model.K8sObject) (*serialized, error) {
	// serialize the current configuration of the object from the server.
	serverBytes, err := runtime.Encode(unstructured.UnstructuredJSONScheme, serverObj)
	if err != nil {
		return nil, errors.Wrap(err, "serialize server config")
	}

	// retrieve the original configuration of the object.
	desiredBytes, err := runtime.Encode(unstructured.UnstructuredJSONScheme, desired.ToUnstructured())
	if err != nil {
		return nil, errors.Wrap(err, fmt.Sprintf("serialize desired config"))
	}

	// retrieve the original configuration of the object. nil is ok if no original config was found.
	pristineBytes, err := p.cfgProvider(serverObj)
	if err != nil {
		return nil, errors.Wrap(err, fmt.Sprintf("retrieve original config"))
	}
	return &serialized{
		server:   serverBytes,
		desired:  desiredBytes,
		pristine: pristineBytes,
	}, nil
}

// deleteEmpty deletes the supplied key for the parent if the value for the
// key is an empty object after deleteEmpty has been called on _it_.
func deleteEmpty(parent map[string]interface{}, key string) {
	entry := parent[key]
	switch value := entry.(type) {
	case map[string]interface{}:
		for k := range value {
			deleteEmpty(value, k)
		}
		if len(value) == 0 {
			delete(parent, key)
		}
	}
}

// isEmptyPatch returns true if the unmarshaled version of the JSON patch is an empty object or only
// contains empty objects. It makes an assumption that there is actually no reason an empty object
// needs to be updated for a Kubernetes resource considering that the server would already have an object
// there on initial create if needed. Things considered empty will be of the form:
//
//	{}
//	{ metadata: { labels: {}, annotations: {} }
//	{ metadata: { labels: {}, annotations: {} }, spec: { foo: { bar: {} } } }
func isEmptyPatch(patch []byte) bool {
	var root map[string]interface{}
	err := json.Unmarshal(patch, &root)
	if err != nil {
		sio.Warnf("could not unmarshal patch %s", patch)
		return false // assume the worst
	}
	for k := range root {
		deleteEmpty(root, k)
	}
	return len(root) == 0
}

func newPatchResult(src string, kind types.PatchType, patch []byte) *updateResult {
	if isEmptyPatch(patch) {
		return &updateResult{SkipReason: identicalObjects}
	}
	pr := &updateResult{
		Operation: opUpdate,
		Source:    src,
		Kind:      kind,
		patch:     patch,
	}
	return pr
}

// getPatchContents returns the contents of the patch to take the supplied object to its modified version considering
// any previous configuration applied. The result has a SkipReason set when nothing needs to be done. This is the only
// way to correctly determine if a patch needs to be applied.
func (p *patcher) getPatchContents(serverObj *unstructured.Unstructured, desired model.K8sObject) (*updateResult, error) {
	// get the serialized versions of server, desired and pristine
	ser, err := p.getSerialized(serverObj, desired)
	if err != nil {
		return nil, err
	}
	var lookupPatchMeta strategicpatch.LookupPatchMeta
	var sch oapi.Schema
	patchContext := fmt.Sprintf("creating patch with:\npristine:\n%s\ndesired:\n%s\nserver:\n%s\nfor:", ser.pristine, ser.desired, ser.server)
	gvk := serverObj.GroupVersionKind()

	// see if a versioned struct is available in scheme
	versionedObject, err := scheme.Scheme.New(gvk)
	if err != nil && !runtime.IsNotRegisteredError(err) {
		return nil, errors.Wrap(err, fmt.Sprintf("getting instance of versioned object for %v:", gvk))
	}

	registered := err == nil

	// prefer open API if available to create a strategic merge patch; but only if the object was registered
	if registered && p.openAPILookup != nil {
		if sch = p.openAPILookup(gvk); sch != nil {
			lookupPatchMeta = strategicpatch.PatchMetaFromOpenAPI{Schema: sch}
			if openapiPatch, err := strategicpatch.CreateThreeWayMergePatch(ser.pristine, ser.desired, ser.server, lookupPatchMeta, p.overwrite); err == nil {
				return newPatchResult("open API", types.StrategicMergePatchType, openapiPatch), nil
			}
			sio.Warnf("warning: error calculating patch from openapi spec: %v\n", err)
		}
	}

	if !registered { // fallback to generic JSON merge patch
		preconditions := []mergepatch.PreconditionFunc{
			mergepatch.RequireKeyUnchanged("apiVersion"),
			mergepatch.RequireKeyUnchanged("kind"),
			mergepatch.RequireMetadataKeyUnchanged("name"),
		}
		patch, err := jsonmergepatch.CreateThreeWayJSONMergePatch(ser.pristine, ser.desired, ser.server, preconditions...)
		if err != nil {
			if mergepatch.IsPreconditionFailed(err) {
				return nil, fmt.Errorf("%s%s", patchContext, "At least one of apiVersion, kind and name was changed")
			}
			return nil, errors.Wrap(err, patchContext)
		}
		return newPatchResult("unregistered", types.MergePatchType, patch), nil
	}

	// strategic merge patch with struct metadata as source
	lookupPatchMeta, err = strategicpatch.NewPatchMetaFromStruct(versionedObject)
	if err != nil {
		return nil, errors.Wrap(err, patchContext)
	}
	patch, err := strategicpatch.CreateThreeWayMergePatch(ser.pristine, ser.desired, ser.server, lookupPatchMeta, p.overwrite)
	if err != nil {
		return nil, errors.Wrap(err, patchContext)
	}
	return newPatchResult("struct definition", types.StrategicMergePatchType, patch), nil
}

func (p *patcher) patchSimple(ctx context.Context, serverObj *unstructured.Unstructured, desired model.K8sObject) (result *updateResult, err error) {
	result, err = p.getPatchContents(serverObj, desired)
	if err != nil {
		return
	}
	if result.SkipReason != "" {
		return
	}
	gvk := serverObj.GroupVersionKind()
	ri, err := p.provider(gvk, serverObj.GetNamespace())
	if err != nil {
		return nil, errors.Wrap(err, fmt.Sprintf("error getting update interface for %v", gvk))
	}
	_, err = ri.Patch(ctx, serverObj.GetName(), result.Kind, result.patch, metav1.PatchOptions{})
	return result, err
}

func (p *patcher) patch(ctx context.Context, serverObj *unstructured.Unstructured, desired model.K8sObject) (*updateResult, error) {
	gvk := serverObj.GroupVersionKind()
	namespace := serverObj.GetNamespace()
	name := serverObj.GetName()
	var getErr error
	result, err := p.patchSimple(ctx, serverObj, desired)
	for i := 1; i <= maxPatchRetry && apiErrors.IsConflict(err); i++ {
		if i > triesBeforeBackOff {
			p.backOff.Sleep(backOffPeriod)
		}
		var ri dynamic.ResourceInterface
		ri, err = p.provider(gvk, namespace)
		if err != nil {
			return nil, err
		}
		serverObj, getErr = ri.Get(ctx, name, metav1.GetOptions{})
		if getErr != nil {
			return nil, getErr
		}
		result, err = p.patchSimple(ctx, serverObj, desired)
	}
	return result, err
}
