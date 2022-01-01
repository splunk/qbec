#!/bin/bash

set -euo pipefail
addlicense -check  -c  "Splunk Inc." -l apache **/*.go || "Run 'make license' to fix missing license headers" 
