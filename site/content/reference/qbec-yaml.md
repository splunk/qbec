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

  # additional library paths when executing jsonnet, no support currently for `http` URLs.
  libPaths: 
  - additional
  - local
  - library
  - paths

  # list of components to exclude by default
  excludes: 
  - default
  - excluded
  - components

  # declaration of late-bound variable definitions that can be passed in on the command line using the --vm:* options. 
  vars: 
    # external variables are accessed as std.extVar('var-name')
    external:
      - name: imageTag # the name of the external variable passed in using --vm:ext-str and related options
        default: 'latest' # the default value to use if this variable is not specified on the command line. Can be an arbitrary object.
        secret: false # when true qbec will not print the plain text value in any debug message

    # for top-level variables, your component's main "object" is a function that accepts a value, typically 
    # initialized with a default in the code.
    topLevel:
      - name: mySecret # the name of the top-level variable (i.e. the name of the function parameter of your component's function)
        components: [ 'service2' ] # the components that require this variable. Must be specified.
        secret: true

  # if the following attribute is set to true and the --app-tag argument is set on the command line, qbec will automatically
  # change the default namespace for the environment in question by suffixing it with <hyphen><tag-value> (e.g. 'myns-tag')
  namespaceTagSuffix: true

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
