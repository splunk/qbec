---
title: Diffs and patches
weight: 400
---

qbec uses a 3-way merge patch similar to `kubectl/ksonnet apply`. The [Kubernetes documentation](https://kubernetes.io/docs/concepts/overview/object-management-kubectl/declarative-config/#how-apply-calculates-differences-and-merges-changes)
describes how this works.

For existing objects, the `qbec diff` command produces a diff between the last applied configuration stored
on the server and the current configuration of the object loaded from source. This diff is "clean" in the
sense of the remote object not having additional fields, default values and so on. 
It faithfully represents the change between the previous and current version of the object produced from
source code.

When `qbec apply` is run, it calculates the patch for existing objects. This calculation _does_ have to account for the
shape of the object as stored by Kubernetes. 

In many if not most cases, if `qbec diff` does not report a diff for an object, `qbec apply` will also
not try to update the object. 

This is not always true. Among other things:

* the local object may have fields that are never stored on the server and every run of `qbec apply` will
  attempt to update these extra fields. This is particularly noticeable for custom resources and definitions.
  
* the local object may represent a value differently from how the server stores it. For example a local
  CPU resource of `1000m` may be stored in the server as `1` instead. Every `qbec apply` will notice 
  this difference and try to update the value back to `1000m`.

## Summary

* Diffs and patches may not always agree on the number of objects that are different.
* Spurious apply patches can appear in the output. These can be noisy but they're benign.
  One way to fix this would be to check the YAML output from the server and try to match the source
  code to have the same representation of the value.


