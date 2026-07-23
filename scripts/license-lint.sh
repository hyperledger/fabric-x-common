#!/bin/bash
#
# Copyright IBM Corp. All Rights Reserved.
#
# SPDX-License-Identifier: Apache-2.0
#
REQUIRED_HEADER="SPDX-License-Identifier: Apache-2.0"

# - JSON does not support comments.
# - `goheader` linter already covers the `.go` files.
# - `go.sum` is automatically generated from the `go.mod` file.
# - Agent instructions and skills (`AGENTS.md`, `.bob/`, `.claude/`) are docs/config, not source.
IGNORE_REGEXP="(.+\.(json|pem|yaml|go|pbbin)|go.sum|testdata/.*|sampleconfig/crypto/.*|LICENSE|AGENTS.md|\.bob/.*|\.claude/.*)$"

# Symlinks (e.g. CLAUDE.md -> AGENTS.md) don't carry their own header; skip them.
# `[ -L "$f" ]` is true for symlinks, so the negated test only forwards regular files.
missing=$(git ls-files | sort -u | grep -vE "${IGNORE_REGEXP}" | \
  while read -r f; do [ -L "$f" ] || echo "$f"; done | \
  xargs grep -L "${REQUIRED_HEADER}")

if [[ -z "$missing" ]]; then
  exit 0
fi

echo "Files without license headers:"
echo "------------------------------"
echo "$missing"
echo "--- FAIL"
exit 1
