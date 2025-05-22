#!/bin/bash
#
# Copyright IBM Corp. All Rights Reserved.
#
# SPDX-License-Identifier: Apache-2.0
#
set -e

# Versions
goimports_version="v0.33.0"
golang_ci_version="v2.0.2"

echo "Installing goimports"
go install "golang.org/x/tools/cmd/goimports@${goimports_version}"

echo
echo "Installing golangci-lint"
curl -sSfL "https://raw.githubusercontent.com/golangci/golangci-lint/master/install.sh"| sh -s -- -b $(go env GOPATH)/bin "${golang_ci_version}"
