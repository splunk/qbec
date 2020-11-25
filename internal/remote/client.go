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
	"time"

	"github.com/ghodss/yaml"
	"github.com/jonboulle/clockwork"
	"github.com/pkg/errors"
	"github.com/splunk/qbec/internal/model"
	"github.com/splunk/qbec/internal/remote/k8smeta"
	"github.com/splunk/qbec/internal/sio"
	"github.com/splunk/qbec/internal/types"
	apiErrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	apiTypes "k8s.io/apimachinery/pkg/types"
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
	errMetadataNotFound = errors.New("server type not found") // returned when metadata could not be found for a gvk
)

// this file contains the client definition and supported CRUD operations.

// TypeWaitOptions are options for waiting on a custom type.
type TypeWaitOptions struct {
	Timeout time.Duration // the total time to wait
	Poll    time.Duration // poll interval
}

// ConditionFunc returns if a specific condition tests as true for the supplied object.
type ConditionFunc func(obj model.K8sMeta) bool

// SyncOptions provides the caller with options for the sync operation.
type SyncOptions struct {
	DryRun          bool            // do not actually create or update objects, return what would happen
	DisableCreate   bool            // only update objects if they exist, do not create new ones
	DisableUpdateFn ConditionFunc   // do not update an existing object
	WaitOptions     TypeWaitOptions // opts for waiting
	ShowSecrets     bool            // show secrets in patches and creations
}

// DeleteOptions provides the caller with options for the delete operation.
type DeleteOptions struct {
	DryRun          bool          // do not actually delete, return what would happen
	DisableDeleteFn ConditionFunc // test to see if deletion should be disabled.
}

type internalSyncOptions struct {
	secretDryRun       bool               // dry-run phase for objects having secrets info
	pristiner          pristineReadWriter // pristine writer
	pristineAnnotation string             // pristine annotation to manipulate for secrets dry-run
}

type resourceClient interface {
	clientForGroupVersionKind(kind schema.GroupVersionKind) (dynamic.Interface, error)
}

// Client is a thick remote client that provides high-level operations for commands as opposed to
// granular ones.
type Client struct {
	resources *k8smeta.Resources        // the server metadata loaded once and never updated
	schema    *k8smeta.ServerSchema     // the server schema
	pool      resourceClient            // the client pool for resource interfaces
	disco     k8smeta.ResourceDiscovery // the discovery interface
	defaultNs string                    // the default namespace to set for namespaced objects that do not define one
	verbosity int                       // log verbosity
}

func newClient(pool resourceClient, disco discovery.DiscoveryInterface, ns string, verbosity int) (*Client, error) {
	start := time.Now()
	resources, err := k8smeta.NewResources(disco, k8smeta.ResourceOpts{WarnFn: sio.Warnln})
	if err != nil {
		return nil, errors.Wrap(err, "get server metadata")
	}
	if verbosity > 0 {
		resources.Dump(sio.Debugln)
	}
	duration := time.Since(start).Round(time.Millisecond)
	sio.Debugln("cluster metadata load took", duration)

	ss := k8smeta.NewServerSchema(disco)
	c := &Client{
		resources: resources,
		schema:    ss,
		pool:      pool,
		disco:     disco,
		defaultNs: ns,
		verbosity: verbosity,
	}
	return c, nil
}

// ValidatorFor returns a validator for the supplied group version kind.
func (c *Client) ValidatorFor(gvk schema.GroupVersionKind) (k8smeta.Validator, error) {
	return c.schema.ValidatorFor(gvk)
}

// objectNamespace returns the namespace for the specified object. It returns a blank
// string when the object is cluster-scoped. For namespace-scoped objects it returns
// the default namespace when the object does not have one set. It does not fail if the
// object type is not known and just returns whatever is specified for the object.
func (c *Client) objectNamespace(o model.K8sMeta) string {
	info := c.resources.APIResource(o.GroupVersionKind())
	ns := o.GetNamespace()
	if info != nil {
		if info.Namespaced {
			if ns == "" {
				ns = c.defaultNs
			}
		} else {
			ns = ""
		}
	}
	return ns
}

// DisplayName returns the display name of the supplied K8s object.
func (c *Client) DisplayName(o model.K8sMeta) string {
	sm := c.resources
	gvk := o.GroupVersionKind()
	info := sm.APIResource(gvk)

	displayType := func() string {
		if info != nil {
			return info.Name
		}
		return strings.ToLower(gvk.Kind)
	}

	displayName := func() string {
		ns := c.objectNamespace(o)
		name := model.NameForDisplay(o)
		if ns == "" {
			return name
		}
		return name + " -n " + ns
	}
	name := fmt.Sprintf("%s %s", displayType(), displayName())
	if l, ok := o.(model.K8sLocalObject); ok {
		comp := l.Component()
		if comp != "" {
			name += fmt.Sprintf(" (source %s)", comp)
		}
	}
	return name
}

func (c *Client) apiResourceFor(gvk schema.GroupVersionKind) (*metav1.APIResource, error) {
	info := c.resources.APIResource(gvk)
	if info == nil {
		return nil, fmt.Errorf("resource not found for %s/%s %s", gvk.Group, gvk.Version, gvk.Kind)
	}
	return info, nil
}

// IsNamespaced returns if the supplied group version kind is namespaced.
func (c *Client) IsNamespaced(gvk schema.GroupVersionKind) (bool, error) {
	res, err := c.apiResourceFor(gvk)
	if err != nil {
		return false, err
	}
	return res.Namespaced, nil
}

func (c *Client) canonicalGroupVersionKind(in schema.GroupVersionKind) (schema.GroupVersionKind, error) {
	return c.resources.CanonicalGroupVersionKind(in)
}

// Get returns the remote object matching the supplied metadata as an unstructured bag of attributes.
func (c *Client) Get(obj model.K8sMeta) (*unstructured.Unstructured, error) {
	rc, err := c.resourceInterfaceWithDefaultNs(obj.GroupVersionKind(), obj.GetNamespace())
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

// ObjectKey returns a string key for the supplied object that includes its group-kind,
// namespace and name. Input values are used in case canonical values cannot be derived
// (e.g. for custom resources whose CRDs haven't yet been created).
func (c *Client) ObjectKey(obj model.K8sMeta) string {
	gvk := obj.GroupVersionKind()
	if canon, err := c.resources.CanonicalGroupVersionKind(gvk); err == nil {
		gvk = canon
	}
	ns := c.objectNamespace(obj)
	return fmt.Sprintf("%s:%s:%s:%s", gvk.Group, gvk.Kind, ns, obj.GetName())
}

// ListQueryScope defines the scope at which list queries need to be executed.
type ListQueryScope struct {
	Namespaces     []string // namespaces of interest
	ClusterObjects bool     // whether to query for cluster objects
}

// GVKFilter returns true if a gvk needs to be processed
type GVKFilter func(gvk schema.GroupVersionKind) bool

// ListQueryConfig is the config with which to execute list queries.
type ListQueryConfig struct {
	Application        string    // must be non-blank
	Tag                string    // may be blank
	Environment        string    // must be non-blank
	ListQueryScope               // the query scope for namespaces and non-namespaced resources
	KindFilter         GVKFilter // filters for group version kind
	Concurrency        int       // concurrent queries to execute
	ClusterScopedLists bool      // perform list queries across namespaces when multiple namespaces in picture
}

// Collection represents a set of k8s objects with the ability to remove a subset of objects from it.
type Collection interface {
	Remove(obj []model.K8sQbecMeta) error // remove all objects represented by the input list
	ToList() []model.K8sQbecMeta          // return a list of remaining objects
}

// ListObjects returns all objects for the application and environment for the namespace /cluster scopes
// and kind filtering indicated by the query configuration.
func (c *Client) ListObjects(scope ListQueryConfig) (Collection, error) {
	if scope.KindFilter == nil {
		scope.KindFilter = func(_ schema.GroupVersionKind) bool { return true }
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

	var namespacedTypes, clusterTypes []schema.GroupVersionKind
	for _, v := range c.resources.CanonicalResources() {
		gvk := schema.GroupVersionKind{Group: v.Group, Version: v.Version, Kind: v.Kind}
		if v.Namespaced {
			namespacedTypes = append(namespacedTypes, gvk)
		} else {
			clusterTypes = append(clusterTypes, gvk)
		}
	}

	qc := queryConfig{
		scope:            scope,
		resourceProvider: c.ResourceInterface,
		namespacedTypes:  filterEligibleTypes(namespacedTypes),
		clusterTypes:     filterEligibleTypes(clusterTypes),
		verbosity:        c.verbosity,
	}
	ol := objectLister{qc}
	coll := newCollection(c.defaultNs, c)
	if err := ol.serverObjects(coll); err != nil {
		return nil, err
	}
	return coll, nil
}

type updateResult struct {
	SkipReason    string             `json:"skip,omitempty"`
	Operation     string             `json:"operation,omitempty"`
	Source        string             `json:"source,omitempty"`
	Kind          apiTypes.PatchType `json:"kind,omitempty"`
	DisplayPatch  string             `json:"patch,omitempty"`
	GeneratedName string             `json:"generatedName,omitempty"`
	patch         []byte
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
			Type:          SyncCreated,
			GeneratedName: u.GeneratedName, // only set when name actually generated
			Details:       u.String(),
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
	Type          SyncResultType // the result type
	GeneratedName string         // the actual name of an object that has generateName set
	Details       string         // additional details that are safe to print to console (e.g. no secrets)
}

func (c *Client) ensureType(gvk schema.GroupVersionKind, opts SyncOptions) error {
	if _, err := c.apiResourceFor(gvk); err == nil {
		return nil
	}
	waitTime := opts.WaitOptions.Timeout
	if waitTime == 0 {
		waitTime = 2 * time.Minute
	}
	end := time.Now().Add(waitTime)

	waitPoll := opts.WaitOptions.Poll
	if waitPoll == 0 {
		waitPoll = 2 * time.Second
	}
	first := true
	for {
		_, err := c.jitResource(gvk)
		if err == nil {
			return nil
		}
		if first {
			first = false
			sio.Noticef("waiting for type %s to be available for up to %s\n", gvk, waitTime.Round(time.Second))
		}
		if time.Now().After(end) {
			return err
		}
		time.Sleep(waitPoll)
	}
}

// Sync syncs the local object by either creating a new one or patching an existing one.
// It does not do anything in dry-run mode. It also does not create new objects if the caller has disabled the feature.
func (c *Client) Sync(original model.K8sLocalObject, opts SyncOptions) (_ *SyncResult, finalError error) {
	// set up the pristine strategy.
	var prw pristineReadWriter = qbecPristine{}
	sensitive := types.HasSensitiveInfo(original.ToUnstructured())

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
			finalError = errors.Wrap(finalError, "sync "+c.DisplayName(original))
		}
	}()

	if !opts.DryRun {
		if err := c.ensureType(original.GroupVersionKind(), opts); err != nil {
			return nil, err
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
	var remObj *unstructured.Unstructured
	var objErr error
	if original.GetName() != "" {
		remObj, objErr = c.Get(original)
	}
	switch {
	// empty name, always create
	case original.GetName() == "":
		break
	// ignore object not found errors
	case objErr == ErrNotFound:
		break
	// treat metadata errors (server type not found) as a "not found" error if dry-run mode is active
	case objErr == errMetadataNotFound && opts.DryRun:
		break
	// report all other errors
	case objErr != nil:
		return nil, errors.Wrap(objErr, "get object")
	}

	var obj model.K8sLocalObject
	if internal.secretDryRun {
		opts.DryRun = true // won't affect caller since passed by value
		obj, _ = types.HideSensitiveLocalInfo(original)
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
			c, _ := types.HideSensitiveInfo(remObj)
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
func (c *Client) Delete(obj model.K8sMeta, opts DeleteOptions) (_ *SyncResult, finalError error) {
	if opts.DisableDeleteFn(obj) {
		upr := &updateResult{
			SkipReason: "deletion disabled due to user request",
		}
		return upr.toSyncResult(), nil
	}
	ret := &SyncResult{
		Type: SyncDeleted,
	}
	if opts.DryRun {
		return ret, nil
	}
	defer func() {
		if finalError != nil {
			finalError = errors.Wrap(finalError, "delete "+c.DisplayName(obj))
		}
	}()

	ri, err := c.resourceInterfaceWithDefaultNs(obj.GroupVersionKind(), obj.GetNamespace())
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
			clone := r
			clone.Group = gvk.Group
			clone.Version = gvk.Version
			return &clone, nil
		}
	}
	return nil, fmt.Errorf("server does not recognize gvk %s", gvk)
}

// ResourceInterface returns a dynamic resource interface for the supplied group version kind and namespace.
func (c *Client) ResourceInterface(gvk schema.GroupVersionKind, namespace string) (dynamic.ResourceInterface, error) {
	client, err := c.pool.clientForGroupVersionKind(gvk)
	if err != nil {
		return nil, err
	}
	res, err := c.apiResourceFor(gvk)
	if err != nil { // could be a resource for a CRD that was just created, re-query discovery
		res, err = c.jitResource(gvk)
		if err != nil {
			return nil, errMetadataNotFound
		}
	}
	base := client.Resource(schema.GroupVersionResource{
		Group:    res.Group,
		Version:  res.Version,
		Resource: res.Name,
	})
	var ret dynamic.ResourceInterface
	if res.Namespaced {
		ret = base.Namespace(namespace)
	} else {
		ret = base
	}
	return ret, nil
}

func (c *Client) resourceInterfaceWithDefaultNs(gvk schema.GroupVersionKind, namespace string) (dynamic.ResourceInterface, error) {
	if namespace == "" {
		namespace = c.defaultNs
	}
	return c.ResourceInterface(gvk, namespace)
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
	ri, err := c.resourceInterfaceWithDefaultNs(obj.GroupVersionKind(), obj.GetNamespace())
	if err != nil {
		return nil, errors.Wrap(err, "get resource interface")
	}
	out, err := ri.Create(obj.ToUnstructured(), metav1.CreateOptions{})
	if err != nil {
		return nil, errors.Wrap(err, "create object")
	}
	if obj.GetName() == "" {
		result.GeneratedName = out.GetName()
	}
	return result, nil
}

func (c *Client) maybeUpdate(obj model.K8sLocalObject, remObj *unstructured.Unstructured, opts SyncOptions) (*updateResult, error) {
	if opts.DisableUpdateFn(model.NewK8sObject(remObj.Object)) {
		return &updateResult{
			SkipReason: "update disabled due to user request",
		}, nil
	}
	res, err := c.schema.OpenAPIResources()
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
