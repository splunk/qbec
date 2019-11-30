---
title: Home
chapter: true
---

# ![qbec](/images/qbec-logo-black.svg)
 
_a tool to configure and create Kubernetes objects on multiple environments_

qbec (pronounced like the [Canadian province](https://en.wikipedia.org/wiki/Quebec)) is a command line tool that 
allows you to create Kubernetes objects on multiple Kubernetes clusters and/ or namespaces configured for 
the target environment in question.

It is based on [jsonnet](https://jsonnet.org) and is similar to other tools in the same space like 
[kubecfg](https://github.com/ksonnet/kubecfg) and [ksonnet](https://ksonnet.io/).
If you already know what the other tools do, read [the comparison document](comparison-with-other-tools/) to understand
how qbec is different from them. Otherwise read the [user guide](userguide/).

## Features

* Deploy Kubernetes objects across multiple clusters and/ or namespaces.
* Create transient objects like one-off `jobs` and `pods` with generated names.
* Deploy cluster-scoped objects (useful for cluster admins)
* Specify common metadata (e.g. annotations) for all objects in one place.
* Track configurations with version control
* Specify environment-specific component lists
* Apply component and kind filters to commands
* Automatic garbage collection for deleted and renamed objects.
* Integrate with jsonnet external and top-level variables for late-bound configuration
* Create differently named objects for branch builds and garbage collect in that limited scope.
* Customize update and delete behavior using directives.
* Usable, safe and secure by default.
  * Duplicate objects having the same kind, namespace, and name are detected and disallowed.
  * Remote commands that change cluster state require confirmation.
  * Secrets are automatically hidden and never appear in any output.
  * Performant (limited by the speed of the jsonnet libraries you use)
* Ability to use qbec environment definitions in other unrelated commands and scripts so that the
  safety properties of qbec are carried over to those commands.

[Get started](getting-started/).
