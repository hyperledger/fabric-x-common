/*
Copyright IBM Corp. All Rights Reserved.

SPDX-License-Identifier: Apache-2.0
*/

package follow_test

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/hyperledger/fabric-x-common/tools/fxadmin/core/follow"
)

// TestRunNotImplemented asserts the follow command is a skeleton that reports
// "not implemented". Replace with a ledger-polling test once the command is
// implemented.
func TestRunNotImplemented(t *testing.T) {
	t.Parallel()
	err := follow.New().Run("admin.yaml", "current.pb", 30*time.Second)
	require.ErrorContains(t, err, "not implemented")
}
