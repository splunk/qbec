---
title: Folders, files, parameters
weight: 1
---

At the very least, you need to have the following files and folders for a qbec app:

* `qbec.yaml` - this needs to be at the root of the source directory and defines your application
   in terms of:
   * supported environments
   * components that should be excluded by default for all environments
   * specific components excluded and included in specific environments.
   * See the [reference document](../../../reference/qbec-yaml/) for more details.
* a folder for components. By default, this is a folder called `components/` under the source root.
  You can change what this folder is called in `qbec.yaml`
* a runtime configuration file that can return runtime parameters based on the value of the
  `qbec.io/env` jsonnet variable. By convention, this is a file called `params.libsonnet` under
  the source root. You can change this name in `qbec.yaml`.
  
## More on runtime configuration

The runtime configuration file is only strictly used by qbec for the `param` sub-commands. The remaining
commands do not make assumptions about how runtime parameters are returned. All they do is set the
`qbec.io/env` external variable to the environment name and expect your code to correctly configure
your components based on its value.

That said, if you follow expected conventions you can use qbec to its fullest.

### The basic parameters object

The parameters object is expected to be of the following form:

```json
{
  "components": {
    "component1": {
      "param1": "value1",
      "param2": "value2"
    },
    "component2": {
      "param1": "value1"    
    }
  }
}
```

That is, it has a top-level `components` key and the configuration values for each component under
a key that is the component name.

The top-level parameters file (typically, `params.libsonnet`) returns an instance of the above object
that is different for every environment. Note that it needs to be able to handle the baseline 
environment (_) as well. 

The `qbec init` command shows you one way to organize your files such that all the above conditions are
met and every environment produces a parameters object that is a specialization of the baseline
configuration.