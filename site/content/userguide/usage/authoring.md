---
title: Create components
weight: 7
--- 

qbec supports components written as YAML, JSON or jsonnet files. YAML and JSON documents are static
and unable to support parameterization per environment. These formats are good enough for Kubernetes
objects like roles, role bindings, service accounts etc. where the content doesn't usually vary
per environment. In order to create components that need to be different per environment, you are 
pretty much required to use jsonnet.

## Component structure

Components are loaded from the components directory defined for your app. 
This defaults to `components/` under the source tree but you can change this path in `qbec.yaml`.

A component is:

* a single jsonnet, json or yaml source file directly under the components directory. In this case the component name
  is the name of the file without the extension and qbec will process this file.
* a subdirectory directly under the components directory that has an `index.jsonnet` file. In this case the component
  name is the subdirectory name and the `index.jsonnet` is processed by qbec.
* a sudirectory containing an `index.yaml` file. In this case the component name is the subdirectory name and all
  json and yaml files under the directory are loaded by qbec.

It is valid for a component to return an empty set of objects if runtime parameters determine that
nothing should be installed for a specific target environment.

## Using helm charts and external data sources

qbec provides integration to run external commands and consume their output in jsonnet code. 
See the [Jsonnet data importer](../../../reference/jsonnet-external-data) for more information on how this works.

Note that the [expandHelmTemplate](../../../reference/jsonnet-native-funcs/#expandhelmtemplate) native function
is now deprecated in favor of the data importer mechanism.

## Using other jsonnet libraries

[k8s-yaml-patch](https://github.com/splunk/k8s-yaml-patch),
for example, is a jsonnet library that allows you to load YAML documents, patch runtime values and
return them for qbec use. 

If you use the above or any other library, the correct way to integrate it with qbec is to use
the [jsonnet bundler](https://github.com/jsonnet-bundler/jsonnet-bundler). download the dependencies
locally to a `vendor` directory, and add this directory to the `libPaths` array in `qbec.yaml`.
You will then be able to use these dependencies in your jsonnet code.

## Creating transient objects

Most objects that you create using qbec are objects with fixed names (e.g. deployments, services etc.). Occasionally
you may need to create transient objects like jobs and one-off pods. 

qbec, like kubectl, allows you to set the `generateName` instead of the `name` metadata attribute for an object.
When the `generateName` attribute is set, qbec will change its behavior such that:

* two objects with the same `generateName` attribute are not considered duplicates.
* qbec tracks the actual name with which the object was created
* it garbage collects previously created transient objects that were not created in the current run of `qbec apply`
