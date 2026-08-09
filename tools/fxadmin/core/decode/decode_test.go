/*
Copyright IBM Corp. All Rights Reserved.

SPDX-License-Identifier: Apache-2.0
*/

package decode_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/hyperledger/fabric-x-common/tools/fxadmin/core/decode"
)

// TestRunNotImplemented asserts the decode command is a skeleton that reports
// "not implemented". Replace with a proto-to-JSON round-trip test once the
// command is implemented.
func TestRunNotImplemented(t *testing.T) {
	t.Parallel()
	err := decode.New().Run("block.pb", "out.json")
	require.ErrorContains(t, err, "not implemented")
}
