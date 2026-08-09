/*
Copyright IBM Corp. All Rights Reserved.

SPDX-License-Identifier: Apache-2.0
*/

package update_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/hyperledger/fabric-x-common/tools/fxadmin/core/update"
)

// TestRunNotImplemented asserts the compute-update command is a skeleton that
// reports "not implemented". Replace with a delta-computation test once the
// command is implemented.
func TestRunNotImplemented(t *testing.T) {
	t.Parallel()
	err := update.New().Run("current.json", "modified.json", "update.pb")
	require.ErrorContains(t, err, "not implemented")
}
