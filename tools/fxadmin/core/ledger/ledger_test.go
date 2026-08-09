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

// TestHandlerNotImplemented asserts every ledger subcommand is a skeleton that
// reports "not implemented" without panicking. Replace each case with a
// behavioral test as the command is implemented.
func TestHandlerNotImplemented(t *testing.T) {
	t.Parallel()

	for _, tc := range []struct {
		name string
		op   string
	}{
		{name: "height", op: "height"},
		{name: "block", op: "block"},
		{name: "config", op: "config"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			h := ledger.New()

			var err error
			switch tc.op {
			case "height":
				err = h.Height("admin.yaml", "current.pb")
			case "block":
				err = h.Block("admin.yaml", "current.pb", "latest", "out.pb")
			case "config":
				err = h.Config("admin.yaml", "current.pb", "latest", "out.pb")
			}
			require.ErrorContains(t, err, "not implemented")
		})
	}
}
