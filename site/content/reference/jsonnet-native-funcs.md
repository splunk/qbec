---
title: Jsonnet native functions
weight: 180
---

A list of all native functions that qbec natively supports.

## expandHelmTemplate

**this function is now deprecated. Integrate with helm using external data sources instead**

The `expandHelmTemplate` function expands a helm chart and returns the resulting objects.
This is EXPERIMENTAL in nature - the API is subject to change in a subsequent release.
It runs the `helm template` command, assuming that the `helm` binary is already installed and
available in the PATH.

### Usage
```
    local expandHelmTemplate = std.native('expandHelmTemplate');
    expandHelmTemplate("path/to/chart", 
        { 
            chartProperty: 'chart-value'
        },
        {
            namespace: 'my-ns',
            name: 'my-name',
            thisFile: std.thisFile, // important
        },
    )
```

* The first argument is a path to a helm chart (which is usually a folder or a `.tar.gz` file).
* The second argument is an object containing the values for the chart. This is turned into a `--values` argument to the template command.
* The last argument is an object representing a set of options, including options to the helm command.

### The options object

This object supports the following keys:

* `thisFile` (string) - the current file from which the function is called. Should be set to [std.thisFile](https://jsonnet.org/ref/stdlib.html#thisFile). Relative references
   are resolved with respect to the supplied file path.
* `namespace` (string) - the `--namespace` argument to the template command
* `name` (string) - the `--name` argument to the template command
* `nameTemplate` (string) - the `--name-template` argument to the template command
* `generateName` (string) - the `--generate-name` argument to the template command
* `execute` (array of strings) - the `--execute` argument to the template command
* `kubeVersion` (string) - the `--kube-version` argument to the template command
* `verbose` (bool) - print `helm template` command invocation to standard error before executing it


## parseJson

The `parseJson` function parses a JSON-encoded string and returns the corresponding object.

### Usage
```
    local parseJson = std.native('parseJson');
    local jsonString = '{ "hello" : "world" }';
    
    parseJson(jsonString) // returns the _object_ { hello: 'world' }
```

## parseYaml

The `parseYaml` function parses an input string into an array of YAML documents. It _always_ returns an array
even if there is only one YAML document in the string.

### Usage
```
    local parseYaml = std.native('parseYaml');
    local yamlString = importstr './path/to/yaml/file';
    
    parseYaml(yamlString) // returns all YAML docs as an array
```

## renderYaml

The `renderYaml` function takes a single input and returns the corresponding YAML as a string. This YAML is compatible
with the output of `qbec fmt`. If the input is an array, it will create multiple YAML documents, one for each element.
To render an array value as a single document wrap the value in a one-element array. Note that top-level nils do not
produce any output.

### Usage
```
   local renderYaml = std.native('renderYaml');
   local oneDoc = renderYaml({ foo: 'bar', bar: 'baz' });
   local twoDocs = renderYaml([{ foo: 'bar' }, null, { foo: 'baz' }]);
   local arrayAsDoc = renderYaml([[{ foo: 'bar' }, { foo: 'baz' }]]);
   {
     one: oneDoc,
     two: twoDocs,
     aDoc: arrayAsDoc,
   }
```
