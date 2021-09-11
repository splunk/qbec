/*
   Copyright 2021 Splunk Inc.
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

// Package dsfactory provides a mechanism to create data sources from URLs with custom schemes.
package dsfactory

import (
	"fmt"
	"net/url"

	"github.com/pkg/errors"
	"github.com/splunk/qbec/vm/datasource"
	"github.com/splunk/qbec/vm/internal/dsexec"
)

// Create creates a new data source from the supplied URL.
// Such a URL has a scheme that is the type of supported data source,
// a hostname that is the name that it should be referred to in user code,
// and a query param called configVar which supplies the data source config.
func Create(u string) (datasource.WithLifecycle, error) {
	parsed, err := url.Parse(u)
	if err != nil {
		return nil, errors.Wrapf(err, "parse URL '%s'", u)
	}
	scheme := parsed.Scheme
	switch scheme {
	case dsexec.Scheme:
	default:
		return nil, fmt.Errorf("data source URL '%s', unsupported scheme '%s'", u, scheme)
	}
	name := parsed.Host
	if name == "" {
		if parsed.Opaque != "" {
			return nil, fmt.Errorf("data source '%s' does not have a name (did you forget the '//' after the ':'?)", u)
		}
		return nil, fmt.Errorf("data source '%s' does not have a name", u)
	}
	varName := parsed.Query().Get("configVar")
	if varName == "" {
		return nil, fmt.Errorf("data source '%s' must have a configVar param", u)
	}
	switch scheme {
	case dsexec.Scheme:
		return makeLazy(dsexec.New(name, varName)), nil
	default:
		return nil, fmt.Errorf("internal error: unable to create a data source for %s", u)
	}
}
