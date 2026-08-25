/*
Copyright IBM Corp. All Rights Reserved.

SPDX-License-Identifier: Apache-2.0
*/

package seek_test

import (
	"math"
	"testing"

	ab "github.com/hyperledger/fabric-protos-go-apiv2/orderer"
	"github.com/stretchr/testify/require"

	"github.com/hyperledger/fabric-x-common/tools/fxadmin/core/seek"
)

func TestNewest(t *testing.T) {
	t.Parallel()
	info := seek.Newest()
	require.IsType(t, &ab.SeekPosition_Newest{}, info.GetStart().GetType())
	require.Equal(t, uint64(math.MaxUint64), info.GetStop().GetSpecified().GetNumber())
}

func TestByNumber(t *testing.T) {
	t.Parallel()
	info := seek.ByNumber(42)
	require.Equal(t, uint64(42), info.GetStart().GetSpecified().GetNumber())
	require.Equal(t, uint64(42), info.GetStop().GetSpecified().GetNumber())
}

func TestForReference(t *testing.T) {
	t.Parallel()

	t.Run("latest seeks the newest block", func(t *testing.T) {
		t.Parallel()
		info, err := seek.ForReference(seek.LatestReference)
		require.NoError(t, err)
		require.IsType(t, &ab.SeekPosition_Newest{}, info.GetStart().GetType())
	})

	t.Run("number seeks that specific block", func(t *testing.T) {
		t.Parallel()
		info, err := seek.ForReference("42")
		require.NoError(t, err)
		require.Equal(t, uint64(42), info.GetStart().GetSpecified().GetNumber())
		require.Equal(t, uint64(42), info.GetStop().GetSpecified().GetNumber())
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
			_, err := seek.ForReference(tc.reference)
			require.ErrorContains(t, err, "invalid block reference")
		})
	}
}
