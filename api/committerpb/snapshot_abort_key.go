/*
Copyright IBM Corp. All Rights Reserved.

SPDX-License-Identifier: Apache-2.0
*/

package committerpb

import (
	"bytes"
	"fmt"

	"github.com/hyperledger/fabric-x-common/common/ledger/util"
)

// snapshotAbortKeyPrefix distinguishes an abort key from a snapshot record's key. Both
// live in the `_snapshot` namespace table, and a record's key is a transaction ID, so
// an abort must be recognizable from the key alone: that is what lets the commit path
// tell an abort transaction from a snapshot request without reading the record it
// names. The sidecar requires transaction IDs to be valid UTF-8. The leading 0xff byte
// cannot occur in valid UTF-8, so an abort key cannot collide with a transaction ID.
var snapshotAbortKeyPrefix = []byte("\xffabort/")

// SnapshotAbortKey returns the `_snapshot` namespace key an abort snapshot transaction
// writes to abandon the snapshot taken at blockNum.
//
// The snapshot is named by block number alone -- a snapshot is a cut of committed state and
// a block holds at most one `_snapshot` transaction, so the transaction number adds no
// distinguishing power -- and the number is encoded order-preserving so aborts can be
// compared and range-scanned directly on the stored key.
func SnapshotAbortKey(blockNum uint64) []byte {
	return append(bytes.Clone(snapshotAbortKeyPrefix), util.EncodeOrderPreservingVarUint64(blockNum)...)
}

// IsSnapshotAbortKey reports whether key is an abort key rather than a snapshot
// record's key.
//
// Only the prefix is inspected: a prefixed key whose block number does not decode is an
// abort transaction with a bad key, which must be rejected as one rather than mistaken
// for a snapshot request.
func IsSnapshotAbortKey(key []byte) bool {
	return bytes.HasPrefix(key, snapshotAbortKeyPrefix)
}

// BlockNumFromSnapshotAbortKey decodes an abort key into the block number of the
// snapshot it abandons.
//
// Trailing bytes are rejected rather than ignored, as for a checkpoint key: accepting a
// prefix would attribute the abort to the wrong snapshot.
func BlockNumFromSnapshotAbortKey(key []byte) (uint64, error) {
	if !IsSnapshotAbortKey(key) {
		return 0, fmt.Errorf("snapshot abort key [%v] does not start with %q", key, snapshotAbortKeyPrefix)
	}
	encoded := key[len(snapshotAbortKeyPrefix):]
	blockNum, n, err := util.DecodeOrderPreservingVarUint64(encoded)
	if err != nil {
		return 0, fmt.Errorf("failed to decode block number from snapshot abort key [%v]: %w", key, err)
	}
	if n != len(encoded) {
		return 0, fmt.Errorf("snapshot abort key [%v] has %d trailing bytes after the block number",
			key, len(encoded)-n)
	}
	return blockNum, nil
}
