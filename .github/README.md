# Release process

1. Update changelog and version directly on main and commit locally with "update changelog, up version" comment
  * Bump minor version when upgrading marquee dependencies (jsonnet, k8s client, golang, etc.) or when making incompatible changes
1. Run `./prepare-release.sh`
1. Push to main with following command: `git push --atomic upstream main <release>`
1. Manually create a Release on GitHub by clicking "Generate Release Notes"

profit!!
