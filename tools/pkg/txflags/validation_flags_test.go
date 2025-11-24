/*
Copyright IBM Corp. All Rights Reserved.

SPDX-License-Identifier: Apache-2.0
*/

package txflags

import (
	"testing"

	"github.com/hyperledger/fabric-x-common/api/protopeer"
	"github.com/stretchr/testify/require"
)

func TestTransactionValidationFlags(t *testing.T) {
	txFlags := NewWithValues(10, protopeer.TxValidationCode_VALID)
	require.Equal(t, 10, len(txFlags))

	txFlags.SetFlag(0, protopeer.TxValidationCode_VALID)
	require.Equal(t, protopeer.TxValidationCode_VALID, txFlags.Flag(0))
	require.Equal(t, true, txFlags.IsValid(0))

	txFlags.SetFlag(1, protopeer.TxValidationCode_MVCC_READ_CONFLICT)
	require.Equal(t, true, txFlags.IsInvalid(1))
}
