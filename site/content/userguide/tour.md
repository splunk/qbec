---
title: A quick tour of qbec
weight: 1
---

## Initialize a new qbec app 

qbec provides an `init` command to set up a new application. Let's run this and see what happens.

```shell
qbec init demo --with-example # --with-example creates a sample "hello" component
```

When the above command runs successfully, it creates a subdirectory called `demo` that has a single
component and environment. The default environment is inferred from the current context in your
kube config.

The following files are created in the `demo` directory:

* `qbec.yaml`: this is a minimal qbec manifest that defines a single environment
* `components/hello.jsonnet`: produces a config map and deployment object
* `params.libsonnet` - the top-level runtime parameters file. This is responsible for returning the
  correct set of runtime parameters based on environment.
* `environments/base.libsonnet` - the baseline runtime parameters with default values
* `environments/default.libsonnet` - the runtime parameters for the default environment

## Run local commands

The following commands run locally and do not communicate with any Kubernetes server

```shell
qbec show default # show the YAML that would be applied to the server

qbec component list default # list all components

qbec param list default  # list all parameters

qbec show -O default # show all object names instead of contents
```

## Filtering

Most qbec commands can be applied to a subset of components or objects. Components can be included
or excluded using the `-c` and the `-C` filters. Specific object kinds can be included and excluded
using the `-k` and `-K` filters.

```shell
qbec show -k deployment default # only shows the deployment but not configmap

qbec show -K deployment default # only shows the configmap and excludes the deployment

qbec show -C hello default # no output since the only component has been excluded
```

## Validate, diff, and apply

```shell
qbec validate default # validates local objects against server metadata

qbec diff default # shows a diff between remote and local objects

qbec apply default  # applies the components to an environment similar to kubectl apply
```

Once objects have been applied to the remote cluster, subsequent `diff` and `apply` commands should
show no changes. Re-run these commands to verify that this is indeed the case.

## Making changes

Let's make some changes to the parameters.

Edit the `environments/default.libsonnet` file and change the replica count and/ or the config string.
Re-run the `diff` and `apply` commands. They should show, respectively, what is different and display 
the patch sent to the server for applying it.

## Clean up

There are two ways to clean up the objects that `qbec` created. In either case, only components
created by qbec will be deleted.

* you can explicitly delete them.

```shell
qbec delete default
``` 

* you can delete the local component and let garbage collection take care of it.

```shell
rm components/hello.jsonnet
qbec apply default
```
