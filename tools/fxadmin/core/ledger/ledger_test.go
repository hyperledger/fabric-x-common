/*
Copyright IBM Corp. All Rights Reserved.

SPDX-License-Identifier: Apache-2.0
*/

package ledger_test

import (
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/hyperledger/fabric-x-common/tools/fxadmin/core/ledger"
)

// TestConfigRejectsNonLatestReference asserts `ledger config` only accepts the
// "latest" reference, and rejects other references before any network access.
func TestConfigRejectsNonLatestReference(t *testing.T) {
	t.Parallel()
	h := ledger.New()
	out := filepath.Join(t.TempDir(), "out.pb")

	for _, reference := range []string{"0", "42", "newest", ""} {
		t.Run(reference, func(t *testing.T) {
			t.Parallel()
			err := h.Config("admin.yaml", "current.pb", reference, out)
			require.ErrorContains(t, err, "unsupported config reference")
		})
	}
}

// TestReferenceValidatedBeforeConfigLoad asserts an invalid `ledger block`
// reference is rejected before the admin config or block are read.
func TestReferenceValidatedBeforeConfigLoad(t *testing.T) {
	t.Parallel()
	h := ledger.New()
	out := filepath.Join(t.TempDir(), "out.pb")

	err := h.Block(filepath.Join(t.TempDir(), "absent.yaml"), "current.pb", "not-a-number", out)
	require.ErrorContains(t, err, "invalid block reference")
}
