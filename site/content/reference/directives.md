---
title: Qbec directives
weight: 105
---
Annotations that you can use for your objects to control qbec behavior.

#### `directives.qbec.io/apply-order` 

* Annotation source: local object
* Allowed values: Any positive integer as a string (i.e. ensure that the value is quoted in YAML)
* Default value: `"0"` (use qbec defaults)

controls the order in which objects are applied. This allows you, for example, to move updates of a custom 
resource to after all other objects have been processed.

#### `directives.qbec.io/delete-policy` 

* Annotation source: in-cluster object
* Allowed values: `"default"`, `"never"`
* Default value: `"default"` 

when set to `"never"`, indicates that the specific object should never be deleted. This applies to both explicit deletes as well as garbage collection.
If you want qbec to delete this object, you need to remove the annotation from the in-cluster object. Changing the source
object to remove this annotation will not work.

#### `directives.qbec.io/update-policy` 

* Annotation source: in-cluster object.
* Allowed values: `"default"`, `"never"`
* Default value: `"default"` 

when set to `"never"`, indicates that the specific object should never be updated.
If you want qbec to update this object, you need to remove the annotation from the in-cluster object. Changing the source
object to remove this annotation will not work.


