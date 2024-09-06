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

// Package factory provides a mechanism to create data sources from URLs with custom schemes.
package factory

import (
	"fmt"
	"net/url"

	"github.com/pkg/errors"
	"github.com/splunk/qbec/vm/internal/ds"
	"github.com/splunk/qbec/vm/internal/ds/exec"
	"github.com/splunk/qbec/vm/internal/ds/helm3"
)

// Create creates a new data source from the supplied URL.
// Such a URL has a scheme that is the type of supported data source,
// a hostname that is the name that it should be referred to in user code,
// and a query param called configVar which supplies the data source config.
func Create(u string) (ds.DataSourceWithLifecycle, error) {
	parsed, err := url.Parse(u)
	if err != nil {
		return nil, errors.Wrapf(err, "parse URL '%s'", u)
	}
	scheme := parsed.Scheme
	switch scheme {
	case exec.Scheme:
	case helm3.Scheme:
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
	case exec.Scheme:
		return makeLazy(exec.New(name, varName)), nil
	case helm3.Scheme:
		return makeLazy(helm3.New(name, varName)), nil
	default:
		return nil, fmt.Errorf("internal error: unable to create a data source for %s", u)
	}
}
