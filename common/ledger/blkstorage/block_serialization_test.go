/*
Copyright IBM Corp. All Rights Reserved.

SPDX-License-Identifier: Apache-2.0
*/

package blkstorage

import (
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

	bb, _ := serializeBlock(block)
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
	bb, info := serializeBlock(block)
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
