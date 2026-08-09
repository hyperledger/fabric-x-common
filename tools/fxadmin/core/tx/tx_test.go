/*
Copyright IBM Corp. All Rights Reserved.

SPDX-License-Identifier: Apache-2.0
*/

package tx_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/hyperledger/fabric-x-common/tools/fxadmin/core/tx"
)

const (
	notImplemented = "not implemented"
	adminYAML      = "admin.yaml"
)

// TestHandlerNotImplemented asserts every tx subcommand is a skeleton that
// reports "not implemented" without panicking. Replace each subtest with a
// behavioral test as the command is implemented.
func TestHandlerNotImplemented(t *testing.T) {
	t.Parallel()
	h := tx.New()

	t.Run("endorse", func(t *testing.T) {
		t.Parallel()
		require.ErrorContains(t, h.Endorse("update.pb", adminYAML, "endorsement.pb"), notImplemented)
	})
	t.Run("merge", func(t *testing.T) {
		t.Parallel()
		require.ErrorContains(t, h.Merge([]string{"e1.pb", "e2.pb"}, "merged.pb"), notImplemented)
	})
	t.Run("prepare", func(t *testing.T) {
		t.Parallel()
		require.ErrorContains(t, h.Prepare("endorsed.pb", adminYAML, "tx.pb"), notImplemented)
	})
	t.Run("submit", func(t *testing.T) {
		t.Parallel()
		require.ErrorContains(t, h.Submit("tx.pb", adminYAML, "current.pb"), notImplemented)
	})
	t.Run("send", func(t *testing.T) {
		t.Parallel()
		require.ErrorContains(t, h.Send("endorsed.pb", adminYAML, "current.pb"), notImplemented)
	})
}
