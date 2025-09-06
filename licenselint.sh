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

YEAR="2025"
OWNER="Splunk Inc."
ERROR=0

addlicense -c "${OWNER}" -l apache -check \
  $(find . -type f ! -path "*testdata*" ! -path "*examples*.yaml" -print0 | xargs -0) \
  || ( echo -e "\nRun 'make fmt-license' to fix missing license headers" && exit 1 )

# array of file patterns to exclude from header check
EXCLUDE_PATTERNS=( \
  "*.json" \
  "*.jsonnet" \
  "*.libsonnet" \
  "*.md" \
  "*.xsonnet" \
  "*testdata*" \
  ".git*" \
  "examples/*.yaml" \
  "go.mod" \
  "go.sum" \
  "LICENSE" 
  "site/*" \
)

# check if the file matches any exclude pattern
exclude_file() {
  for pattern in "${EXCLUDE_PATTERNS[@]}"; do
    if [[ "$1" == $pattern ]]; then
      return 0
    fi
  done
  return 1
}

for file in $(git ls-files); do
  if exclude_file "$file"; then
    continue
  fi
  if ! grep -q "Copyright $YEAR $OWNER" "$file"; then
    echo "Missing or incorrect license header in: $file"
    ERROR=1
  fi
done

exit $ERROR