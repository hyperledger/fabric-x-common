/*
Copyright IBM Corp. All Rights Reserved.

SPDX-License-Identifier: Apache-2.0
*/

package blkstorage

import (
	"github.com/hyperledger/fabric-protos-go-apiv2/common"
	"github.com/pkg/errors"
	"google.golang.org/protobuf/encoding/protowire"

	"github.com/hyperledger/fabric-x-common/protoutil"
)

type serializedBlockInfo struct {
	blockHeader *common.BlockHeader
	txOffsets   []*txindexInfo
	metadata    *common.BlockMetadata
}

// The order of the transactions must be maintained for history
type txindexInfo struct {
	txID string
	loc  *locPointer
}

// indexNeeds says which of the per transaction facts an index will actually read back out
// of a serialized block. Serialization sits on the critical path of every append, and both
// of these cost real work at block scale: the offsets are an allocation per transaction,
// and a txID is a full envelope unmarshal each. Neither is produced unless an index reads
// it -- see blockIndex.indexBlock, where txOffsets is touched only under
// IndexableAttrTxID or IndexableAttrBlockNumTranNum, and txID only under the former.
type indexNeeds struct {
	txOffsets bool
	txIDs     bool
}

// allIndexNeeds asks for everything serialization can produce. For callers with no index to
// consult, which is what the block store's own tests and the read-back paths want.
var allIndexNeeds = indexNeeds{txOffsets: true, txIDs: true}

func serializeBlock(block *common.Block, needs indexNeeds) ([]byte, *serializedBlockInfo) {
	buf := make([]byte, 0, serializedBlockSize(block))
	buf = appendHeaderBytes(buf, block.Header)
	buf = protowire.AppendVarint(buf, uint64(len(block.Data.Data)))
	dataStart := len(buf)
	for _, txEnvelopeBytes := range block.Data.Data {
		buf = protowire.AppendBytes(buf, txEnvelopeBytes)
	}
	buf = appendMetadataBytes(buf, block.Metadata)

	info := &serializedBlockInfo{
		blockHeader: block.Header,
		metadata:    block.Metadata,
	}
	// Nothing reads the offsets back, or there are none to build: return the nil slice that
	// extractData also returns for an empty block.
	if needs.txOffsets && len(block.Data.Data) > 0 {
		info.txOffsets = constructTxIndexInfo(block.Data.Data, dataStart, needs)
	}
	return buf, info
}

func deserializeBlock(serializedBlockBytes []byte) (*common.Block, error) {
	block := &common.Block{}
	var err error
	b := newBuffer(serializedBlockBytes)
	if block.Header, err = extractHeader(b); err != nil {
		return nil, err
	}
	if block.Data, _, err = extractData(b); err != nil {
		return nil, err
	}
	if block.Metadata, err = extractMetadata(b); err != nil {
		return nil, err
	}
	return block, nil
}

func extractSerializedBlockInfo(serializedBlockBytes []byte) (*serializedBlockInfo, error) {
	info := &serializedBlockInfo{}
	var err error
	b := newBuffer(serializedBlockBytes)
	info.blockHeader, err = extractHeader(b)
	if err != nil {
		return nil, err
	}
	_, info.txOffsets, err = extractData(b)
	if err != nil {
		return nil, err
	}

	info.metadata, err = extractMetadata(b)
	if err != nil {
		return nil, err
	}
	return info, nil
}

// serializedBlockSize is the exact length serializeBlock produces, so the buffer is
// allocated once up front rather than grown a few times per block. It mirrors the writers
// below: every protowire.Append call has a Size counterpart of the same shape.
func serializedBlockSize(block *common.Block) int {
	size := protowire.SizeVarint(block.Header.Number) +
		protowire.SizeBytes(len(block.Header.DataHash)) +
		protowire.SizeBytes(len(block.Header.PreviousHash)) +
		protowire.SizeVarint(uint64(len(block.Data.Data)))
	for _, txEnvelopeBytes := range block.Data.Data {
		size += protowire.SizeBytes(len(txEnvelopeBytes))
	}

	var metadata [][]byte
	if block.Metadata != nil {
		metadata = block.Metadata.Metadata
	}
	size += protowire.SizeVarint(uint64(len(metadata)))
	for _, b := range metadata {
		size += protowire.SizeBytes(len(b))
	}
	return size
}

func appendHeaderBytes(buf []byte, blockHeader *common.BlockHeader) []byte {
	buf = protowire.AppendVarint(buf, blockHeader.Number)
	buf = protowire.AppendBytes(buf, blockHeader.DataHash)
	buf = protowire.AppendBytes(buf, blockHeader.PreviousHash)
	return buf
}

// constructTxIndexInfo locates each envelope in the block bytes without re-reading them:
// serializeBlock wrote them consecutively from dataStart, each as a length prefix followed by
// the envelope, so protowire.SizeBytes gives the span of each.
// The infos and their locations come out of one backing array each rather than two
// allocations per transaction; the elements are only ever addressed, never copied out, so
// pointers into them are stable for as long as the slices are.
func constructTxIndexInfo(txEnvelopes [][]byte, dataStart int, needs indexNeeds) []*txindexInfo {
	infos := make([]txindexInfo, len(txEnvelopes))
	locs := make([]locPointer, len(txEnvelopes))
	txOffsets := make([]*txindexInfo, len(txEnvelopes))

	offset := dataStart
	for i, txEnvelopeBytes := range txEnvelopes {
		size := protowire.SizeBytes(len(txEnvelopeBytes))
		locs[i] = locPointer{offset, size}
		offset += size

		if needs.txIDs {
			txid, err := protoutil.GetOrComputeTxIDFromEnvelope(txEnvelopeBytes)
			if err != nil {
				logger.Warningf("error while extracting txid from tx envelope bytes during "+
					"serialization of block. Ignoring this error as this is caused by a "+
					"malformed transaction. Error:%s", err)
			}
			infos[i].txID = txid
		}
		infos[i].loc = &locs[i]
		txOffsets[i] = &infos[i]
	}
	return txOffsets
}

func appendMetadataBytes(buf []byte, blockMetadata *common.BlockMetadata) []byte {
	numItems := uint64(0)
	if blockMetadata != nil {
		numItems = uint64(len(blockMetadata.Metadata))
	}
	buf = protowire.AppendVarint(buf, numItems)
	if blockMetadata == nil {
		return buf
	}
	for _, b := range blockMetadata.Metadata {
		buf = protowire.AppendBytes(buf, b)
	}
	return buf
}

func extractHeader(buf *buffer) (*common.BlockHeader, error) {
	header := &common.BlockHeader{}
	var err error
	if header.Number, err = buf.DecodeVarint(); err != nil {
		return nil, errors.Wrap(err, "error decoding the block number")
	}
	if header.DataHash, err = buf.DecodeRawBytes(false); err != nil {
		return nil, errors.Wrap(err, "error decoding the data hash")
	}
	if header.PreviousHash, err = buf.DecodeRawBytes(false); err != nil {
		return nil, errors.Wrap(err, "error decoding the previous hash")
	}
	if len(header.PreviousHash) == 0 {
		header.PreviousHash = nil
	}
	return header, nil
}

func extractData(buf *buffer) (*common.BlockData, []*txindexInfo, error) {
	data := &common.BlockData{}
	var txOffsets []*txindexInfo
	var numItems uint64
	var err error

	if numItems, err = buf.DecodeVarint(); err != nil {
		return nil, nil, errors.Wrap(err, "error decoding the length of block data")
	}
	for i := uint64(0); i < numItems; i++ {
		var txEnvBytes []byte
		var txid string
		txOffset := buf.GetBytesConsumed()
		if txEnvBytes, err = buf.DecodeRawBytes(false); err != nil {
			return nil, nil, errors.Wrap(err, "error decoding the transaction envelope")
		}
		if txid, err = protoutil.GetOrComputeTxIDFromEnvelope(txEnvBytes); err != nil {
			logger.Warningf("error while extracting txid from tx envelope bytes during deserialization of block. Ignoring this error as this is caused by a malformed transaction. Error:%s",
				err)
		}
		data.Data = append(data.Data, txEnvBytes)
		idxInfo := &txindexInfo{txID: txid, loc: &locPointer{txOffset, buf.GetBytesConsumed() - txOffset}}
		txOffsets = append(txOffsets, idxInfo)
	}
	return data, txOffsets, nil
}

func extractMetadata(buf *buffer) (*common.BlockMetadata, error) {
	metadata := &common.BlockMetadata{}
	var numItems uint64
	var metadataEntry []byte
	var err error
	if numItems, err = buf.DecodeVarint(); err != nil {
		return nil, errors.Wrap(err, "error decoding the length of block metadata")
	}
	for i := uint64(0); i < numItems; i++ {
		if metadataEntry, err = buf.DecodeRawBytes(false); err != nil {
			return nil, errors.Wrap(err, "error decoding the block metadata")
		}
		metadata.Metadata = append(metadata.Metadata, metadataEntry)
	}
	return metadata, nil
}
