package rollout

import (
	"encoding/json"
	"fmt"
	"strconv"

	"github.com/pkg/errors"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

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

func deploymentStatus(base *unstructured.Unstructured, revision int64) (*ObjectStatus, error) {
	if err := revisionCheck(base, revision); err != nil {
		return nil, err
	}
	var d struct {
		Metadata struct {
			Generation      int64  `json:"generation"`
			ResourceVersion string `json:"resourceVersion"`
		} `json:"metadata"`
		Spec struct {
			Replicas int32 `json:"replicas"`
		} `json:"spec"`
		Status struct {
			ObservedGeneration int64 `json:"observedGeneration"`
			Replicas           int32 `json:"replicas"`
			AvailableReplicas  int32 `json:"availableReplicas"`
			UpdatedReplicas    int32 `json:"updatedReplicas"`
			Conditions         []struct {
				Type   string `json:"type"`
				Reason string `json:"reason,omitempty"`
			} `json:"conditions"`
		} `json:"status"`
	}
	if err := reserialize(base, &d); err != nil {
		return nil, err
	}

	var ret ObjectStatus

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

func daemonsetStatus(base *unstructured.Unstructured, _ int64) (*ObjectStatus, error) {
	var d struct {
		Metadata struct {
			Generation      int64  `json:"generation"`
			ResourceVersion string `json:"resourceVersion"`
		} `json:"metadata"`
		Spec struct {
			UpdateStrategy struct {
				Type string
			} `json:"updateStrategy"`
		} `json:"spec"`
		Status struct {
			DesiredNumberScheduled int32 `json:"desiredNumberScheduled"`
			UpdatedNumberScheduled int32 `json:"updatedNumberScheduled"`
			NumberAvailable        int32 `json:"numberAvailable"`
			ObservedGeneration     int64 `json:"observedGeneration"`
		} `json:"status"`
	}
	if err := reserialize(base, &d); err != nil {
		return nil, err
	}

	var ret ObjectStatus
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

func statefulsetStatus(base *unstructured.Unstructured, _ int64) (*ObjectStatus, error) {
	var d struct {
		Metadata struct {
			Generation      int64  `json:"generation"`
			ResourceVersion string `json:"resourceVersion"`
		} `json:"metadata"`
		Spec struct {
			UpdateStrategy struct {
				Type          string
				RollingUpdate struct {
					Partition *int32 `json:"partition"`
				} `json:"rollingUpdate"`
			} `json:"updateStrategy"`
			Replicas *int32 `json:"replicas"`
		} `json:"spec"`
		Status struct {
			UpdatedReplicas    int32  `json:"updatedReplicas"`
			ReadyReplicas      int32  `json:"readyReplicas"`
			CurrentRevision    string `json:"currentRevision"`
			UpdateRevision     string `json:"updateRevision"`
			ObservedGeneration int64  `json:"observedGeneration"`
		} `json:"status"`
	}
	if err := reserialize(base, &d); err != nil {
		return nil, err
	}

	var ret ObjectStatus

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
