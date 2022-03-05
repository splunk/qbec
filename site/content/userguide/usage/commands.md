---
title: Running qbec commands
weight: 30
---

The qbec CLI provides usage help for all commands. The commands that qbec supports are:

```shell
$ qbec --help

qbec provides a set of commands to manage kubernetes objects on multiple clusters.

Usage:
  qbec [command]

Available Commands:
  alpha       experimental qbec commands
  apply       apply one or more components to a Kubernetes cluster
  completion  Output shell completion for bash
  component   component lists and diffs
  delete      delete one or more components from a Kubernetes cluster
  diff        diff one or more components against objects in a Kubernetes cluster
  env         environment lists and details
  eval        evaluate the supplied file optionally under a qbec environment
  fmt         format jsonnet, yaml or json files
  help        Help about any command
  init        initialize a qbec app
  param       parameter lists and diffs
  show        show output in YAML or JSON format for one or more components
  validate    validate one or more components against the spec of a kubernetes cluster
  version     print program version

...
```

## Command lifecycle

You would typically use commands in the following order for a new application:

* `qbec init` - to initialize the app
* `qbec show` -  to display/ debug the output of your components
* `qbec validate` - to ensure that all Kubernetes objects are valid
* `qbec apply` - to apply the objects to the remote server

Once the above is working, you will typically add new environments. The following commands are then
useful.

* `qbec component list|diff` - to list components and diff component lists across environments
* `qbec param list|diff` - to list/ diff parameters for an environment

If you mistakenly apply components prematurely, you can delete them using `qbec delete`

## Filters

Most commands accept filtering options. Filters allow you to restrict the scope at which commands execute.

### Component filters

Component filters allow you to specify components on which the command operates. You can do this by providing an
inclusion list or an exclusion list.

To include specific components, use `-c component1 -c component2 ...`

To exclude specific components, use `-C component1 -C component2 ...`

The behavior of component filters is as follows:

* all components specified on the command line must be valid. That is, a component with that name must actually exist
  in the components directory.
* specifying a component that is excluded for the environment is a noop. That is, if component `foo` was part of the
  exclusion list for an environment, specifying `-c foo` has no effect. Instead, a warning is printed to the terminal.

The above rules do not apply to the `param` subcommands, since we expect you may create component sections in parameters
for components that don't exist (e.g. a `shared` section for settings common to multiple components).

### Kind filters

Kind filters allow you to specify the "kind" of object that should be in scope for the command. The kind is the "kind"
attribute for a Kubernetes object.

To include specific kinds, use `-k kind1 -k kind2 ...`

To exclude specific kinds, use `-K kind1 -K kind2 ...`

Unlike components that are known in advance, qbec has no way of knowing what kinds are acceptable values. In addition,
qbec doesn't try to determine the list by interrogating server metadata since this filter is allowed even for commands
that normally do not need cluster access (e.g. `qbec show`).

In order to implement kind filters, qbec makes case-insensitive comparisons of the object kind with the supplied value(s)
accounting for simple pluralization.

For instance, `-k secret` and `-k secrets` will both extract just the objects with the kind `Secret`. While this provides
the _illusion_ of working like `kubectl` does, kind filters do not account for abbreviations. You cannot say `deploy`
to mean `deployment`.

### Namespace and cluster scope filters

For projects that create objects in multiple namespaces, you can use namespace filters to filter objects for specific
namespaces. As with the other filters, you can provide an inclusion or exclusion list.

To include specific namespaces, use `-p namespace1 -p namespace1 ...`

To exclude specific namespaces, use `-P namespace1 -P namespace2 ...`

Normally cluster scoped objects are not filtered. However, when a namespace filter is applied, cluster scoped objects
are automatically filtered out. You can control this behavior explicitly for any command using
`--include-cluster-objects=true|false`. This flag can also be used independent of namespace filters to hide all
cluster scoped objects.

*Note:* specifying namespace / cluster-scope filters requires qbec to access the cluster in order to retrieve metadata
on object kinds. This means that a `qbec show` command that normally does not need cluster access will now require it.

## Command help

Help and examples for every sub-command can be displayed with a `--help` flag.
For example, here's help on the `show` sub-command.

```shell
$ qbec show --help
show output in YAML or JSON format for one or more components

Usage:
  qbec show <environment> [flags]

Examples:

# show all components for the 'dev' environment in YAML
qbec show dev

# expand just 2 components and output JSON
qbec show dev -c postgres -c redis -o json

# expand all but 2 components
qbec show dev -C postgres -C redis

# show only deployments and config maps
qbec show dev -k deployment -k configmap

# show all objects except secrets
qbec show dev -K secret

# list all objects for the dev environment
qbec show dev -O

Flags:
  -c, --component stringArray           include just this component
  -C, --exclude-component stringArray   exclude this component
  -K, --exclude-kind stringArray        exclude objects with this kind
  -o, --format string                   Output format. Supported values are: json, yaml (default "yaml")
  -h, --help                            help for show
  -k, --kind stringArray                include objects with this kind
  -O, --objects                         Only print names of objects instead of their contents
  -S, --show-secrets                    do not obfuscate secret values in the output
      --sort-apply                      sort output in apply order (requires cluster access)

Use "qbec options" for a list of global options available to all commands.
```

## Running other scripts for qbec environments

Sometimes you need to run other commands and scripts in addition to `qbec apply` that operate on
the same cluster/ namespace as a qbec environment. qbec provides the `env list` and `env vars`
command to enumerate its environments and set `kubectl` args to point to the same context
as qbec would use for an environment.

The output of the `env vars` command can be `eval`-ed in the shell and used to run other commands.

Example:
```
$ qbec env vars dev
KUBECONFIG='/path/to/kube/config';
KUBE_CLUSTER='dev-cluster-name';
KUBE_CONTEXT='dev';
KUBE_NAMESPACE='dev-ns';
KUBECTL_ARGS='--context=dev--cluster=dev.example.com --namespace=default';
export KUBECONFIG KUBE_CLUSTER KUBE_CONTEXT KUBE_NAMESPACE KUBECTL_ARGS
```

The basic output can be eval-ed like so and used in `kubectl` or other commands:

```
$ eval $(qbec env vars dev) # this sets the environment variables printed above
$ kubectl ${KUBECTL_ARGS} apply -f somefile.yaml
```

If you want the cluster, context, namespace etc. as structured output you can use the `-o json`
option.

```
$ qbec env vars dev -o json
{
  "configFile": "/path/to/kube/config",
  "context": "dev",
  "cluster": "dev1-cluster-name",
  "namespace": "dev-ns"
}
```

## Experimental commands

`qbec` includes some experimental commands that are not ready for primetime. These commands are not guaranteed to be backwards compatible between releases. They might also be removed in a future release. Use with caution.

To see a list of experimental commands, run:

```shell
qbec alpha --help
```

