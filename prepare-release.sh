#!/bin/bash

set -euo pipefail

version_num=$(grep ^VERSION Makefile  | awk -F= '{print $2}' | sed 's/ //')

if [[ -z "${version_num}" ]]
then
    echo "unable to derive version, abort" >&2
    exit 1
fi

version="v${version_num}"
echo "publish version ${version}"

if [[ ! -z "$(git tag -l ${version})" ]]
then
    echo "tag ${version} already exists, abort" >&2
    exit 1
fi

git tag -s -m "${version} release" ${version}

