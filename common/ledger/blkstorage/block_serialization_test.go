/*
Copyright IBM Corp. All Rights Reserved.

SPDX-License-Identifier: Apache-2.0
*/

package blkstorage

import (
	"fmt"
	"testing"

	"github.com/hyperledger/fabric-protos-go-apiv2/common"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/encoding/protowire"

	"github.com/hyperledger/fabric-x-common/common/ledger/testutil"
	"github.com/hyperledger/fabric-x-common/protoutil"
)

func TestBlockSerialization(t *testing.T) {
	block := testutil.ConstructTestBlock(t, 1, 10, 100)

	// malformed Payload
	block.Data.Data[1] = protoutil.MarshalOrPanic(&common.Envelope{
		Signature: []byte{1, 2, 3},
		Payload:   []byte("Malformed Payload"),
	})

	// empty TxID
	block.Data.Data[2] = protoutil.MarshalOrPanic(&common.Envelope{
		Signature: []byte{1, 2, 3},
		Payload: protoutil.MarshalOrPanic(&common.Payload{
			Header: &common.Header{
				ChannelHeader: protoutil.MarshalOrPanic(&common.ChannelHeader{
					TxId: "",
				}),
			},
		}),
	})

	bb, _ := serializeBlock(block, allIndexNeeds)
	deserializedBlock, err := deserializeBlock(bb)
	require.NoError(t, err)
	require.Equal(t, block, deserializedBlock)
}

func TestSerializedBlockInfo(t *testing.T) {
	c := &testutilTxIDComputator{
		t:               t,
		malformedTxNums: map[int]struct{}{},
	}

	t.Run("txID is present in all transaction", func(t *testing.T) {
		block := testutil.ConstructTestBlock(t, 1, 10, 100)
		testSerializedBlockInfo(t, block, c)
	})

	t.Run("txID is not present in one of the transactions", func(t *testing.T) {
		block := testutil.ConstructTestBlock(t, 1, 10, 100)
		// empty txid for txNum 2
		block.Data.Data[1] = protoutil.MarshalOrPanic(&common.Envelope{
			Payload: protoutil.MarshalOrPanic(&common.Payload{
				Header: &common.Header{
					ChannelHeader: protoutil.MarshalOrPanic(&common.ChannelHeader{
						TxId: "",
					}),
					SignatureHeader: protoutil.MarshalOrPanic(&common.SignatureHeader{
						Creator: []byte("fake user"),
						Nonce:   []byte("fake nonce"),
					}),
				},
			}),
		})
		testSerializedBlockInfo(t, block, c)
	})

	t.Run("malformed tx-envelop for one of the transactions", func(t *testing.T) {
		block := testutil.ConstructTestBlock(t, 1, 10, 100)
		// malformed Payload for
		block.Data.Data[1] = protoutil.MarshalOrPanic(&common.Envelope{
			Payload: []byte("Malformed Payload"),
		})
		c.reset()
		c.malformedTxNums[1] = struct{}{}
		testSerializedBlockInfo(t, block, c)
	})
}

func testSerializedBlockInfo(t *testing.T, block *common.Block, c *testutilTxIDComputator) {
	bb, info := serializeBlock(block, allIndexNeeds)
	infoFromBB, err := extractSerializedBlockInfo(bb)
	require.NoError(t, err)
	require.Equal(t, info, infoFromBB)
	require.Equal(t, len(block.Data.Data), len(info.txOffsets))
	for txIndex, txEnvBytes := range block.Data.Data {
		txid := c.computeExpectedTxID(txIndex, txEnvBytes)
		indexInfo := info.txOffsets[txIndex]
		indexTxID := indexInfo.txID
		indexOffset := indexInfo.loc

		require.Equal(t, indexTxID, txid)
		b := bb[indexOffset.offset:]
		length, num := protowire.ConsumeVarint(b)
		if num < 0 {
			length, num = 0, 0
		}
		txEnvBytesFromBB := b[num : num+int(length)]
		require.Equal(t, txEnvBytes, txEnvBytesFromBB)
	}
}

type testutilTxIDComputator struct {
	t               *testing.T
	malformedTxNums map[int]struct{}
}

func (c *testutilTxIDComputator) computeExpectedTxID(txNum int, txEnvBytes []byte) string {
	txid, err := protoutil.GetOrComputeTxIDFromEnvelope(txEnvBytes)
	if _, ok := c.malformedTxNums[txNum]; ok {
		require.Error(c.t, err)
	} else {
		require.NoError(c.t, err)
	}
	return txid
}

func (c *testutilTxIDComputator) reset() {
	c.malformedTxNums = map[int]struct{}{}
}

// TestSerializeBlockIndexNeeds pins the two things a caller relies on when it asks for less
// than everything: the bytes on disk do not change, and what it did not ask for is absent.
func TestSerializeBlockIndexNeeds(t *testing.T) {
	t.Parallel()
	block := testutil.ConstructTestBlock(t, 1, 10, 100)
	fullBytes, fullInfo := serializeBlock(block, allIndexNeeds)

	t.Run("offsets without txIDs", func(t *testing.T) {
		t.Parallel()
		bb, info := serializeBlock(block, indexNeeds{txOffsets: true})
		require.Equal(t, fullBytes, bb)
		require.Len(t, info.txOffsets, len(block.Data.Data))
		for txIndex, offset := range info.txOffsets {
			require.Empty(t, offset.txID)
			require.Equal(t, fullInfo.txOffsets[txIndex].loc, offset.loc)
		}
	})

	t.Run("neither offsets nor txIDs", func(t *testing.T) {
		t.Parallel()
		bb, info := serializeBlock(block, indexNeeds{})
		require.Equal(t, fullBytes, bb)
		require.Nil(t, info.txOffsets)
		require.Equal(t, block.Header, info.blockHeader)
		require.Equal(t, block.Metadata, info.metadata)
	})

	t.Run("a block with no transactions has no offsets", func(t *testing.T) {
		t.Parallel()
		empty := testutil.ConstructTestBlock(t, 1, 0, 0)
		for _, needs := range []indexNeeds{allIndexNeeds, {txOffsets: true}, {}} {
			bb, info := serializeBlock(empty, needs)
			require.Nil(t, info.txOffsets)
			infoFromBB, err := extractSerializedBlockInfo(bb)
			require.NoError(t, err)
			require.Equal(t, info, infoFromBB)
		}
	})

	t.Run("the block reads back whatever was asked for", func(t *testing.T) {
		t.Parallel()
		for _, needs := range []indexNeeds{allIndexNeeds, {txOffsets: true}, {}} {
			bb, _ := serializeBlock(block, needs)
			deserialized, err := deserializeBlock(bb)
			require.NoError(t, err)
			require.Equal(t, block, deserialized)
		}
	})
}

// TestBlockIndexSerializationNeeds checks that the index asks for exactly what indexBlock
// reads. The txID index needs both; the blockNum-tranNum index needs the offsets only; a
// block-number-only index, which is what a committer sidecar with the txID index disabled
// configures, needs neither.
func TestBlockIndexSerializationNeeds(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name  string
		attrs []IndexableAttr
		want  indexNeeds
	}{
		{"txID", []IndexableAttr{IndexableAttrTxID}, indexNeeds{txOffsets: true, txIDs: true}},
		{"blockNumTranNum", []IndexableAttr{IndexableAttrBlockNumTranNum}, indexNeeds{txOffsets: true}},
		{"blockNum only", []IndexableAttr{IndexableAttrBlockNum}, indexNeeds{}},
		{"nothing", nil, indexNeeds{}},
		{
			"txID and blockNum",
			[]IndexableAttr{IndexableAttrTxID, IndexableAttrBlockNum},
			indexNeeds{txOffsets: true, txIDs: true},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			index := &blockIndex{indexItemsMap: map[IndexableAttr]bool{}}
			for _, attr := range tc.attrs {
				index.indexItemsMap[attr] = true
			}
			require.Equal(t, tc.want, index.serializationNeeds())
		})
	}
}

// TestSerializedBlockSizeIsExact pins serializedBlockSize to what serializeBlock actually
// writes. An exact pre-allocation means the buffer was never grown, so the returned slice's
// capacity equals its length; an over-estimate leaves slack and an under-estimate reallocates.
func TestSerializedBlockSizeIsExact(t *testing.T) {
	t.Parallel()
	noMetadata := testutil.ConstructTestBlock(t, 1, 3, 50)
	noMetadata.Metadata = nil

	for _, tc := range []struct {
		name  string
		block *common.Block
	}{
		{"full block", testutil.ConstructTestBlock(t, 1, 10, 100)},
		{"no transactions", testutil.ConstructTestBlock(t, 1, 0, 0)},
		{"no metadata", noMetadata},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			bb, _ := serializeBlock(tc.block, allIndexNeeds)
			require.Equal(t, len(bb), cap(bb), "serializedBlockSize is not exact")
		})
	}
}

// BenchmarkSerializeBlock measures serialization, which sits on the critical path of every
// append, across the index configurations a store can be in: the txID index needs both the
// IDs and the offsets, the blockNum-tranNum index only the offsets, and a block-number-only
// store -- a committer sidecar with the txID index disabled -- neither.
func BenchmarkSerializeBlock(b *testing.B) {
	for _, size := range []struct{ numTx, txSize int }{{10, 100}, {500, 300}, {10000, 300}} {
		block := constructBenchmarkBlocks(b, 2, size.numTx, size.txSize)[1]
		for _, tc := range []struct {
			name  string
			needs indexNeeds
		}{
			{"allIndexes", allIndexNeeds},
			{"offsetsOnly", indexNeeds{txOffsets: true}},
			{"noIndex", indexNeeds{}},
		} {
			b.Run(fmt.Sprintf("numTx=%d/txSize=%d/%s", size.numTx, size.txSize, tc.name), func(b *testing.B) {
				b.ReportAllocs()
				for b.Loop() {
					serializeBlock(block, tc.needs)
				}
			})
		}
	}
}

// BenchmarkSerializedBlockSize measures the pre-allocation size pass on its own. It walks the
// envelopes a second time, so it has to stay cheap next to the buffer growth it saves.
func BenchmarkSerializedBlockSize(b *testing.B) {
	block := constructBenchmarkBlocks(b, 2, 10000, 300)[1]
	b.ReportAllocs()
	for b.Loop() {
		serializedBlockSize(block)
	}
}
