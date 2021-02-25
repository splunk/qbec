#!/usr/bin/env bash

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
