#!/bin/bash
set -e

go get
mkdir -p bin
CGO_ENABLED=1 go build  -v -ldflags="-w -s" -o bin/ups-taint