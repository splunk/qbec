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
	"bytes"
	"fmt"
	"testing"

	"github.com/splunk/qbec/internal/model"
	"github.com/splunk/qbec/internal/rollout"
	"github.com/splunk/qbec/internal/sio"
	"github.com/stretchr/testify/assert"
)

func testDisplayName(obj model.K8sMeta) string {
	return fmt.Sprintf("%s/%s %s/%s", obj.GroupVersionKind().Group, obj.GroupVersionKind().Kind, obj.GetNamespace(), obj.GetName())
}

func testDeployment(name string) model.K8sMeta {
	return model.NewK8sObject(map[string]interface{}{
		"apiVersion": "apps/v1",
		"kind":       "Deployment",
		"metadata": map[string]interface{}{
			"namespace": "test-ns",
			"name":      name,
		},
	})
}

func TestWaitListener(t *testing.T) {
	var buf bytes.Buffer
	oldOutput, oldColors := sio.Output, sio.EnableColors
	defer func() {
		sio.Output = oldOutput
		sio.EnableColors = oldColors
	}()
	sio.Output = &buf
	sio.EnableColors = false

	d1, d2, d3 := testDeployment("d1"), testDeployment("d2"), testDeployment("d3")
	wl := &waitListener{displayNameFn: testDisplayName}
	wl.OnInit([]model.K8sMeta{d1, d2, d3})
	wl.OnStatusChange(d1, rollout.ObjectStatus{Description: "starting d1 rollout"})
	wl.OnStatusChange(d2, rollout.ObjectStatus{Description: "1 of 2 replicas updated"})
	wl.OnStatusChange(d3, rollout.ObjectStatus{Description: "waiting for version"})
	wl.OnStatusChange(d1, rollout.ObjectStatus{Description: "successful rollout", Done: true})
	wl.OnStatusChange(d2, rollout.ObjectStatus{Description: "successful rollout", Done: true})
	wl.OnStatusChange(d3, rollout.ObjectStatus{Description: "successful rollout", Done: true})

	wl.OnEnd(nil)

	output := buf.String()
	a := assert.New(t)
	a.Contains(output, "waiting for readiness of 3 objects")
	a.Contains(output, "- apps/Deployment test-ns/d1")
	a.Contains(output, "0s    : apps/Deployment test-ns/d1 :: starting d1 rollout")
	a.Contains(output, "✓ 0s    : apps/Deployment test-ns/d1 :: successful rollout (2 remaining)")
	a.Contains(output, "rollout complete")
}

func TestWaitListenerTimeout(t *testing.T) {
	var buf bytes.Buffer
	oldOutput, oldColors := sio.Output, sio.EnableColors
	defer func() {
		sio.Output = oldOutput
		sio.EnableColors = oldColors
	}()
	sio.Output = &buf
	sio.EnableColors = false

	d1, d2, d3 := testDeployment("d1"), testDeployment("d2"), testDeployment("d3")
	wl := &waitListener{displayNameFn: testDisplayName}
	wl.OnInit([]model.K8sMeta{d1, d2, d3})
	wl.OnStatusChange(d1, rollout.ObjectStatus{Description: "starting d1 rollout"})
	wl.OnStatusChange(d2, rollout.ObjectStatus{Description: "1 of 2 replicas updated"})
	wl.OnStatusChange(d3, rollout.ObjectStatus{Description: "waiting for version"})
	wl.OnStatusChange(d1, rollout.ObjectStatus{Description: "successful rollout", Done: true})
	wl.OnStatusChange(d2, rollout.ObjectStatus{Description: "successful rollout", Done: true})
	wl.OnError(d3, fmt.Errorf("d3 missing"))

	wl.OnEnd(fmt.Errorf("1 error"))

	output := buf.String()
	a := assert.New(t)
	a.Contains(output, "waiting for readiness of 3 objects")
	a.Contains(output, "- apps/Deployment test-ns/d1")
	a.Contains(output, "0s    : apps/Deployment test-ns/d1 :: starting d1 rollout")
	a.Contains(output, "✓ 0s    : apps/Deployment test-ns/d1 :: successful rollout (2 remaining)")
	a.Contains(output, "✘ 0s    : apps/Deployment test-ns/d3 :: d3 missing")
	a.Contains(output, "rollout not complete for the following 1 object")
}

func TestWaitWatcher(t *testing.T) {

}
