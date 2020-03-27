Changelog
---

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
