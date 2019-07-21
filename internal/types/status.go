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

package types

import (
	"encoding/json"
	"fmt"
	"strconv"

	"github.com/pkg/errors"
	"github.com/splunk/qbec/internal/model"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

// this file contains the logic of extracting rollout status from specific k8s object types.
// Logic is kubectl logic but code is our own. In particular we declare the relevant
// attributes of the objects we need instead of using the code-generated types.

// RolloutStatus is the opaque rollout status of an object.
type RolloutStatus struct {
	Description string // the description of status for display
	Done        bool   // indicator if the status is "ready"
}

func (s *RolloutStatus) withDesc(desc string) *RolloutStatus {
	s.Description = desc
	return s
}

func (s *RolloutStatus) withDone(done bool) *RolloutStatus {
	s.Done = done
	return s
}

// RolloutStatusFunc returns the rollout status for the supplied object.
type RolloutStatusFunc func(obj *unstructured.Unstructured, revision int64) (status *RolloutStatus, err error)

// StatusFuncFor returns the status function for the specified object or nil
// if a status function does not exist for it.
func StatusFuncFor(obj model.K8sMeta) RolloutStatusFunc {
	gk := obj.GroupVersionKind().GroupKind()
	switch gk {
	case schema.GroupKind{Group: "apps", Kind: "Deployment"},
		schema.GroupKind{Group: "extensions", Kind: "Deployment"}:
		return deploymentStatus
	case schema.GroupKind{Group: "apps", Kind: "DaemonSet"},
		schema.GroupKind{Group: "extensions", Kind: "DaemonSet"}:
		return daemonsetStatus
	case schema.GroupKind{Group: "apps", Kind: "StatefulSet"}:
		return statefulsetStatus
	default:
		return nil
	}
}

func reserialize(un *unstructured.Unstructured, target interface{}) error {
	b, err := json.Marshal(un)
	if err != nil {
		return errors.Wrap(err, "json marshal")
	}
	if err := json.Unmarshal(b, target); err != nil {
		return errors.Wrap(err, "json unmarshal")
	}
	return nil
}

func revisionCheck(base *unstructured.Unstructured, revision int64) error {
	getRevision := func() (int64, error) {
		v, ok := base.GetAnnotations()["deployment.kubernetes.io/revision"]
		if !ok {
			return 0, nil
		}
		return strconv.ParseInt(v, 10, 64)
	}
	if revision > 0 {
		deploymentRev, err := getRevision()
		if err != nil {
			return errors.Wrap(err, "get revision")
		}
		if revision != deploymentRev {
			return fmt.Errorf("desired revision (%d) is different from the running revision (%d)", revision, deploymentRev)
		}
	}
	return nil
}

func deploymentStatus(base *unstructured.Unstructured, revision int64) (*RolloutStatus, error) {
	if err := revisionCheck(base, revision); err != nil {
		return nil, err
	}
	var d struct {
		Metadata struct {
			Generation      int64
			ResourceVersion string
		}
		Spec struct {
			Replicas int32
		}
		Status struct {
			ObservedGeneration int64
			Replicas           int32
			AvailableReplicas  int32
			UpdatedReplicas    int32
			Conditions         []struct {
				Type   string
				Reason string
			}
		}
	}
	if err := reserialize(base, &d); err != nil {
		return nil, err
	}

	var ret RolloutStatus

	if d.Metadata.Generation > d.Status.ObservedGeneration {
		return ret.withDesc(fmt.Sprintf("waiting for spec update to be observed")), nil
	}

	for _, c := range d.Status.Conditions {
		if c.Type == "Progressing" && c.Reason == "ProgressDeadlineExceeded" {
			return nil, fmt.Errorf("deployment exceeded progress deadline")
		}
	}

	if d.Status.UpdatedReplicas < d.Spec.Replicas {
		return ret.withDesc(fmt.Sprintf("%d out of %d new replicas have been updated", d.Status.UpdatedReplicas, d.Spec.Replicas)), nil
	}
	if d.Status.Replicas > d.Status.UpdatedReplicas {
		return ret.withDesc(fmt.Sprintf("%d old replicas are pending termination", d.Status.Replicas-d.Status.UpdatedReplicas)), nil
	}
	if d.Status.AvailableReplicas < d.Status.UpdatedReplicas {
		return ret.withDesc(fmt.Sprintf("%d of %d updated replicas are available", d.Status.AvailableReplicas, d.Status.UpdatedReplicas)), nil
	}
	return ret.withDone(true).withDesc("successfully rolled out"), nil
}

func daemonsetStatus(base *unstructured.Unstructured, _ int64) (*RolloutStatus, error) {
	var d struct {
		Metadata struct {
			Generation      int64
			ResourceVersion string
		}
		Spec struct {
			UpdateStrategy struct {
				Type string
			}
		}
		Status struct {
			DesiredNumberScheduled int32
			UpdatedNumberScheduled int32
			NumberAvailable        int32
			ObservedGeneration     int64
		}
	}
	if err := reserialize(base, &d); err != nil {
		return nil, err
	}

	var ret RolloutStatus
	if d.Spec.UpdateStrategy.Type != "RollingUpdate" {
		return ret.withDone(true).withDesc(fmt.Sprintf("skip rollout check for daemonset (strategy=%s)", d.Spec.UpdateStrategy.Type)), nil
	}

	if d.Metadata.Generation > d.Status.ObservedGeneration {
		return ret.withDesc("waiting for spec update to be observed"), nil
	}

	if d.Status.UpdatedNumberScheduled < d.Status.DesiredNumberScheduled {
		return ret.withDesc(fmt.Sprintf("%d out of %d new pods have been updated", d.Status.UpdatedNumberScheduled, d.Status.DesiredNumberScheduled)), nil
	}
	if d.Status.NumberAvailable < d.Status.DesiredNumberScheduled {
		return ret.withDesc(fmt.Sprintf("%d of %d updated pods are available", d.Status.NumberAvailable, d.Status.DesiredNumberScheduled)), nil
	}
	return ret.withDone(true).withDesc("successfully rolled out"), nil
}

func statefulsetStatus(base *unstructured.Unstructured, _ int64) (*RolloutStatus, error) {
	var d struct {
		Metadata struct {
			Generation      int64
			ResourceVersion string
		}
		Spec struct {
			UpdateStrategy struct {
				Type          string
				RollingUpdate struct {
					Partition *int32
				}
			}
			Replicas *int32
		}
		Status struct {
			UpdatedReplicas    int32
			ReadyReplicas      int32
			CurrentRevision    string
			UpdateRevision     string
			ObservedGeneration int64
		}
	}
	if err := reserialize(base, &d); err != nil {
		return nil, err
	}

	var ret RolloutStatus

	if d.Spec.UpdateStrategy.Type != "RollingUpdate" {
		return ret.withDone(true).withDesc(fmt.Sprintf("skip rollout check for stateful set (strategy=%s)", d.Spec.UpdateStrategy.Type)), nil
	}

	if d.Metadata.Generation > d.Status.ObservedGeneration {
		return ret.withDesc("waiting for spec update to be observed"), nil
	}

	if d.Spec.Replicas != nil && d.Spec.UpdateStrategy.RollingUpdate.Partition != nil {
		newPodsNeeded := *d.Spec.Replicas - *d.Spec.UpdateStrategy.RollingUpdate.Partition
		if d.Status.UpdatedReplicas < newPodsNeeded {
			return ret.withDesc(fmt.Sprintf("%d of %d updated", d.Status.UpdatedReplicas, newPodsNeeded)), nil
		}
		return ret.withDone(true).withDesc(fmt.Sprintf("%d new pods updated", newPodsNeeded)), nil
	}

	if d.Status.UpdateRevision != d.Status.CurrentRevision {
		return ret.withDesc(fmt.Sprintf("%d pods at revision %s", d.Status.UpdatedReplicas, d.Status.UpdateRevision)), nil
	}
	return ret.withDone(true).withDesc("successfully rolled out"), nil
}
