# Release process

* Update changelog and version directly on main and commit locally with ""update changelog, up version" comment  (thank any contributors in changelog)
  * Bump minor version when upgrading marquee dependencies (jsonnet, k8s client, golang, etc.) or when making incompatible changes
* run `./prepare-release.sh`
* push to main with --tags

profit!!
