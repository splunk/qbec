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

// Package datasource provides a mechanism to create data sources from URLs with custom schemes.
package datasource

import (
	"fmt"
	"net/url"

	"github.com/pkg/errors"
	"github.com/splunk/qbec/internal/datasource/api"
	"github.com/splunk/qbec/internal/datasource/exec"
	"github.com/splunk/qbec/internal/datasource/exechttp"
)

// Create creates a new data source from the supplied URL.
// Such a URL has a scheme that is the type of supported data source and
// a hostname that is the name that it should be referred to in user code.
// Additional paths and query parameters are implementation specific and interpreted as documented for
// each data source.
func Create(u string) (api.DataSource, error) {
	parsed, err := url.Parse(u)
	if err != nil {
		return nil, errors.Wrapf(err, "parse URL '%s'", u)
	}
	switch parsed.Scheme {
	case exechttp.Scheme:
		return exechttp.FromURL(u)
	case exec.Scheme:
		return exec.FromURL(u)
	default:
		return nil, fmt.Errorf("data source URL '%s', unsupported scheme '%s'", u, parsed.Scheme)
	}
}
