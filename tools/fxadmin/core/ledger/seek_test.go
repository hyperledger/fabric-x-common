/*
Copyright IBM Corp. All Rights Reserved.

SPDX-License-Identifier: Apache-2.0
*/

package ledger

import (
	"testing"

	ab "github.com/hyperledger/fabric-protos-go-apiv2/orderer"
	"github.com/stretchr/testify/require"
)

func TestSeekForReference(t *testing.T) {
	t.Parallel()

	t.Run("latest seeks the newest block", func(t *testing.T) {
		t.Parallel()
		seek, err := seekForReference(latestReference)
		require.NoError(t, err)
		require.IsType(t, &ab.SeekPosition_Newest{}, seek.GetStart().GetType())
	})

	t.Run("number seeks that specific block", func(t *testing.T) {
		t.Parallel()
		seek, err := seekForReference("42")
		require.NoError(t, err)
		require.Equal(t, uint64(42), seek.GetStart().GetSpecified().GetNumber())
		require.Equal(t, uint64(42), seek.GetStop().GetSpecified().GetNumber())
	})

	// failure cases
	for _, tc := range []struct {
		name      string
		reference string
	}{
		{name: "non-numeric", reference: "newest"},
		{name: "empty", reference: ""},
		{name: "negative", reference: "-1"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			_, err := seekForReference(tc.reference)
			require.ErrorContains(t, err, "invalid block reference")
		})
	}
}
