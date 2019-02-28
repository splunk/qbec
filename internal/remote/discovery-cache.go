// Copyright 2017 The kubecfg authors
//
//
//    Licensed under the Apache License, Version 2.0 (the "License");
//    you may not use this file except in compliance with the License.
//    You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
//    Unless required by applicable law or agreed to in writing, software
//    distributed under the License is distributed on an "AS IS" BASIS,
//    WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
//    See the License for the specific language governing permissions and
//    limitations under the License.

package remote

// copied from kubecfg and modified as needed.

import (
	"sync"

	"github.com/emicklei/go-restful/swagger"
	"github.com/googleapis/gnostic/OpenAPIv2"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/version"
	"k8s.io/client-go/discovery"
	"k8s.io/client-go/rest"
)

type cacheDiscoveryClient struct {
	cl              discovery.DiscoveryInterface
	lock            sync.RWMutex
	serverGroups    *metav1.APIGroupList
	serverResources map[string]*metav1.APIResourceList
	schemas         map[string]*swagger.ApiDeclaration
	schema          *openapi_v2.Document
}

// newCachedDiscoveryClient creates a new DiscoveryClient that caches results in memory
func newCachedDiscoveryClient(cl discovery.DiscoveryInterface) discovery.CachedDiscoveryInterface {
	c := &cacheDiscoveryClient{cl: cl}
	c.Invalidate()
	return c
}

func (c *cacheDiscoveryClient) Fresh() bool {
	return true
}

func (c *cacheDiscoveryClient) Invalidate() {
	c.lock.Lock()
	defer c.lock.Unlock()

	c.serverGroups = nil
	c.serverResources = make(map[string]*metav1.APIResourceList)
	c.schemas = make(map[string]*swagger.ApiDeclaration)
}

func (c *cacheDiscoveryClient) RESTClient() rest.Interface {
	return c.cl.RESTClient()
}

func (c *cacheDiscoveryClient) ServerGroups() (*metav1.APIGroupList, error) {
	c.lock.Lock()
	defer c.lock.Unlock()

	var err error
	if c.serverGroups != nil {
		return c.serverGroups, nil
	}
	c.serverGroups, err = c.cl.ServerGroups()
	return c.serverGroups, err
}

func (c *cacheDiscoveryClient) ServerResourcesForGroupVersion(groupVersion string) (*metav1.APIResourceList, error) {
	c.lock.Lock()
	defer c.lock.Unlock()

	var err error
	if v := c.serverResources[groupVersion]; v != nil {
		return v, nil
	}
	c.serverResources[groupVersion], err = c.cl.ServerResourcesForGroupVersion(groupVersion)
	return c.serverResources[groupVersion], err
}

func (c *cacheDiscoveryClient) ServerResources() ([]*metav1.APIResourceList, error) {
	return c.cl.ServerResources()
}

func (c *cacheDiscoveryClient) ServerPreferredResources() ([]*metav1.APIResourceList, error) {
	return c.cl.ServerPreferredResources()
}

func (c *cacheDiscoveryClient) ServerPreferredNamespacedResources() ([]*metav1.APIResourceList, error) {
	return c.cl.ServerPreferredNamespacedResources()
}

func (c *cacheDiscoveryClient) ServerVersion() (*version.Info, error) {
	return c.cl.ServerVersion()
}

func (c *cacheDiscoveryClient) OpenAPISchema() (*openapi_v2.Document, error) {
	c.lock.Lock()
	defer c.lock.Unlock()

	if c.schema != nil {
		return c.schema, nil
	}

	sch, err := c.cl.OpenAPISchema()
	if err != nil {
		return nil, err
	}

	c.schema = sch
	return sch, nil
}

var _ discovery.CachedDiscoveryInterface = &cacheDiscoveryClient{}
