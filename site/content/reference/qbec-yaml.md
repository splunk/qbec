---
title: Application YAML
weight: 10
---

The app configuration is a file called `qbec.yaml` and needs to be at the root of the directory tree.

```yaml
apiVersion: qbec.io/v1alpha1 # only supported version currently
kind: App # must always be "App"
metadata:
  name: my-app # app name. Allows multiple qbec apps to deploy different objects to the same namespace without GC collisions
spec:
  componentsDir: components    # directory where component files can be found. Not recursive. default: components
  paramsFile: params.libsonnet # file to load for `param list` and `param diff` commands. Not otherwise used.

  libPaths: # additional library paths when executing jsonnet, no support currently for `http` URLs.
  - additional
  - local
  - library
  - paths

  excludes: # list of components to exclude by default
  - components
  - to
  - exclude
  - by
  - default

  environments: # map of environment names to environment objects

    minikube:
      server: https://minikube:8443 # server end point
      defaultNamespace: my-ns # the namespace to use when namespaced object does not define it.
      includes: # components to include, subset of global exclusion list
      - components
      - to
      - include
      excludes: # additional components to exclude
      - more
      - exclusions

    dev:
      server: https://dev-server
```

### Notes

* The list of components is loaded from the `componentsDir` directory. Components may be `.jsonnet`, `.json` or `.yaml`
  files. The name of the component is the name of the file without the extension. You may not create two files with the
  same name and different extensions.
* Once the list is loaded, all exclusion and inclusion lists are checked to ensure that they refer to valid components.
* The global exclusion list allows you to introduce a new component gradually by only including it in a dev environment.
