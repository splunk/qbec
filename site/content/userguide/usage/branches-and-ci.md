---
title: Branch builds and CI
weight: 25
---

You have created jsonnet components, a `qbec.yaml` file and declared a dev environment. These are
good enough for local development and/ or commits to a master branch. Now you are looking for a
way to create objects with slightly different names as part of functional tests of your pull request builds.

You want to ensure that each branch build runs in its own "scope" and does not interfere with your master builds.
Specifically, you don't want GC to run amok and delete objects that it was never meant to delete.

qbec provides a command-line argument called `--app-tag` that allows you to create a set of objects scoped
for the application, environment, _and tag_. Leaving out the tag for the master deploy creates its own
"tagless" scope.

The app-tag feature provides the following facilities:

* GC is scoped to a tag such that it doesn't try to delete mainline objects or objects created with other tags.
* It exposes the tag value as a jsonnet external variable (`qbec.io/tag`) so your components can use this to create per-tag objects with unique
  names.
* It exposes the name of the default namespace used for the run (`qbec.io/defaultNs`) and, on request, can automatically
  suffix this value for you.

## Rules of engagement

To be able to use this feature:

* You can only create namespaced objects. The only cluster scoped object you can create is a namespace itself.
  This feature is not meant to be used with cluster scoped objects.

* You must either modify the name of _every_ object you create based on the supplied tag name _or_
  create a new namespace for every tag and create the objects in it. Failure to do so will cause non-deterministic and
  unexpected GC behavior.

* The tag passed to qbec cannot have special characters like `/`. It must conform to what can be used as a label
  value in Kubernetes. qbec will validate this and will not let you proceed otherwise.

* `qbec` is built with safety in mind. All command with side-effects require user confirmation. This behaviour can be overridden by using
the `--yes` flag on commands or by setting the environment variable `QBEC_DISABLE_PROMPTS=true`.

## Usage pattern 1: create a test namespace for every branch

This is the simpler usage pattern that requires you to have permissions to create new namespaces on the cluster.
You would do the following:

* Set the `namespaceTagSuffix` attribute under `spec` to `true` in `qbec.yaml`.
* Leave out the `namespace` attribute from all your Kubernetes objects.
* Create the namespace itself like so:

```jsonnet
{
    apiVersion: 'v1',
    kind: 'Namespace',
    metadata: {
        name: std.extVar('qbec.io/defaultNs'),
    },
}
```

For your master deploy you would do a `qbec apply env` as usual.

For a branch deploy for the `foo`  branch, you would run the command as `qbec apply --app-tag=foo env`

Note that the environment is the same in both cases, assuming you are using the same cluster for both runs.

Assuming that the default namespace declared in `qbec.yaml` is `my-ns` the default namespace seen by your component
will be `my-ns` for the tagless run and `my-ns-foo` for the tagged run.

Once you are done with tests, you can now delete the branch-specific objects by running
`qbec delete --app-tag=foo env`.

## Usage pattern 2: mangle every object's name

This is slightly more involved but is your only option if you do not have permissions to create namespaces on the cluster.
You would do the following:

* Leave out the `namespace` attribute from all your Kubernetes objects.
* Write a helper function that generates a name, like so:

```jsonnet
local makeName = function (baseName) (
    local tag = std.extVar('qbec.io/tag');
    if tag == '' then baseName else baseName + '-' + tag
);
```
* Code your components to use this function for assigning object names

```jsonnet
{
    apiVersion: 'v1',
    kind: 'ConfigMap'
    metadata: {
        name: makeName('my-cm'),
    },
    data: { foo: 'bar' }
}
```

For your master deploy you would do a `qbec apply env` as usual.

For a branch deploy for the `foo`  branch, you would run the command as `qbec apply --app-tag=foo env`

Note that the environment is the same in both cases, assuming you are using the same cluster for both runs.

Once you are done with tests, you can now delete the branch-specific objects by running
`qbec delete --app-tag=foo env`.
