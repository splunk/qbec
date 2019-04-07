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
	"encoding/json"
	"fmt"
	"strings"

	"github.com/ghodss/yaml"
	"github.com/jonboulle/clockwork"
	"github.com/pkg/errors"
	"github.com/splunk/qbec/internal/model"
	"github.com/splunk/qbec/internal/sio"
	apiErrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/discovery"
	"k8s.io/client-go/dynamic"
)

const (
	identicalObjects = "objects are identical"
	opUpdate         = "update object"
	opCreate         = "create object"
)

// structured errors
var (
	ErrForbidden        = errors.New("forbidden")             // returned due to an authn/ authz error
	ErrNotFound         = errors.New("not found")             // returned when a remote object does not exist
	ErrSchemaNotFound   = errors.New("schema not found")      // returned when a validation schema is not found
	errMetadataNotFound = errors.New("server type not found") // returned when metadata could not be found for a gvk
)

// this file contains the client definition and supported CRUD operations.

// SyncOptions provides the caller with options for the sync operation.
type SyncOptions struct {
	DryRun        bool // do not actually create or update objects, return what would happen
	DisableCreate bool // only update objects if they exist, do not create new ones
	ShowSecrets   bool // show secrets in patches and creations
}

type internalSyncOptions struct {
	secretDryRun       bool               // dry-run phase for objects having secrets info
	pristiner          pristineReadWriter // pristine writer
	pristineAnnotation string             // pristine annotation to manipulate for secrets dry-run
}

// Client is a thick remote client that provides high-level operations for commands as opposed to
// granular ones.
type Client struct {
	sm           *ServerMetadata                  // the server metadata loaded once and never updated
	pool         dynamic.ClientPool               // the client pool for resource interfaces
	disco        minimalDiscovery                 // the discovery interface
	defaultNs    string                           // the default namespace to set for namespaced objects that do not define one
	verbosity    int                              // log verbosity
	dynamicTypes map[schema.GroupVersionKind]bool // crds seen by this client
}

func newClient(pool dynamic.ClientPool, disco discovery.DiscoveryInterface, ns string, verbosity int) (*Client, error) {
	sm, err := newServerMetadata(disco, ns, verbosity)
	if err != nil {
		return nil, errors.Wrap(err, "get server metadata")
	}
	c := &Client{
		sm:           sm,
		pool:         pool,
		disco:        disco,
		defaultNs:    ns,
		verbosity:    verbosity,
		dynamicTypes: map[schema.GroupVersionKind]bool{},
	}
	return c, nil
}

// ServerMetadata returns server metadata for the cluster that this client connects to.
func (c *Client) ServerMetadata() *ServerMetadata {
	return c.sm
}

// ValidatorFor returns a validator for the supplied group version kind.
func (c *Client) ValidatorFor(gvk schema.GroupVersionKind) (Validator, error) {
	return c.ServerMetadata().ValidatorFor(gvk)
}

// DisplayName returns the display name of the supplied K8s object.
func (c *Client) DisplayName(o model.K8sMeta) string {
	return c.ServerMetadata().DisplayName(o)
}

// IsNamespaced returns if the supplied group version kind is namespaced.
func (c *Client) IsNamespaced(kind schema.GroupVersionKind) (bool, error) {
	return c.ServerMetadata().IsNamespaced(kind)
}

// Get returns the remote object matching the supplied metadata as an unstructured bag of attributes.
func (c *Client) Get(obj model.K8sMeta) (*unstructured.Unstructured, error) {
	rc, err := c.resourceInterfaceWithDefaultNs(obj.GetObjectKind().GroupVersionKind(), obj.GetNamespace())
	if err != nil {
		return nil, err
	}
	u, err := rc.Get(obj.GetName(), metav1.GetOptions{})
	if err != nil {
		if apiErrors.IsNotFound(err) {
			return nil, ErrNotFound
		}
		if apiErrors.IsForbidden(err) {
			return nil, ErrForbidden
		}
		return nil, err
	}
	return u, nil
}

// ListQueryScope defines the scope at which list queries need to be executed.
type ListQueryScope struct {
	Namespaces     []string // namespaces of interest
	ClusterObjects bool     // whether to query for cluster objects
}

// ListQueryConfig is the config with which to execute list queries.
type ListQueryConfig struct {
	Application string // must be non-blank
	Environment string // must be non-blank
	ListQueryScope
	ComponentFilter     model.Filter // filters for object component
	KindFilter          model.Filter // filters for object kind
	Concurrency         int          // concurrent queries to execute
	DisableAllNsQueries bool         // do not perform list queries across namespaces when multiple namespaces in picture
}

// ListExtraObjects returns all objects for the application and environment that do not belong to the ignore list
// supplied. The ignore list is usually the unfiltered list of all objects belonging to all components for an
// environment.
func (c *Client) ListExtraObjects(ignore []model.K8sQbecMeta, scope ListQueryConfig) ([]model.K8sQbecMeta, error) {
	if scope.KindFilter == nil {
		kf, _ := model.NewKindFilter(nil, nil)
		scope.KindFilter = kf
	}
	if scope.ComponentFilter == nil {
		cf, _ := model.NewComponentFilter(nil, nil)
		scope.ComponentFilter = cf
	}
	ignoreCollection := newCollection(c.defaultNs, c.sm)
	for _, obj := range ignore {
		if !scope.KindFilter.ShouldInclude(obj.GetKind()) {
			continue
		}
		if err := ignoreCollection.add(obj); err != nil {
			return nil, errors.Wrap(err, "add retained object")
		}
	}

	// handle special cases
	filterEligibleTypes := func(types []schema.GroupVersionKind) []schema.GroupVersionKind {
		var ret []schema.GroupVersionKind
		for _, t := range types {
			switch {
			// the issue with endpoints is that every service creates endpoints objects and
			// propagates its own labels to it. These have not been created by qbec.
			case t.Group == "" && t.Kind == "Endpoints":
				if c.verbosity > 0 {
					sio.Debugf("not listing objects of type %v\n", t)
				}
			default:
				ret = append(ret, t)
			}
		}
		return ret
	}

	qc := queryConfig{
		scope:            scope,
		resourceProvider: c.resourceInterface,
		namespacedTypes:  filterEligibleTypes(c.sm.namespacedTypes()),
		clusterTypes:     filterEligibleTypes(c.sm.clusterTypes()),
		verbosity:        c.verbosity,
	}
	ol := objectLister{qc}
	coll := newCollection(c.defaultNs, c.sm)
	if err := ol.serverObjects(coll); err != nil {
		return nil, err
	}

	baseList := coll.subtract(ignoreCollection).toList()
	var ret []model.K8sQbecMeta
	for _, ob := range baseList {
		if scope.ComponentFilter.ShouldInclude(ob.Component()) {
			ret = append(ret, ob)
		}
	}
	return ret, nil
}

type updateResult struct {
	SkipReason   string          `json:"skip,omitempty"`
	Operation    string          `json:"operation,omitempty"`
	Source       string          `json:"source,omitempty"`
	Kind         types.PatchType `json:"kind,omitempty"`
	DisplayPatch string          `json:"patch,omitempty"`
	patch        []byte
}

func (u *updateResult) String() string {
	b, err := yaml.Marshal(u)
	if err != nil {
		sio.Warnln("unable to marshal result to YAML")
	}
	return string(b)
}

func (u *updateResult) toSyncResult() *SyncResult {
	switch {
	case u.SkipReason == identicalObjects:
		return &SyncResult{
			Type:    SyncObjectsIdentical,
			Details: u.SkipReason,
		}
	case u.SkipReason != "":
		return &SyncResult{
			Type:    SyncSkip,
			Details: u.SkipReason,
		}
	case u.Operation == opCreate:
		return &SyncResult{
			Type:    SyncCreated,
			Details: u.String(),
		}
	case u.Operation == opUpdate:
		return &SyncResult{
			Type:    SyncUpdated,
			Details: u.String(),
		}
	default:
		panic(fmt.Errorf("invalid operation:%s, %v", u.Operation, u))
	}
}

// SyncResultType indicates what notionally happened in a sync operation.
type SyncResultType int

// Sync result types
const (
	_                    SyncResultType = iota
	SyncObjectsIdentical                // sync was a noop due to local and remote being identical
	SyncSkip                            // object was skipped for sync (e.g. creation needed but disabled)
	SyncCreated                         // object was created
	SyncUpdated                         // object was updated
	SyncDeleted                         // object was deleted
)

// SyncResult is the result of a sync operation. There is no difference in the output for a real versus
// a dry-run.
type SyncResult struct {
	Type    SyncResultType // the result type
	Details string         // additional details that are safe to print to console (e.g. no secrets)
}

func extractCustomTypes(obj model.K8sObject) (schema.GroupVersionKind, error) {
	var ret schema.GroupVersionKind
	var crd struct {
		Spec struct {
			Group   string `json:"group"`
			Version string `json:"version"`
			Names   struct {
				Kind string `json:"kind"`
			} `json:"names"`
		} `json:"spec"`
	}
	b, err := obj.ToUnstructured().MarshalJSON()
	if err != nil {
		return ret, err
	}
	if err := json.Unmarshal(b, &crd); err != nil {
		return ret, err
	}
	return schema.GroupVersionKind{Group: crd.Spec.Group, Version: crd.Spec.Version, Kind: crd.Spec.Names.Kind}, nil
}

// Sync syncs the local object by either creating a new one or patching an existing one.
// It does not do anything in dry-run mode. It also does not create new objects if the caller has disabled the feature.
func (c *Client) Sync(original model.K8sLocalObject, opts SyncOptions) (_ *SyncResult, finalError error) {
	// set up the pristine strategy.
	var prw pristineReadWriter = qbecPristine{}
	sensitive := model.HasSensitiveInfo(original.ToUnstructured())

	internal := internalSyncOptions{
		secretDryRun:       false,
		pristiner:          prw,
		pristineAnnotation: model.QbecNames.PristineAnnotation,
	}

	if sensitive && !opts.ShowSecrets {
		internal.secretDryRun = true
	}

	defer func() {
		if finalError != nil {
			finalError = errors.Wrap(finalError, "sync "+c.sm.DisplayName(original))
		}
	}()

	gvk := original.GetObjectKind().GroupVersionKind()
	if gvk.Kind == "CustomResourceDefinition" && gvk.Group == "apiextensions.k8s.io" {
		t, err := extractCustomTypes(original)
		if err != nil {
			sio.Warnf("error extracting types for custom resource %s, %v\n", original.GetName(), err)
		} else {
			c.dynamicTypes[t] = true
		}
	}

	result, err := c.doSync(original, opts, internal)
	if err != nil {
		return nil, err
	}
	// exit if we are done
	if !internal.secretDryRun || opts.DryRun {
		return result.toSyncResult(), nil
	}
	internal.secretDryRun = false
	_, err = c.doSync(original, opts, internal) // do the real sync
	if err != nil {
		return nil, err
	}
	return result.toSyncResult(), err
}

func (c *Client) doSync(original model.K8sLocalObject, opts SyncOptions, internal internalSyncOptions) (*updateResult, error) {
	gvk := original.GetObjectKind().GroupVersionKind()
	remObj, objErr := c.Get(original)
	switch {
	// ignore object not found errors
	case objErr == ErrNotFound:
		break
	// treat metadata errors (server type not found) as a "not found" error under the following conditions:
	// - dry-run mode is active
	// - a prior custom resource with that GVK has been applied
	case objErr == errMetadataNotFound && opts.DryRun && c.dynamicTypes[gvk]:
		break
	// error but with better message
	case objErr == errMetadataNotFound && opts.DryRun:
		return nil, fmt.Errorf("server type %v not found and no prior CRD installs it", gvk)
	// report all other errors
	case objErr != nil:
		return nil, objErr
	}

	var obj model.K8sLocalObject
	if internal.secretDryRun {
		opts.DryRun = true // won't affect caller since passed by value
		obj, _ = model.HideSensitiveLocalInfo(original)
	} else {
		o, err := internal.pristiner.createFromPristine(original)
		if err != nil {
			return nil, errors.Wrap(err, "create from pristine")
		}
		obj = o
	}

	// create or update as needed, each of these routines is responsible for correct dry-run handling.
	var result *updateResult
	var err error
	if remObj == nil {
		result, err = c.maybeCreate(obj, opts)
	} else {
		if internal.secretDryRun {
			ann := remObj.GetAnnotations()
			if ann == nil {
				ann = map[string]string{}
			}
			delete(ann, internal.pristineAnnotation)
			remObj.SetAnnotations(ann)
			c, _ := model.HideSensitiveInfo(remObj)
			remObj = c
		}
		result, err = c.maybeUpdate(obj, remObj, opts)
	}
	if err != nil {
		return nil, err
	}

	// create a prettier patch for display, if needed
	result.DisplayPatch = string(result.patch)
	if result.patch != nil {
		var data interface{}
		if err := json.Unmarshal(result.patch, &data); err == nil {
			b, err := json.MarshalIndent(data, "", "    ")
			if err == nil {
				result.DisplayPatch = string(b)
			}
		}
	}
	return result, nil
}

// Delete delete the supplied object if it exists. It does not do anything in dry-run mode.
func (c *Client) Delete(obj model.K8sMeta, dryRun bool) (_ *SyncResult, finalError error) {
	ret := &SyncResult{
		Type: SyncDeleted,
	}
	if dryRun {
		return ret, nil
	}
	defer func() {
		if finalError != nil {
			finalError = errors.Wrap(finalError, "delete "+c.sm.DisplayName(obj))
		}
	}()

	ri, err := c.resourceInterfaceWithDefaultNs(obj.GetObjectKind().GroupVersionKind(), obj.GetNamespace())
	if err != nil {
		return nil, errors.Wrap(err, "get resource interface")
	}

	pp := metav1.DeletePropagationForeground
	err = ri.Delete(obj.GetName(), &metav1.DeleteOptions{PropagationPolicy: &pp})
	if err != nil {
		if apiErrors.IsNotFound(err) {
			ret.Type = SyncSkip
			ret.Details = "object not found on the server"
			return ret, nil
		}
		if apiErrors.IsConflict(err) && obj.GetKind() == "Namespace" {
			ret.Type = SyncSkip
			ret.Details = "namespace delete had conflict, ignore"
			return ret, nil
		}
		return nil, err
	}
	return ret, nil
}

func (c *Client) jitResource(gvk schema.GroupVersionKind) (*metav1.APIResource, error) {
	rl, err := c.disco.ServerResourcesForGroupVersion(gvk.GroupVersion().String())
	if err != nil {
		return nil, err
	}
	for _, r := range rl.APIResources {
		if strings.Contains(r.Name, "/") { // ignore sub-resources
			continue
		}
		if r.Kind == gvk.Kind {
			return &r, nil
		}
	}
	return nil, fmt.Errorf("server does not recognize gvk %s", gvk)
}

func (c *Client) resourceInterface(gvk schema.GroupVersionKind, namespace string) (dynamic.ResourceInterface, error) {
	client, err := c.pool.ClientForGroupVersionKind(gvk)
	if err != nil {
		return nil, err
	}
	info, err := c.sm.infoFor(gvk)
	var res *metav1.APIResource
	if err != nil { // could be a resource for a CRD that was just created, re-query discovery
		res, err = c.jitResource(gvk)
		if err != nil {
			return nil, errMetadataNotFound
		}
	} else {
		res = &info.resource
	}
	return client.Resource(res, namespace), nil
}

func (c *Client) resourceInterfaceWithDefaultNs(gvk schema.GroupVersionKind, namespace string) (dynamic.ResourceInterface, error) {
	if namespace == "" {
		namespace = c.defaultNs
	}
	return c.resourceInterface(gvk, namespace)
}

func (c *Client) maybeCreate(obj model.K8sLocalObject, opts SyncOptions) (*updateResult, error) {
	if opts.DisableCreate {
		return &updateResult{
			SkipReason: "creation disabled due to user request",
		}, nil
	}
	b, err := json.Marshal(obj)
	if err != nil {
		return nil, errors.Wrap(err, "json marshal")
	}
	result := &updateResult{
		Operation: opCreate,
		Source:    "local",
		patch:     b,
	}
	if opts.DryRun {
		return result, nil
	}
	ri, err := c.resourceInterfaceWithDefaultNs(obj.GetObjectKind().GroupVersionKind(), obj.GetNamespace())
	if err != nil {
		return nil, errors.Wrap(err, "get resource interface")
	}
	_, err = ri.Create(obj.ToUnstructured())
	if err != nil {
		return nil, err
	}
	return result, nil
}

func (c *Client) maybeUpdate(obj model.K8sLocalObject, remObj *unstructured.Unstructured, opts SyncOptions) (*updateResult, error) {
	res, _, err := c.sm.openAPIResources()
	if err != nil {
		sio.Warnln("get open API resources", err)
	}
	var lookup openAPILookup
	if res != nil {
		lookup = res.LookupResource
	}

	p := patcher{
		provider: c.resourceInterfaceWithDefaultNs,
		cfgProvider: func(obj *unstructured.Unstructured) ([]byte, error) {
			pristine, _ := getPristineVersion(obj, false)
			if pristine == nil {
				p := map[string]interface{}{
					"kind":       obj.GetKind(),
					"apiVersion": obj.GetAPIVersion(),
					"metadata": map[string]interface{}{
						"name": obj.GetName(),
					},
				}
				pb, _ := json.Marshal(p)
				return pb, nil
			}
			b, _ := json.Marshal(pristine)
			return b, nil
		},
		overwrite:     true,
		backOff:       clockwork.NewRealClock(),
		openAPILookup: lookup,
	}

	var result *updateResult
	if opts.DryRun {
		result, err = p.getPatchContents(remObj, obj)
	} else {
		result, err = p.patch(remObj, obj)
	}
	return result, err
}
