#!/bin/bash

set -euo pipefail
addlicense -check  -c  "Splunk Inc." -l apache **/*.go || "Run 'addlicense -c 'Splunk Inc.' -l apache ./**/*.go' to fix missing license headers" 
