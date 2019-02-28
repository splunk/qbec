---
title: Create components
weight: 7
--- 

qbec supports components written as YAML, JSON or jsonnet files. YAML and JSON documents are static
and unable to support parameterization per environment. These formats are good enough for Kubernetes
objects like roles, role bindings, service accounts etc. where the content doesn't usually vary
per environment.

In order to create components that need to be different per environment, you are pretty much required
to use jsonnet.

[k8s-yaml-patch](https://github.com/splunk/k8s-yaml-patch),
for example, is a jsonnet library that allows you to load YAML documents, patch runtime values and
return them for qbec use. 

If you use the above or any other library, the correct way to integrate it with qbec for now is to use
the [jsonnet bundler](https://github.com/jsonnet-bundler/jsonnet-bundler) and download the dependencies
locally to a `vendor` directory and add this directory to the `libPaths` array in `qbec.yaml`.

You will then be able to use these dependencies in your jsonnet codebase.



 