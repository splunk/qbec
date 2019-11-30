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

package commands

import (
	"sort"
	"strings"

	"github.com/splunk/qbec/internal/model"
	"github.com/splunk/qbec/internal/sio"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

const (
	policyNever   = "never"
	policyDefault = "default"
)

// isSet return true if the annotation name specified as directive is equal to the supplied value.
// The allowedValues parameter specifies what other values are allowed and causes a warning if
// the annotation exists but doesn't have one of the allowed values.
func isSet(ob model.K8sMeta, directive, value string, otherAllowedValues []string) bool {
	anns := ob.GetAnnotations()
	if anns != nil {
		v := anns[directive]
		if v == value {
			return true
		}
		if v != "" {
			found := false
			for _, allowed := range otherAllowedValues {
				if v == allowed {
					found = true
					break
				}
			}
			if !found {
				allVals := append([]string{value}, otherAllowedValues...)
				sort.Strings(allVals)
				sio.Warnf("ignored annotation %s=%s does not have one of the allowed values: %s\n", directive, v, strings.Join(allVals, ", "))
			}
		}
	}
	return false
}

type updatePolicy struct{}

func (u *updatePolicy) disableUpdate(ob model.K8sMeta) bool {
	return isSet(ob, model.QbecNames.Directives.UpdatePolicy, policyNever, []string{policyDefault})
}

func newUpdatePolicy() *updatePolicy {
	return &updatePolicy{}
}

type deletePolicy struct {
	nsFunc         func(kind schema.GroupVersionKind) (bool, error)
	defaultNS      string
	keepNamespaces map[string]bool
}

func newDeletePolicy(nsFunc func(kind schema.GroupVersionKind) (bool, error), defaultNS string) *deletePolicy {
	return &deletePolicy{
		nsFunc:    nsFunc,
		defaultNS: defaultNS,
		keepNamespaces: map[string]bool{
			"default":     true, // never try to delete the default namespace
			"kube-system": true, // ditto for system namespace
		}}
}

func (d *deletePolicy) disableDelete(ob model.K8sMeta) bool {
	ret := isSet(ob, model.QbecNames.Directives.DeletePolicy, policyNever, []string{policyDefault})
	if ret {
		isNamespaced, _ := d.nsFunc(ob.GroupVersionKind())
		if isNamespaced {
			ns := ob.GetNamespace()
			if ns == "" {
				ns = d.defaultNS
			}
			d.keepNamespaces[ns] = true
		}
		return true
	}
	if ob.GroupVersionKind().Group == "" && ob.GetKind() == "Namespace" {
		return d.keepNamespaces[ob.GetName()]
	}
	return false
}
