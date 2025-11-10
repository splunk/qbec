Changelog
---

## v0.30.0 (Nov 4, 2025)

* Update go-jsonnet to 0.21.0
* Bump k8s.io/kube-openapi & k8s.io/gengo

## v0.29.0 (Nov 4, 2025)

* Update Kubernetes client from 1.33.5 to 1.34.1

## v0.28.0 (Nov 4, 2025)

* Update Kubernetes client from 1.32.9 to 1.33.5

## v0.27.0 (Oct 30, 2025)

* Update Kubernetes client from 1.31.13 to 1.32.9

## v0.26.0 (Oct 30, 2025)

* Update Kubernetes client from 1.30.14 to 1.31.13

## v0.25.0 (Oct 30, 2025)

* Update Kubernetes client from 1.29.15 to 1.30.14

## v0.24.0 (Oct 29, 2025)

* Update Kubernetes client from 1.28.15 to 1.29.15

## v0.23.0 (Oct 29, 2025)

* Update Kubernetes client from 1.27.16 to 1.28.15
* Bump protobuf dependency to v1.36.10

## v0.22.0 (Oct 29, 2025)

* Update Kubernetes client from 1.26.15 to 1.27.16

## v0.21.0 (Oct 29, 2025)

* Update Kubernetes client from 1.25.16 to 1.26.15

## v0.20.0 (Oct 29, 2025)

* Update Kubernetes client from 1.24.17 to 1.25.16
* Update various dependencies (spf13 & go-openapi among others)

## v0.19.0 (Oct 29, 2025)

* Update Kubernetes client from 1.23.1 to 1.24.17
* Update various dependencies (spf13 & go-openapi among others)

## v0.18.0 (Oct 29, 2025)

* Update Go from 1.22 to 1.24
* Bump golang.org/x/oauth2 to v0.27.0
* Bump doubelstarbmatcuk/doublestar from v4.0.2 to v4.9.1

## v0.17.0 (Oct 28, 2025)

* Update Go from 1.17 to 1.22
* Update go-jsonnet from v0.18.0 to v0.20.0 & alter some Windows path parsing logic
* Update various dependencies to address security vulnerabilities
* Add license headers

## v0.16.3 (Dec 2, 2024)

* update release documentation
* update various dependencies to address security vulnerabilities

## v0.16.2 (Nov 26, 2024)

* Fix Release configuration

## v0.16.1 (Nov 26, 2024)

* Fix typo in model.md: 
* Fix Go Releaser build step

## v0.16.0 (Sep 6, 2024)

* Added support for helm as an external data source

## v0.15.2 (Mar 5, 2022)

* Add ability to filter by namespaces using `-p` and `-P` and the ability to filter out cluster-scoped-objects
  via `--include-cluster-objects=false`

## v0.15.1 (Feb 4, 2022)

* Fix info in `qbec version`

## v0.15.0 (Feb 4, 2022)

* `apply` now waits for `batch/job` objects (thanks @kvaps)
* Improved error message includes helpful hints on operations requiring exactly one qbec environment as input (thanks @kalhanreddy)
* `eval` now supports inline datasources without the need to specify a qbec environment
* Kubernetes list operations now use a page size of `1000` by default. `--k8s:list-page-size` can be used to adjust the size
  Setting this value too low can impact performance when dealing with large lists
* Update jsonnet library to `v0.18.0`, k8s client libs to `v1.23.1` and golang version to `1.17`
* Misc. housekeeping improvements

Backward incompatibilities:
* `apply` now waits for `batch/job` objects. To disable waiting for all objects `--wait-all=false`
  can be used.
* Adjustments to the internal behaviour around listing Kubernetes objects. Use `--k8s:list-page-size`
  with appropriate values.
* Any corner-case behavior from updating k8s client, jsonnet libraries or golang version

## v0.14.8 (Sep 11, 2021)

* Expose data source creation in the VM implementation for public use

## v0.14.7 (Sep 9, 2021)

* Expose VM implementation as a publicly consumable library
* Change `params.jsonnet` created by `qbec init` to automatically pull environment files using the glob importer (thanks to @kvaps)

## v0.14.6 (Aug 16, 2021)

* Add a `renderYaml` native function to the qbec VM to generate YAML output compatible with `qbec fmt`. If this native
  function is passed an array, it will render multiple YAML docs with separators. Top-level nils are ignored for output.
  If you want an array value to be serialized as a single document, wrap it in a single-element array.

## v0.14.5 (Aug 12, 2021)

* Fix a bug where glob imports could not import relative files outside the qbec root/ current working dir

## v0.14.4 (Jul 7, 2021)

* `fmt` and `alpha lint` commands now accept exclusion patterns using which vendored files, intentionally bad test
   data etc. can be filtered out. Pattern matching of exlcudes is done using the [doublestar library](https://github.com/bmatcuk/doublestar).
*  `qbec.yaml` can now contain examples of data source outputs. These are returned by the mock linter implementation
   when data source imports are found. This allows you to correctly lint files that use data source imports.

## v0.14.3 (Jun 27, 2021)

* `fmt` is now a top-level qbec command. The `alpha fmt` version is deprecated and will be removed in a later release.
* `fmt` has the following enhancements:
  * no longer requires a qbec.yaml to be present to function
  * prints all errors in check mode as opposed to failing fast on the first error encountered
  * prints the filenames of all files that were modified in write mode
  * allows for explicit control on whether it should fail fast or not. The default behavior is to fail fast when
    not in check mode and continue on errors otherwise.
* a new `alpha lint` command can now be used to lint jsonnet and libsonnet files using jsonnet-lint. This command is alpha quality in
  both interface and implementation although it should work just fine on most projects. Known issues are:
  * data sources are not correctly supported
  * the linter can [hang on complex jsonnet files](https://github.com/google/go-jsonnet/issues/541)
  * the linter can [panic under certain conditions](https://github.com/google/go-jsonnet/issues/544)
* Releases now contain ARM64 builds for Darwin and Linux
* Allows the QPS and Burst for the kubernetes client to be configured on the command line via the `k8s:qps` and `k8s:burst`
  flags. Setting these values can be beneficial to large projects producing thousands of objects to improve the
  performance of `diff` and `apply`.

## v0.14.2 (Mar 31, 2021)

* Add ability to inherit the qbec shell environment when processing data sources.

## v0.14.1 (Mar 2, 2021)

There are no changes in this release. The previous release used a jsonnet version
[that had a regression](https://github.com/google/go-jsonnet/issues/515) in the `std.manifestJson[Ex]` functions.
This version [uses a commit](https://github.com/google/go-jsonnet/commit/9b6cbef4caf7ceeff7ab2086d80df7dd63466556)
that fixes this issue.

## v0.14.0 (Feb 27, 2021)

**Note: there is a regression with the `std.manifestJson[Ex]` function in this release. Please use v0.14.1 instead.**

* qbec now allows data to be imported from external data sources. The current implementation allows you to run
  a command whose standard output may be included in your jsonnet component code.
  The [reference page](https://qbec.io/reference/jsonnet-external-data/) has more details.
  At this point, the `expandHelmTemplate` native function should be considered deprecated and will be removed
  in a future release. There is nothing that this function could do that cannot be accomplished using an
  external data source.

* A new command `qbec eval` can now evaluate a single jsonnet file similar to `jsonnet eval`. This can also
  be run in the context of a qbec environment to be able to access environment specific properties.
  See `qbec eval --help` for more details. This deprecates the `jsonnet-qbec` executable that is packaged in the
  release. This executable may no longer be packaged in a future release.

* qbec now allows computed variables to be defined in qbec.yaml. This works especially well with external
  data sources and also allow you to cache complex calculations.
  The [reference page for qbec.yaml](https://qbec.io/reference/qbec-yaml/) has more details.

* Environment files for a qbec environment may now be specified as a glob pattern. qbec will load all matching
  files in sorted alphabetical order.

* Miscellaneous improvements and optimizations for jsonnet evaluation. Most users, unless they are generating
  hundreds of objects or using complex jsonnet libraries will not see any significant performance improvements.

### Incompatibilities

* Preprocessors introduced in `v0.13.4` have now been removed. These had awkward semantics and were honestly
  not ready for prime-time. No one was probably using this. The replacement functionality is more general
  and allows you to define computed vaiables in qbec.yaml.

* Qbec no longer sets the `qbec.io/component` variable when evaluating components. This was also introduced in
  `v0.13.4` and had dodgy semantics. There is no replacement for this functionality. If you started using this
  and are now stuck, please raise an issue where we can discuss this.

## v0.13.4 (Jan 11, 2021)

**Note: some of the features in this release have been removed in `v0.14.0`. Please refer to those release
notes as well.**

a.k.a the "scale" release. This release mainly contains enhancements that allow qbec to be used at scale on
large monorepos and/ or when deploying to tens of clusters.

This release has no backwards-incompatible changes.

* update the glob importer to support `**` patterns. Glob references are now matched using the
  [doublestar library](https://github.com/bmatcuk/doublestar)
* Add a native function `labelsMatchSelector` to expose K8s label matching in jsonnet code
* Allow `componentsDir` to be a glob as opposed to a single directory. This change makes qbec load components
  from potentially multiple directories but does not introduce any namespace semantics. Components must still be
  uniquely named across all such directories. It is only useful for compartmentalizing files in a monorepo
  where different directories have different code owners.
* Add support for pre-processors. A pre-processor is some jsonnet code that is evaluated before any components is.
  The return value of every such preprocessor is set as the external code variable called `computed.qbec.io/<base-name>`
  where `<base-name>` is the base name of the preprocessor file without its extension. This variable is available for
  use in component code. This allows for caching values that are costly to compute.
* qbec now sets the `qbec.io/component` external variable when it evaluates components, pre- and post-processors.
  This may be used in component code to key into a params object, for example. For pre- and post-processors,
  the component name is set to `<pre|post>processor.qbec.io/<base-name>` where `<base-name>` has the same semantics
  as described above.
* Allow multiple pre- and post-processors. The qbec.yaml attributes, `preProcessor` and `postProcessor` although
  singular, can be set to a list of files separated by `:`. Attribute names will be fixed to be in plural form and
  accept an array of strings in a future, backwards-incompatible release.

## v0.13.3 (Dec 23, 2020)

* Add ability to add the component name as a label in addition to the existing annotation. This is opt-in and is activated
  by setting the `addComponentLabel` property to `true` in qbec.yaml (thanks @korroot and @hudymi).

## v0.13.2 (Dec 3, 2020)

* Allow force options to be set via environment variables (thanks @splkforrest)

## v0.13.1 (Dec 2, 2020)

* Fix a bug where the `alpha fmt` command would stop processing arguments after encountering a directory.

## v0.13.0 (Nov 27, 2020)

* Misc. CI build changes
* Update jsonnet library to `v0.17.0` and k8s client libs to `v1.17.13`
* Add json formatter to the `qbec alpha fmt` command
* Fix diff commands to show skipped updates and deletes based on qbec directives specified for existing objects.
  This will no longer show spurious diffs for deletes and updates if those have been turned off.
* Use per-namespace queries by default when multiple namespaces are present, allow using cluster-scoped queries
  using an opt-in flag.
* The `--env-file` option now allows http(s) URLs in addition to local files. In addition, the `envFiles` attribute
  in `qbec.yaml` can also contain http(s) URLs. (thanks @dan1)
* String data in secrets is now obfuscated in addition to binary data

### Incompatibilities

This release is incompatible from previous minor versions in the following ways:

* `qbec apply` will now wait on all objects by default. That is, the `--wait-all` now defaults to `true`.
  To get the previous behavior, you need to add `--wait-all=false` to the `apply` command.
* `qbec diff` now exits 0 by default even when diffs are found. To restore previous behavior, add `--error-exit`
  to the command.
* qbec now defaults to per-namespace queries when multiple namespaces are present. To get the previous behavior
  of using cluster-scoped queries add `clusterScopedLists: true` under `spec` in `qbec.yaml`
* The command line syntax of the `qbec alpha fmt` command has changed in incompatible ways. Instead of options like
 `--jsonnet`, `--yaml` etc. you need to specify options as `--type=jsonnet`, `--type=yaml` and so on.
* YAML formatter now follows `prettier` conventions requiring arrays to be indented under the parent key.
* Any corner-case behavior from updating k8s client and jsonnet libraries.

## v0.12.5 (Oct 2, 2020)

* Add a new `wait-policy` directive to disable waits on specific deployments
  and daemonsets. The annotation `"directives.qbec.io/wait-policy": "never"`
  will cause qbec to not wait on the deployment even if it has changed.

## v0.12.4 (Sep 24, 2020)

* Add `--wait-all` flag to the `apply` command to wait on all objects instead of just the ones that were changed in the
  current run.

## v0.12.3 (Sep 9, 2020)

* Add ability to import a bag of files using a glob pattern (see #153 for details). At this point this should be
  considered experimental. Do not rely on it yet until the next release when we will have docs for it.
* Add windows build in CI, thanks to @harsimranmaan

## v0.12.2 (Aug 30, 2020)

* Fix a bug where under certain circumstances of failed discovery, qbec would delete resources not meant to be deleted.
  Thanks to @sj14 for the bug report and partial fix.
* Add a warning when remote listing for GC switches to cluster scoped mode with a reason as to why this is happening.
  These are typically setup errors by authors who want to deploy to a single namespace.
* Fix pluralization for more kinds when using the kind filter
* Create and run basic integration tests for qbec against a local kind cluster

## v0.12.1 (Jun 20, 2020)

* Add an `error-exit` option to the `diff` command to be able to exit 0 even when diffs are present. This currently has a
  `true` default for backwards compatibility. The next minor release of qbec will flip this default to `false`.
* Improve error message when object processing fails due to bad json created by a component. Thanks to @wurbanski
  for this contribution.

## v0.12.0 (Jun 8, 2020)

There are no backwards-incompatible changes in this release. The minor version upgrade is to account for any
inadvertent incompatibilities introduced by the jsonnet library upgrade.

* Add `alpha fmt` command to format jsonnet/ libsonnet files and, optionally, yaml files (thanks to @harsimranmaan).
  This also has facilities to only check if all files are well-formatted. Type `qbec alpha fmt --help` for details.

## v0.11.2 (May 27, 2020)

* Fix regression in previous release where `LD_FLAGS` were not set correctly causing qbec to report a wrong version.

Please avoid using `v0.11.1` as a result.

## v0.11.1 (May 24, 2020)

* Add ability to refer to an environment by context name rather than server URL. This is useful for environments
  such as `minikube` and `kind` that do not have a stable URL. Since this requires a shared context name, it
  should be used sparingly.
* Add ability to introduce an environment file not declared in qbec.yaml. This is controlled by the `--env-file`
  parameter to qbec commands. Environments in this additional file will override environments of the same name
  if they already exist.
* Add ability to force namespace to the current value in the kubeconfig using the keywork `__current__`. This can
  only be done only if the context is also similarly forced.
* Reduce verbosity of `apply` output. By default, dry-run works as before and actual apply only shows a single
  line per object added/ updated/ deleted. This behavior can be explicitly controlled by the `--show-details` flag
  for the apply command.

## v0.11.0 (Mar 26, 2020)

There are no backwards-incompatible changes in this release. The minor version upgrade is to account for any
inadvertent incompatibilities introduced by the jsonnet library upgrade.

* Fix a bug with namespace forced on the command line not being used in every place that it should have, especially
  when setting the `qbec.io/defaultNs` external variable.
* Upgrade jsonnet to v0.15.0

## v0.10.5 (Feb 1, 2020)

* change alogirthm of merging environment properties with base properties to not use a merge patch. This means that
  `null`s in property values will be retained.

## v0.10.4 (Jan 21, 2020)

* no code changes in this release. This version will be the first to be published as a brew tap thanks to
  @harsimranmaan and @aaqel-s

## v0.10.3 (Dec 19, 2019)

* when returning environment properties introduced in the previous release, first
  apply a JSON merge-patch on to base properties before returning it.

## v0.10.2 (Dec 11, 2019)

* Provide the ability to load environment definitions from external files in addition to defining them inline.
* Add the ability to associate a properties object with every environment and also define baseline properties.
  These are exposed as the `qbec.io/envProperties` external variable.

The test app under `examples` demonstrates both of these features.

* Internal build improvements: add code coverage (thanks @harsimranmaan)

## v0.10.1 (Dec 1, 2019)

Fix a regression in the metadata check introduced in v0.10.0. Nil maps for annotations
and labels were being reported as an error, when they shouldn't have.

## v0.10.0 (Nov 30, 2019)

* better support for custom types that are created lazily, for example, by an operator.
  * qbec now waits for custom types to be available in discovery for up to 2 minutes before applying their instances.
* [support for annotations](https://qbec.io/userguide/usage/directives/) to control qbec processing. This includes the
  ability to lock objects for updates and deletes, and to specify a custom apply order for objects. This includes
  standard rules to ensure that the default and system namespaces are never deleted.
* various internal CI and release process fixes (thanks @harsimranmaan)

Backward incompatibilities:

qbec now checks every object to ensure that their labels and annotations have values that are strictly strings.
Previously metadata having non-string values were dropped in their entirety.

## v0.9.0 (Nov 16, 2019)

* allow for component subdirectories at one level of nesting. A subdirectory directly under the components directory
  is treated as a multi-file component if it has an `index.jsonnet` file (which is the file loaded for the component),
  or an `index.yaml` file (in which case all JSON and YAML files in the directory are loaded).
* add `--force:*` options to be able to override K8s context and namespace from command line (thanks @abhide).
  This allows for new use-cases like in-cluster applies. `qbec options` shows the available options and special values.
  Note that using these options suppresses qbec safety checks. Use with care.
* add `QBEC_YES` environment variable as default for the `--yes` option.
  Provide better messages when a non-interactive build needs confirmation (thanks @harsimranmaan)
* update `client-go` version to `kubernetes-1.15.5`. Add client-go version to the list of versions displayed by the
  `qbec version` command.
* various internal CI improvements (thanks @harsimranmaan)
  * github actions workflow for builds
  * release workflow for tags
  * break build on `gofmt` failures

Backward incompatibilities:

* If you previously had a component subdirectory that has an `index.jsonnet` or `index.yaml` file, qbec will now treat
  it as a component. Rename these files to restore previous behavior.
* client-go upgrade has the potential to be backwards-incompatible for edge cases.

## v0.8.0 (Oct 31, 2019)

* update jsonnet version to v0.14.0
* add initial version of bash completion command (thanks @e-zhang)
* add qbec logo (thanks @kvaps)
* minor bug fixes

There are no backwards-incompatible changes in this release. The minor version upgrade
is to account for any unintentional backward incompatibilities caused by the jsonnet
library upgrade.

## v0.7.5 (Jul 28, 2019)

* Fixes #51 by aligning the patch logic between qbec and kubectl more closely.

## v0.7.4 (Jul 28, 2019)

* Fix a bug (#33) where the original object for patches was using the live server object when it was not supposed to
* Add kubectl's last applied annotation as a source for the original object when qbec annotation not found

## v0.7.3 (Jul 22, 2019)

* allow user to define a jsonnet post-processor in `qbec.yaml` that is provided with every object returned
  from evaliating jsonnet components and has the ability to decorate it, typically with additional annotations
  and labels. This allows common metadata to be set in one place.
* add a `--clean` option to the `show` command that strips qbec metadata from the output. This reduces the noise
  when inspecting objects for debugging. Introduce a standard external variable called `qbec.io/cleanMode` that is
  `off` by default for all commands and only turned `on` for the `show --clean` command.
* the above means that the post processor can use the value of the external variable to add annotations or not.
  This provides for a "real clean" experience.

## v0.7.2 (Jul 19, 2019)

* add a `--wait` option to the `apply` command to automatically wait for deployments, daemonsets and
  statefulsets to be rolled out before the command exits.

## v0.7.1 (Jul 7, 2019)

* update `jsonnet` version to `v0.13`
* add `env list` and `env vars` command to enable arbitrary scripts to iterate over and get cluster information
  from qbec environments.
* add [support for transient objects](https://github.com/splunk/qbec/commit/78e778b19e5761c2a530917bd5bba9b7abb6fabf)
  that do not have a name but have `generateName` set. Always create such objects and garbage collect the versions of
  the object created in previous runs.

## v0.7.0 (Jun 20, 2019)

safety feature: add duplicate checks to disallow more than one object with the same API group, kind, namespace and name.

These checks occur before any component or kind filtering and cannot be suppressed. To this end, this release _may_
be backwards-incompatible if you already have duplicate objects in your component list.

## v0.6.6 (Jun 19, 2019)

* add global options to pass in a list of string var definitions from a file

## v0.6.5 (Jun 14, 2019)

* add `--silent` option to validate to suppress success/ unknown type messages

## v0.6.4 (Jun 13, 2019)

* enhance diffs to show content that will be added and removed rather than single lines that said 'object not on sever',
  'object not present locally' etc.

## v0.6.3 (Apr 20, 2019)

* correctly configure the Kubernetes client such that auth plugins are supported. There are no features in this release.

## v0.6.2 (Apr 16, 2019)

* add support for [declaring and defaulting jsonnet variables](https://github.com/splunk/qbec/pull/10) including TLAs
* add support for [GC tag scope, expose more standard variables](https://github.com/splunk/qbec/pull/13)
* usability fix: ensure confirmation prompts for apply do not get [obscured by background goroutine warnings](https://github.com/splunk/qbec/pull/16)
* additions to qbec spec, no backwards incompatible changes

## v0.6.1 (Apr 1, 2019)

* change how components are [evaluated internally](https://github.com/splunk/qbec/pull/6)
* add EXPERIMENTAL support to [expand helm templates](https://github.com/splunk/qbec/pull/8)

## v0.6

* initial release
