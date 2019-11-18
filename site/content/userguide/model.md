---
title: Core concepts
weight: 20
---

qbec uses the following concepts.

## Components

Components are the source code that you write that represent Kubernetes objects. 
You will typically organize related Kubernetes objects as a single component 
(e.g. a microservice that needs a service account, deployment, service, and config map
kubernetes objects).

Components are loaded from jsonnet, json, or yaml files. Only jsonnet files provide the ability
to customize object definitions for different environments. In addition, qbec support loading
objects from helm charts, and using jsonnet libraries to produce them.

More details can be found in the [authoring components](userguide/authoring/) page.

## Environments

Components are applied to environments. An environment is a cluster as represented by a server URL and an
optional default namespace to use. This allows for the following variations:

* Every environment is a separate cluster where you deploy components. In such a setup you may freely
  create cluster scoped objects as well as K8s objects in multiple namespaces.
  
* Multiple environments can be on the same cluster using different namespaces. In this case, you would
  only create namespace scoped objects and leave out the namespace metadata in the object definition.
  This way, updates to different environments will update different namespaces on the same cluster.
  
## Application manifest

qbec expects a file called `qbec.yaml` at the root of your source tree. This file provides a name
for your application and defines the environments to which Kubernetes objects will be deployed.

A minimal manifest with one environment looks like this:

```yaml
apiVersion: qbec.io/v1alpha1
kind: App
metadata:
  name: my-app
spec:
  environments:
    default:
      server: https://minikube:8443 
      defaultNamespace: my-ns
```

The [reference section](../../reference/qbec-yaml) has a detailed description of this file.

## Environment-specific component lists

The set of all components, by definition, is all the supported files present in the components directory
of your app. You can modify this list per environment in the following ways by adding more information
to `qbec.yaml`.

* Supply a list of components that should not be included by default in any environment.
* Supply explicit inclusion and exclusion lists per environment.

This has the following implications.

* A component that is excluded by default has to be explicitly included in an environment for the
  component to be applied there.
* A component that would be included by default can be explicitly excluded for an environment.

## Baseline environment

In addition to the environments you define in the application manifest (see below), qbec automatically
adds a baseline environment called `_` that can be used for commands that operate locally and do not
affect a cluster. These commands include `show`, as well as the `component` and `param` subcommands.

## Basic runtime configuration

Different environments need different runtime parameters. These can include, among other things, replica counts,
image versions, specialized config maps, different secrets and so forth. 

qbec sets a jsonnet variable called `qbec.io/env` to the environment name when it loads your components 
for an environment.  You can use the value of this variable to return different runtime parameters per environment.
Note that for some invocations, the environment may be set to `_` (representing baseline) and your code
should be able to handle this correctly and return default values.

qbec doesn't really mandate a specific file structure to define params. 
The main commands like `apply`, `show`,  and `diff` will work independent of how you set up your runtime parameters.

That said, the `param` subcommands expect runtime configuration to be set up in a specific way.
More information on this can be found in the [folders and files](../usage/basic) section of the user guide.

## Late-bound configuration

Some configuration parameters like image tags of an image that was just built to be used for the deployment,
or secrets cannot be checked into source code. For these situations, qbec provides a way to declare jsonnet variables
that need to be passed in with default values to be used when they are not. 

This allows you to not have to _always_ pass in these extra arguments when developing components locally and still
be able to parameterize them in, say, your CI build. More information on this feature can be found in the 
[runtime parameters](../usage/runtime-params) section of the user guide.

## Creating objects with different names for branch builds

qbec provides support that allows you to use the same component and environment definitions for 
creating objects with different names (or namespaces) for branch builds and garbage collect them without 
affecting your mainline objects. More information on this feature can be found in 
the [branch builds and CI](../usage/branches-and-ci) section of the user guide.
