# Release process

1. Update [CHANGELOG.md](../CHANGELOG.md) and [Makefile](../Makefile) directly on main and commit locally with "update changelog, up version" comment
  * Bump minor version when upgrading marquee dependencies (jsonnet, k8s client, golang, etc.) or when making incompatible changes
1. Run `./prepare-release.sh`
1. Push to main with following command: `git push --atomic upstream main <release>`
