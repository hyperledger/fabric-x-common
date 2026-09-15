/*
Copyright IBM Corp. All Rights Reserved.

SPDX-License-Identifier: Apache-2.0
*/

package committerpb_test

import (
	"testing"
	"unicode/utf8"

	"github.com/stretchr/testify/require"

	"github.com/hyperledger/fabric-x-common/api/committerpb"
)

func TestSnapshotAbortKeyRoundTrip(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name     string
		blockNum uint64
	}{
		{name: "zero", blockNum: 0},
		{name: "one", blockNum: 1},
		{name: "small", blockNum: 42},
		{name: "large", blockNum: 1 << 40},
		{name: "max", blockNum: ^uint64(0)},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			key := committerpb.SnapshotAbortKey(tc.blockNum)
			require.False(t, utf8.Valid(key), "abort keys must not be valid UTF-8 transaction IDs")
			require.True(t, committerpb.IsSnapshotAbortKey(key))

			decoded, err := committerpb.BlockNumFromSnapshotAbortKey(key)
			require.NoError(t, err)
			require.Equal(t, tc.blockNum, decoded)
		})
	}
}

// An abort key must never be mistaken for a snapshot record's key, which is a
// transaction ID, because both live in the ns__snapshot table.
func TestIsSnapshotAbortKeyRejectsRecordKeys(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name string
		key  []byte
	}{
		{name: "transaction ID", key: []byte("3f2a1b9c8d7e6f504132231415161718")},
		{name: "transaction ID with abort prefix", key: []byte("abort/foo")},
		{name: "transaction ID matching old abort key", key: []byte("abort/\x01\x07")},
		{name: "unicode transaction ID", key: []byte("快照/7")},
		{name: "empty", key: nil},
		{name: "bare block number without the prefix", key: []byte{1, 7}},
		{name: "prefix in the middle", key: []byte("xabort/")},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			require.False(t, committerpb.IsSnapshotAbortKey(tc.key))

			_, err := committerpb.BlockNumFromSnapshotAbortKey(tc.key)
			require.ErrorContains(t, err, "does not start with")
		})
	}
}

func TestBlockNumFromSnapshotAbortKeyRejectsMalformedKeys(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name        string
		key         []byte
		errContains string
	}{
		{
			name:        "prefix with no block number",
			key:         []byte("\xffabort/"),
			errContains: "failed to decode block number",
		},
		{
			name:        "trailing bytes after the block number",
			key:         append(committerpb.SnapshotAbortKey(7), []byte("junk")...),
			errContains: "trailing bytes",
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			_, err := committerpb.BlockNumFromSnapshotAbortKey(tc.key)
			require.ErrorContains(t, err, tc.errContains)
		})
	}
}

// The encoding is order-preserving so an administrator can range-scan aborts by block
// number directly on the stored key, exactly as for a checkpoint key.
func TestSnapshotAbortKeyIsOrderPreserving(t *testing.T) {
	t.Parallel()
	for i := range uint64(1_000) {
		require.Greater(t, committerpb.SnapshotAbortKey(i+1), committerpb.SnapshotAbortKey(i))
	}
}
