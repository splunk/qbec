#!/bin/bash

set -euo pipefail
ds_path=${DATA_SOURCE_PATH}

if [[ "${ds_path}" == "/john" ]]
then
  cat <<'EOF'
{
  "name": "John Doe",
  "address": "1 Main St, San Jose"
}
EOF
elif [[ "${ds_path}" == "/jane" ]];then
  cat <<'EOF'
{
  "name": "Jane Doe",
  "address": "1 Main St, San Francisco"
}
EOF
else
  echo "Unsupported path: ${ds_path}" >&2
  exit 1
fi
