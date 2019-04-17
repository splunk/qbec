---
title: Home
chapter: true
---

# &#9884; qbec

_a tool to configure and create Kubernetes objects on multiple environments_

qbec (pronounced like the [Canadian province](https://en.wikipedia.org/wiki/Quebec)) is a command line tool that 
allows you to create Kubernetes objects on multiple Kubernetes clusters and/ or namespaces configured for 
the target environment in question.

It is based on [jsonnet](https://jsonnet.org) and is similar to other tools in the same space like 
[kubecfg](https://github.com/ksonnet/kubecfg) and [ksonnet](https://ksonnet.io/).
If you already know what the other tools do, read [the comparison document](comparison-with-other-tools/) to understand
how qbec is different from them. Otherwise read the [user guide](userguide/).

## Features

* Deploy across multiple clusters and/ or namespaces
* Deploy cluster-scoped objects (useful for cluster admins)
* Track configurations with version control
* Specify environment-specific component lists
* Apply component and kind filters to commands
* Automatic garbage collection for deleted and renamed objects.
* Integrate with jsonnet external and top-level variables for late-bound configuration
* Create differently named objects for branch builds and garbage collect in that limited scope.
* Usable, safe and secure by default.
  * Remote commands that change cluster state require confirmation.
  * Secrets are automatically hidden and never appear in any output.
  * Performant (limited by the speed of the jsonnet libraries you use)

[Get started](getting-started/).