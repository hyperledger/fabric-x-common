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

// TestHandlerNotImplemented asserts every tx subcommand is a skeleton that
// reports "not implemented" without panicking. Replace each case with a
// behavioral test as the command is implemented.
func TestHandlerNotImplemented(t *testing.T) {
	t.Parallel()

	for _, tc := range []struct {
		name string
		op   string
	}{
		{name: "endorse", op: "endorse"},
		{name: "merge", op: "merge"},
		{name: "prepare", op: "prepare"},
		{name: "submit", op: "submit"},
		{name: "send", op: "send"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			h := tx.New()

			var err error
			switch tc.op {
			case "endorse":
				err = h.Endorse("update.pb", "admin.yaml", "endorsement.pb")
			case "merge":
				err = h.Merge([]string{"e1.pb", "e2.pb"}, "merged.pb")
			case "prepare":
				err = h.Prepare("endorsed.pb", "admin.yaml", "tx.pb")
			case "submit":
				err = h.Submit("tx.pb", "admin.yaml", "current.pb")
			case "send":
				err = h.Send("endorsed.pb", "admin.yaml", "current.pb")
			}
			require.ErrorContains(t, err, "not implemented")
		})
	}
}
