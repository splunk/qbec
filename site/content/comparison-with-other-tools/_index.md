---
title: Comparison with other tools
weight: 20
---

We compare qbec with ksonnet (v0.13) and kubecfg (v0.9.0) along multiple dimensions.

### Environments

qbec and ksonnet use an application manifest to declare environments. Kubernetes contexts are derived based off the
server URL defined for environments. Kubecfg requires the context to be specified on the command line.

This avoids command-line typos of a dangerous nature.

### Runtime configuration

* kubecfg requires the user to parameterize runtime configuration based of jsonnet variables passed to the command line.
  In short, it leaves parameterization up to the user.

* ksonnet has a lot of structure for defining per-environment parameters and creates a code variable (i.e. `__ksonnet/params`)
  that provide this information to components. ksonnet also provides commands to set environment specific parameters,
  list them etc.

* qbec takes a middle road. It automatically creates a `qbec.io/env` variable that has the environment name and expects
  user code to return parameters based on this. It _does_ support listing and displaying parameters per environment
  provided the user has followed expected conventions. The rest of the commands (`show`, `apply` etc.) do not care 
  about this.

The reason for this choice is that it is _far_ easier to setup a string variable in your IDE (e.g. VSCode), have 
all components resolve statically, and have the IDE produce jsonnet errors. This allows the user to see error messages
as they are developing components. Since the environment variable name is tied to the environment via the application
manifest, this strategy also provides the minimal structure neeeded such that people cannot easily make mistakes and 
apply a dev configuration to a prod cluster.

Finally `qbec` stays out of the business of _how_ runtime configuration is set up in files by not having setters for parameters.
In practice, we have found that we need to compose runtime parameters from cloud information, secrets from environment variables,
versions of images supplied by the CD pipeline etc. and it is not always explicitly set up in a standard format.

### Component lists by environment

* kubecfg does not have a a formal notion of components by environment
* ksonnet produces a list of components and creates a code variable (i.e. `__ksonnet/components`) using this list.
  This can be augmented at runtime per environment using additional object definitions. 
* qbec loads all components by default but the user has the option to specify (in `qbec.yaml`)
  * the components that should be excluded by default for all environments
  * environment specific inclusions and exclusions of specific components.
  
This allows for qbec to create component lists in a declarative manner. Unlike ksonnet, it does not require (or support) 
a main module per environment that returns this list. The advantage over the ksonnet approach is that
component filters work uniformly in all cases when component lists are formally specified.

### Magic jsonnet variables

* kubecfg does not create any jsonnet variables by default
* ksonnet creates 2 code variables `__ksonnet/components` and `__ksonnet/params` 
* qbec creates a string variable `qbec.io/env` that has the environment name

See the runtime configuration section for pros and cons

### Object updates

Both ksonnet and qbec use a 3-way merge patch for applying object updates while keeping track of the last applied
configuration similar to `kubectl`. Kubecfg does not do this which leads to spurious diffs and incorrect updates
for a few cases (e.g. service host ports).

### Garbage collection

qbec has garbage collection turned on by default and does not require additional machinery like `--gc-tag` etc. to enable
it. It can be disabled on demand. Diffs show objects that would be deleted.

See how garbage collection works in qbec in the reference section.

### Secure by default

qbec hides the real values for K8s secrets by and replaces them with a hash of the value prefixed by 
a random string that changes for every invocation. This strategy allows diffs to work but 
ensures that no command dumps sensitive output.
All commands can be safely used in a CI environment without having to worry about exposing
secrets in the build log.

### Safe by default

Mutating commands in qbec require confirmation by default.

Diffs are clean and dry-runs show the patches that would be applied on the server.
This allows users to eyeball the logs and ensure that bad things won't happen.

### Support for network import paths

Unlike the other tools, qbec *does not* implement `http` import paths. This is not a technical limitation but a choice
to see how far we can go with this approach, which requires all remote components to be pulled in locally before
executing `qbec`. 

We are not averse to adding this feature but would like to do it consciously after evaluating other options.

### Support for community components, prototypes, helm charts etc.

Only ksonnet has these features. In this author's opinion these features have made the ksonnet surface area large and
difficult to reason about so we have left them out. 

It is probably useful to provide support for helm charts in qbec.

**Update**: As of v0.6.1, qbec has experimental support to expand helm chart templates. This is implemented as a
native function that can be called from jsonnet code.
