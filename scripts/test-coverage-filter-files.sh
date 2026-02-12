#!/bin/bash
#
# Copyright IBM Corp. All Rights Reserved.
#
# SPDX-License-Identifier: Apache-2.0
#

# We filter some of the files for test coverage reporting.
sed -i -E -f - coverage.profile <<EOF
# The main file cannot be covered by tests as it may call os.Exit(1).
/main\.go/d
# Generated files (e.g., mocks, protobuf) may contain unused methods.
/\.pb(\.gw)?\.go/d
/\/mocks?\//d
/\/fakes?\//d
# Test files that are included in non-test files.
/test_exports?\.go/d
/\/test\//d
/configtest\//d
/testtools\//d
/testprotos\//d
/testutil\//d
/configtest\//d
EOF
