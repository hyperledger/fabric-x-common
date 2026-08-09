/*
Copyright IBM Corp. All Rights Reserved.

SPDX-License-Identifier: Apache-2.0
*/

package ledger_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/hyperledger/fabric-x-common/tools/fxadmin/core/ledger"
)

const (
	notImplemented = "not implemented"
	adminYAML      = "admin.yaml"
	currentPB      = "current.pb"
)

// TestHandlerNotImplemented asserts every ledger subcommand is a skeleton that
// reports "not implemented" without panicking. Replace each subtest with a
// behavioral test as the command is implemented.
func TestHandlerNotImplemented(t *testing.T) {
	t.Parallel()
	h := ledger.New()

	t.Run("height", func(t *testing.T) {
		t.Parallel()
		require.ErrorContains(t, h.Height(adminYAML, currentPB), notImplemented)
	})
	t.Run("block", func(t *testing.T) {
		t.Parallel()
		require.ErrorContains(t, h.Block(adminYAML, currentPB, "latest", "out.pb"), notImplemented)
	})
	t.Run("config", func(t *testing.T) {
		t.Parallel()
		require.ErrorContains(t, h.Config(adminYAML, currentPB, "latest", "out.pb"), notImplemented)
	})
}
