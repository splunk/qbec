---
title: Metadata for K8s objects
weight: 12
---

## Labels

All Kubernetes objects produced by qbec have the following labels associated with them:

* `qbec.io/application` - the app name from `qbec.yaml`.
* `qbec.io/environment` - the environment name in `qbec.yaml` for which the object was created.
* `qbec.io/tag` - the `--app-tag` parameter passed in on the command line. This label is only set when non-blank.

The labels are used to efficiently find all cluster objects for a specific app and environment
(and tag, if specified) for garbage collection. 

{{% notice note %}}
If you rename an app, environment, or component, garbage collection for the next immediate run of `qbec apply` may
not work correctly. Subsequent apply operations will then work as usual since the object labels will be updated with
the new values.
{{% /notice %}}

## Annotations

All Kubernetes objects produced by qbec have the following annotation associated with them:

* `qbec.io/last-applied` - this is the pristine version of the object stored for the purposes of diff and 3-way merge
  patches and plays the same role as the `kubectl.kubernetes.io/last-applied-configuration` annotation set by `kubectl apply`.
* `qbec.io/component` - the component that created the object. This is derived from the file name of the component.

The component annotation is used to respect component filters for `apply` and  `delete` operations.
Specifically, if `apply` is being run with component filters, only the extra remote objects matching the filter are
garbage-collected.

{{% notice note %}}
If you are using qbec to update an object that was created by another tool, you may see strange diffs for the very first time when
this annotation is missing. Once applied, the annotation will now be in place and subsequent updates will show cleaner
diffs.
{{% /notice %}}


