#!/bin/bash

set -euo pipefail

if [[ ! -z "$(git status --porcelain)" ]]
then
    echo "unclean work dir, abort" >&2
    exit 1
fi

COMMIT=$(git rev-parse --short HEAD)

make site
rsync -rvtl ./site/public/ ../qbec-docs/
cd ../qbec-docs
git add .
git commit -m "update docs from $COMMIT"
git push origin gh-pages

