---
title: Garbage collection
weight: 50
---

While garbage collection is supported as a first-class operation qbec and enabled by default, it is a
complex, nuanced subject fraught with special cases. We hope that the explanation below can help users 
figure out the causes of issues they might see in this area and create better bug reports.

## What garbage collection means

Garbage collection is the act of deleting objects that were once applied for a qbec app but no longer
exist in source code. This can be caused by removing local component files, removing object definitions
from files or renaming objects in them.
If garbage collection were not enabled, this would cause the deleted objects or the ones with the
old name to be left behind on the server. 

## The problem space

Here are a few reasons why GC is a non-trivial problem:

* We need to be able to tell that seemingly different objects are actually the same. A simple example is
  a deployment that has a group version of `extensions/v1beta1` but the server has a preferred version
  of `extensions/v1beta2`. These are the same object, just represented differently.
* Some groups have been aliased. So now we need to be able to tell that a deployment having a 
  group version of `extensions/v1beta1` is the same as a deployment having a group version of `apps/v1beta1`
* We need to efficiently gather a list of server-side objects that have been created in past for the qbec
  application.
* When bootstrapping a new cluster with cluster scoped objects, some custom resource definitions may not
  even exist on the server. Trying to get lists of server-side resources based on local resource types
  may not even work.
* Some objects (like specialized controllers and services) create other objects (pods and endpoints) and 
  propagate their labels to these children. A naive approach of looking at all server objects having the
  labels for the app and environment and deleting the ones not locally defined may end up 
  deleting objects that we didn't create in the first place.
* The process of producing lists of objects of a specific kind requires the user to have permissions to 
  list that kind of object. For instance, a user with namespace-only scope may be unable to list some
  or all cluster objects.
* A required object that exists on the server but has not been created by qbec in the first place
  should not be a candidate for deletion. Another example is a `qbec apply` that is run with component
  filters. A target object that exists but with a different component name that does not match the filter
  should still be left alone.

## How qbec implements garbage collection

### Step 1: Load server metadata

Load all group/ version/ kind combinations that the server supports and map each of them to the
canonical version preferred for the server. This takes aliasing (e.g. extensions -> apps) into account.

### Step 2: Figure out the scope at which objects need to be listed.

In this step, qbec looks at all source components and

  * computes a list of affected namespaces which always includes the default namespace for the environment
  * computes whether any cluster-scoped objects exist in the list
 
This computation is always done by looking at all objects in source irrespective of the component and
kind filters passed to the command.

### Step 3: List remote objects
  * If source objects affect a single namespace, query that namespace for all server-side objects having
    labels that match the qbec application and environment.
  * If multiple namespaces, list objects across all namespaces using label filters. This is done for
    efficiency and assumes that the user has list permissions across namespaces. There is currently
    no way to control this behavior (i.e. listing objects one namespace at a time)
  * If cluster-scoped objects are involved, query all cluster scoped objects as well
 
Note that:

  * listings are performed only for the canonical group-version-kind combinations discovered in step 1.
  * listings are done without taking component filters into account. If kind filters are specified,
     only the kinds matching the filter are listed.

### Step 4: Handle special cases

* Remove objects created by a controller from the list
* Always remove all `Endpoints` objects.  

### Step 5: Delete objects

* Create the local list of all objects with the canonical group version kinds.
* Remove all objects from the remote list that match any local object
* Apply the component filters on the filtered remote list
* Delete objects one at a time in reverse apply order

## Known gotchas

* Since the list scope is determined by looking at currently used namespaces, it can miss a namespace
  that used to exist in source code but no longer does. It can similarly miss cluster scoped objects
  that were once created but no longer are.
* When multiple namespaces are involved, qbec issues list queries that span all namespaces. This 
  operation can fail if the user has permissions to list each of the individual namespaces but is 
  not allowed to list objects for all namespaces.
* When applying cluster scoped objects for the very first time, some object types may not exist on the server.
  This can be worked around by disabling GC for the initial run or by using kind filters to exclude
  the custom resources.
