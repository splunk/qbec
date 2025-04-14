#!/usr/bin/env bash

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

# the implementation of the config-map data source that shows that you can pull data out
# of arguments, the environment and stdin
cat <<EOF
apiVersion: v1
kind: ConfigMap
metadata:
  name: my-cm-${qbec_env}
data:
  path: ${__DS_PATH__}
  value: "$1"
  message: |
    $(cat -)
EOF
