Changelog
---

# v0.6.6 (Jun 19, 2019)

* add global options to pass in a list of string var definitions from a file

# v0.6.5 (Jun 14, 2019)

* add `--silent` option to validate to suppress success/ unknown type messages

# v0.6.4 (Jun 13, 2019)

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