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

rm -rf dist/
mkdir -p dist/assets

make clean get

if [[ ! -z "$(git status --porcelain)" ]]
then
    echo "unclean work dir, abort" >&2
    exit 1
fi


for env in darwin-amd64 linux-amd64 windows-amd64
do
    echo ===
    echo build ${env}
    echo ===
    export GOOS=$(echo ${env} | awk -F- '{print $1}')
    export GOARCH=$(echo ${env} | awk -F- '{print $2}')
    export CGO_ENABLED=0
    make os_archive
done

(cd dist/assets && shasum -a 256 *> sha256-checksums.txt)

git tag -s -m "${version} release" ${version}

