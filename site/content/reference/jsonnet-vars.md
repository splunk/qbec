---
title: Standard jsonnet variables
weight: 11
---

qbec exposes the following standard jsonnet variables whenever it evaluates components.

* `qbec.io/env` - the name of the environment for which processing occurs.
* `qbec.io/tag` - the tag specified for the command using the `--app-tag` option.
* `qbec.io/defaultNs` - the default namespace in use. This is typically picked from the environment definition,
   possibly changed for app tags, or the value forced from the command line using the `--force:k8s-namespace` option.
* `qbec.io/cleanMode` - has the `off` or `on`. The `on` value is only set for the `show --clean` command.

In addition to the above, qbec will also set the default values for declared external variables and
override them from command line arguments.

See the [component evaluation page](../component-evaluation) for the gory details.
