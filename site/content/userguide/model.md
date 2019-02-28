---
title: Core concepts
weight: 20
---

qbec uses the following concepts.

## Components

Components are the source code that you write that represent Kubernetes objects.
A component is single source file that produces a collection of logically related Kubernetes objects. 
You implement components by writing jsonnet, YAML or JSON files.

It is also valid for a component to return an empty set of objects if runtime parameters determine that
nothing should be installed for a specific target environment.

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

## Runtime configuration

Different environments need different runtime parameters. These can include, among other things, replica counts,
image versions, specialized config maps, different secrets and so forth. 

qbec sets a jsonnet variable called `qbec.io/env` to the environment name when it loads your components 
for an environment. 
You can use the value of this variable to return different runtime parameters per environment.
Note that for some invocations, the environment may be set to `_` (representing baseline) and your code
should be able to handle this correctly and return default values.

qbec doesn't really mandate a specific file structure to define params. 
The main commands like `apply`, `show`,  and `diff` will work independent of how you set up your runtime parameters.

That said, the `param` subcommands expect runtime configuration to be set up in a specific way.
More information on this can be found in the [folders and files](../usage/basic) section of the user-guide.


