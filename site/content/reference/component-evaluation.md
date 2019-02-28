---
title: Component evaluation
weight: 20
---

How qbec evaluates component code using jsonnet and what it expects the output to look like.

## Jsonnet evaluation

This works as follows:

* Collect the list of files to be evaluated for the environment. This takes into account all components in the directory,
  inclusion and exclusion lists for the current environment and component filters specified on the command line.
* Assuming this leads to 3 files, say, `c1.jsonnet`, `c2.json`, and `c3.yaml` create a jsonnet snippet as follows:

```
local parseYaml = std.native('parseYaml');
local parseJson = std.native('parseJson');
{
  'c1': import '/path/to/c1.jsonnet',
  'c2': parseJson(importstr '/path/to/c2.json'),
  'c3': parseYaml(importstr '/path/to/c3.yaml'),
}
```
* Evaluate this snippet after setting the `qbec.io/env` extension variable to the environment name in question.

## Converting component output to Kubernetes objects

The evaluation above creates a map of component names to outputs returned by the jsonnet, json and yaml files.

The output is allowed to be:

* A single Kubernetes object (identified as such by virtue of having a `kind` and `apiVersion`
  fields, where `kind` is not `List`).
* A Kubernetes list object where `kind` is `List` and it has an `items` array attribute
  containing an array of outputs.
* A map of string keys to outputs
* An array of outputs

In the latter 3 cases, the output is processed recursively to get to the leaf k8s objects.

