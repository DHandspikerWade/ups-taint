#!/bin/bash
set -e

go get
CGO_ENABLED=0 go build  -v -ldflags="-w -s" -o ups-taint