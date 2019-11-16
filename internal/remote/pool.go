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
	"sync"

	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"
	restclient "k8s.io/client-go/rest"
)

// clientPoolImpl implements resourceClient and caches clients for the resource group versions
// is asked to retrieve. This type is thread safe.
type clientPoolImpl struct {
	lock                sync.RWMutex
	config              *restclient.Config
	clients             map[schema.GroupVersion]dynamic.Interface
	apiPathResolverFunc dynamic.APIPathResolverFunc
}

// newResourceClient instantiates a new dynamic client pool with the given config.
func newResourceClient(cfg *restclient.Config) resourceClient {
	confCopy := *cfg
	return &clientPoolImpl{
		config:              &confCopy,
		clients:             map[schema.GroupVersion]dynamic.Interface{},
		apiPathResolverFunc: dynamic.LegacyAPIPathResolverFunc,
	}
}

// ClientForGroupVersion returns a client for the specified groupVersion, creates one if none exists. Kind
// in the GroupVersionKind may be empty.
func (c *clientPoolImpl) clientForGroupVersionKind(kind schema.GroupVersionKind) (dynamic.Interface, error) {
	c.lock.Lock()
	defer c.lock.Unlock()

	gv := kind.GroupVersion()
	// do we have a client already configured?
	if existingClient, found := c.clients[gv]; found {
		return existingClient, nil
	}

	// avoid changing the original config
	confCopy := *c.config
	conf := &confCopy
	conf.APIPath = c.apiPathResolverFunc(kind)
	conf.GroupVersion = &gv

	dynamicClient, err := dynamic.NewForConfig(conf)
	if err != nil {
		return nil, err
	}
	c.clients[gv] = dynamicClient
	return dynamicClient, nil
}
