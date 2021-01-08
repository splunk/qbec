---
title: Jsonnet glob importer
weight: 190
---

In many situations it is useful to be able to import a bag of files as an object. For instance, it is convenient to
load all environment configuration files present with a single invocation to `import` to pick up all files found without
having to name them individually.

For this purpose, qbec supplies a glob importer for importing a bag of  files using a glob pattern. 
It supports two variants, one for importing the files that match the glob as code
and another for importing the files as a string (suitable, for example, for YAML files).

The basic syntax is:

```
import 'glob-import:subdir/*.libsonnet' // imports all libsonnet files from the subdirectory
```

and

```
import 'glob-importstr:*.yaml' // imports all YAML files in the current directory as strings
```

Let's say that in the first example you had files `a.libsonnet`, `b.libsonnet` and `c.libsonnet` in `subdir/`.
This would result in the following code being evaluated by jsonnet.

```
{
   'subdir/a.libsonnet': import 'subdir/a.libsonnet',
   'subdir/b.libsonnet': import 'subdir/b.libsonnet',
   'subdir/c.libsonnet': import 'subdir/c.libsonnet',
}
```

In the second example if you had `a.yaml`, `b.yaml` and `c.yaml` in the current directory, the following code
would be evaluated:

```
{
   'a.yaml': importstr 'a.yaml',
   'b.yaml': importstr 'b.yaml',
   'c.yaml': importstr 'c.yaml',
}
```

In both cases, the keys are paths to files relative to the directory where the `import` was invoked, and the objects
are the objects as returned by the jsonnet `import` and `importstr` directives respectively.

### Notes

* Globs are resolved using Go semantics. It is **not** an error for the pattern to not match any files. In that case,
  an empty object is returned. If you want to treat this case as an error, you can do so in the jsonnet code after the
  import is done.
* Library paths are not considered by this importer. That's because it is ambiguous as to what `*.libsonnet` means
  when there are libsonnet files in the current directory as well as in the library paths.
* The objects that are returned typically have to be post-processed in jsonnet to make them usable (e.g. stripping
  directory paths, removing extensions etc.). See the `globutil.libsonnet` file in the qbec source tree for ideas.
