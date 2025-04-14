#!/bin/bash
# Copyright 2025 Splunk Inc.
#
# Licensed under the Apache License, Version 2.0 (the "License");
# you may not use this file except in compliance with the License.
# You may obtain a copy of the License at
#
#     http://www.apache.org/licenses/LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# See the License for the specific language governing permissions and
# limitations under the License.


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

