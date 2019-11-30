---
title: Controlling qbec behavior
weight: 27
---

qbec usually does the right thing when applying objects. Sometimes this behavior needs tweaking. Qbec allows you to 
annotate objects with specific directives to control its behavior. All annotations are in the `directives.qbec.io/`
namespace.

### Updating and deleting objects

Usually qbec will update and delete objects as required. You can "lock" objects from being updated or deleted using the
following annotations:

* `directives.qbec.io/update-policy: never`
* `directives.qbec.io/delete-policy: never`

The first is useful when dealing with jobs that typically should not be updated. The second is useful for preventing
storage objects like persistent volume claims from ever being deleted.

Note that `qbec` will look for these annotations on the object that is already present on the cluster and will ignore
the annotations from source code when implementing these policies. This ensures, for example, that you do not 
accidentally start updating and deleting objects by removing these annotations from source code.

In addition if you lock a namespaced object from being deleted, qbec will automatically ensure that the 
corresponding namespace, if it exists, is also never deleted.

### Controlling apply order

`qbec apply` evaluates all components in source code and internally assigns an "apply order" to every object. It then
sorts this list and applies objects in order, one at a time. The default ordering applies cluster objects before
namespaced objects, config maps and secrets before deployments etc.

For specific objects, you can control the apply order by setting the annotation `directives.qbec.io/apply-order` to a
numeric value as a string (e.g. `"1000"`).

This feature should be used with care since it is possible to get nonsensical ordering (e.g. trying
to apply a namespaced object before the namespace is created). The most common use of this feature is to provide a 
high number to delay processing of a specific object (typically a custom resource) until the end. Even in this case,
qbec default processing may be enough since it automatically delays the apply of resources having unknown types to the
end.

If you have many instances of a resource that needs a custom apply order, [consider using a post-processor](../common-metadata/)
to set the annotation for all instances of the type instead of annotating each instance.
 
