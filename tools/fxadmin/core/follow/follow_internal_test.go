/*
Copyright IBM Corp. All Rights Reserved.

SPDX-License-Identifier: Apache-2.0
*/

package follow

import (
	"strings"
	"testing"
	"time"

	cb "github.com/hyperledger/fabric-protos-go-apiv2/common"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/proto"

	"github.com/hyperledger/fabric-x-common/protoutil"
)

// Endpoint addresses reused across the classification and summary tests.
const (
	endpointA = "a:1"
	endpointB = "b:2"
	endpointC = "c:3"
	endpointD = "d:4"
)

// TestClassify asserts an assembler is committed once its last config sequence
// reaches the expected sequence (>=), behind when it is lower, and unreachable
// when it never reported a sequence.
func TestClassify(t *testing.T) {
	t.Parallel()

	require.Equal(t, statusCommitted, classify(assemblerResult{lastConfigSequence: 5, ok: true}, 5))
	// A later config sequence also counts as committed.
	require.Equal(t, statusCommitted, classify(assemblerResult{lastConfigSequence: 6, ok: true}, 5))
	require.Equal(t, statusBehind, classify(assemblerResult{lastConfigSequence: 4, ok: true}, 5))
	require.Equal(t, statusUnreachable, classify(assemblerResult{ok: false}, 5))
}

// TestFormatSummary asserts the summary reports the polling timeout, then a
// header and one row per assembler, in order, reporting each assembler's last
// block number, last config sequence, and status, with dashes for assemblers
// that were never reached.
func TestFormatSummary(t *testing.T) {
	t.Parallel()

	results := []assemblerResult{
		{endpoint: endpointA, lastBlockNumber: 104, lastConfigSequence: 5, ok: true},
		{endpoint: endpointB, lastBlockNumber: 103, lastConfigSequence: 4, ok: true},
		{endpoint: endpointC, ok: false},
	}

	lines := strings.Split(strings.TrimSpace(formatSummary(results, 5, 90*time.Second)), "\n")
	require.Len(t, lines, 5) // timeout + header + three assemblers

	require.Contains(t, lines[0], "polling timeout: 1m30s")
	require.Contains(t, lines[1], "ASSEMBLER")
	require.Contains(t, lines[1], "LAST BLOCK")
	require.Contains(t, lines[1], "LAST CONFIG SEQUENCE")
	require.Contains(t, lines[1], "STATUS")

	require.Regexp(t, `^a:1\s+104\s+5\s+committed$`, lines[2])
	require.Regexp(t, `^b:2\s+103\s+4\s+behind$`, lines[3])
	require.Regexp(t, `^c:3\s+-\s+-\s+unreachable$`, lines[4])
}

// TestNotCommitted asserts notCommitted counts every assembler that has not
// committed a block at the expected sequence, including behind and unreachable.
func TestNotCommitted(t *testing.T) {
	t.Parallel()

	results := []assemblerResult{
		{endpoint: endpointA, lastConfigSequence: 5, ok: true}, // committed
		{endpoint: endpointB, lastConfigSequence: 4, ok: true}, // behind
		{endpoint: endpointC, ok: false},                       // unreachable
	}
	require.Equal(t, 2, notCommitted(results, 5)) // behind + unreachable
	require.Equal(t, 1, notCommitted(results, 4)) // a,b committed; c still unreachable
}

// TestSummaryLine asserts the one-line summary reports the expected sequence and
// how many assemblers committed a block at that sequence.
func TestSummaryLine(t *testing.T) {
	t.Parallel()

	results := []assemblerResult{
		{endpoint: endpointA, lastConfigSequence: 5, ok: true},
		{endpoint: endpointB, lastConfigSequence: 5, ok: true},
		{endpoint: endpointC, lastConfigSequence: 4, ok: true},
		{endpoint: endpointD, ok: false},
	}

	line := summaryLine(results, 5)
	require.Equal(t,
		"expected last config sequence: 5, 2 out of 4 assemblers committed a block with last config sequence 5",
		line)
}

// TestAgreedConfigBlock asserts a block is returned only when the given quorum
// of assemblers report the same block — compared by header and data, ignoring
// the per-assembler signature metadata — at the expected sequence.
//
//nolint:paralleltest
func TestAgreedConfigBlock(t *testing.T) {
	// blockX and blockY are distinct blocks at the same sequence 5,
	// as a divergent assembler would serve.
	blockX := protoutil.NewBlock(5, []byte("x"))
	blockY := protoutil.NewBlock(5, []byte("y"))

	t.Run("returns the block once a quorum reports it", func(t *testing.T) {
		results := []assemblerResult{committedAt(5, blockX), committedAt(5, blockX), committedAt(5, blockY)}
		agreed, count := agreedConfigBlock(results, 5, 2)
		require.NotNil(t, agreed)
		require.Equal(t, 2, count)
		require.True(t, proto.Equal(blockX, agreed))
	})

	t.Run("blocks differing only in metadata agree", func(t *testing.T) {
		// Two assemblers commit the same config block but carry different signature
		// metadata (different order/subset of orderer signatures), as real
		// assemblers do. They must still agree, since agreement ignores metadata.
		signedA := protoutil.NewBlock(5, []byte("h"))
		signedA.Metadata = &cb.BlockMetadata{Metadata: [][]byte{[]byte("sig-A")}}
		signedB := protoutil.NewBlock(5, []byte("h"))
		signedB.Metadata = &cb.BlockMetadata{Metadata: [][]byte{[]byte("sig-B")}}
		require.Equal(t, protoutil.BlockHeaderHash(signedA.GetHeader()), protoutil.BlockHeaderHash(signedB.GetHeader()),
			"sanity: the two blocks share a header hash and differ only in metadata")

		agreed, count := agreedConfigBlock([]assemblerResult{committedAt(5, signedA), committedAt(5, signedB)}, 5, 2)
		require.NotNil(t, agreed)
		require.Equal(t, 2, count)
	})

	t.Run("blocks differing in data do not agree", func(t *testing.T) {
		// Same sequence but different data must not agree, even at quorum 2.
		agreed, count := agreedConfigBlock([]assemblerResult{committedAt(5, blockX), committedAt(5, blockY)}, 5, 2)
		require.Nil(t, agreed)
		require.Zero(t, count)
	})

	t.Run("nil when no block reaches the quorum", func(t *testing.T) {
		// Three assemblers at sequence 5 but each on a different block: no block
		// gathers 2 identical copies.
		results := []assemblerResult{
			committedAt(5, blockX), committedAt(5, blockY), committedAt(5, protoutil.NewBlock(5, []byte("z"))),
		}
		agreed, count := agreedConfigBlock(results, 5, 2)
		require.Nil(t, agreed)
		require.Zero(t, count)
	})

	t.Run("quorum derived from party count may exceed len(results)", func(t *testing.T) {
		// Two reachable assemblers agree, but the network has more parties (quorum 3),
		// so their agreement is not yet enough.
		results := []assemblerResult{committedAt(5, blockX), committedAt(5, blockX)}
		agreed, count := agreedConfigBlock(results, 5, 3)
		require.Nil(t, agreed)
		require.Zero(t, count)
	})

	t.Run("ignores results at the wrong sequence or without a block", func(t *testing.T) {
		results := []assemblerResult{
			committedAt(4, blockX),             // behind: wrong sequence
			{lastConfigSequence: 5, ok: false}, // unreachable: no block
			{lastConfigSequence: 5, ok: true},  // committed but nil block
			committedAt(5, blockX),
		}
		agreed, _ := agreedConfigBlock(results, 5, 2)
		require.Nil(t, agreed) // only one valid copy of blockX
	})
}

// committedAt returns an assemblerResult for an assembler that reported block at
// the given last config sequence.
func committedAt(sequence uint64, block *cb.Block) assemblerResult {
	return assemblerResult{lastConfigSequence: sequence, configBlock: block, ok: true}
}
