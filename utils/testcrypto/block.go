/*
Copyright IBM Corp. All Rights Reserved.

SPDX-License-Identifier: Apache-2.0
*/

package testcrypto

import (
	"slices"

	"github.com/IBM/idemix/common/flogging"
	"github.com/hyperledger/fabric-protos-go-apiv2/common"
	"google.golang.org/protobuf/proto"

	"github.com/hyperledger/fabric-x-common/msp"
	"github.com/hyperledger/fabric-x-common/protoutil"
)

// BlockPrepareParameters describe the parameters needed to prepare a valid block.
// Each field is optional, however missing fields may create an invalid block,
// depending on the verification level.
type BlockPrepareParameters struct {
	PrevBlock            *common.Block
	LastConfigBlockIndex uint64
	ConsenterMetadata    []byte
	ConsenterSigners     []msp.SigningIdentity
	// If UseIdentifierHeader==true, use IdentifierHeader instead of SignatureHeader.
	UseIdentifierHeader bool
	// ConsenterIDs for UseIdentifierHeader (must match ConsenterSigners length).
	ConsenterIDs []uint32
}

var logger = flogging.MustGetLogger("testcrypto")

// PrepareBlockHeaderAndMetadata adds a valid header and metadata to the block.
func PrepareBlockHeaderAndMetadata(block *common.Block, p BlockPrepareParameters) *common.Block {
	block = proto.CloneOf(block)
	var blockNumber uint64
	var previousHash []byte
	if p.PrevBlock != nil {
		blockNumber = p.PrevBlock.Header.Number + 1
		previousHash = protoutil.BlockHeaderHash(p.PrevBlock.Header)
	}
	if block.Data == nil {
		block.Data = &common.BlockData{}
	}
	block.Header = &common.BlockHeader{
		Number:       blockNumber,
		DataHash:     protoutil.ComputeBlockDataHash(block.Data),
		PreviousHash: previousHash,
	}
	meta := block.Metadata
	if meta == nil {
		meta = &common.BlockMetadata{}
		block.Metadata = meta
	}
	expectedSize := len(common.BlockMetadataIndex_name)
	meta.Metadata = slices.Grow(meta.Metadata, max(0, expectedSize-cap(meta.Metadata)))[:expectedSize]

	// 1. Prepare the OrdererBlockMetadata payload
	lastConfigIdx := p.LastConfigBlockIndex
	if protoutil.IsConfigBlock(block) {
		// A config block points to itself.
		lastConfigIdx = blockNumber
	}
	ordererMetadata := &common.OrdererBlockMetadata{
		LastConfig:        &common.LastConfig{Index: lastConfigIdx},
		ConsenterMetadata: p.ConsenterMetadata,
	}
	ordererMetadataBytes := protoutil.MarshalOrPanic(ordererMetadata)
	blockHeaderBytes := protoutil.BlockHeaderBytes(block.Header)

	// 2. The value to be signed includes the OrdererBlockMetadata bytes and the Block Header
	// and a Header that is either SignatureHeader (non-BFT) or IdentifierHeader (BFT).
	sigs := make([]*common.MetadataSignature, 0, len(p.ConsenterSigners))
	for i, signer := range p.ConsenterSigners {
		var sig common.MetadataSignature
		messageToSign := &protoutil.MessageToSign{
			BlockHeader:          blockHeaderBytes,
			OrdererBlockMetadata: ordererMetadataBytes,
		}
		if p.UseIdentifierHeader {
			// BFT mode: use IdentifierHeader
			sig.IdentifierHeader = protoutil.MarshalOrPanic(&common.IdentifierHeader{Identifier: p.ConsenterIDs[i]})
			messageToSign.IdentifierHeader = sig.IdentifierHeader
		} else {
			// Non-BFT mode: use SignatureHeader
			creator, err := signer.Serialize()
			if err != nil {
				logger.Warnf("failed to serialize signer: %v", err)
				continue
			}
			sig.SignatureHeader = protoutil.MarshalOrPanic(&common.SignatureHeader{Creator: creator})
			messageToSign.IdentifierHeader = sig.SignatureHeader
		}

		var err error
		sig.Signature, err = signer.Sign(messageToSign.ASN1MarshalOrPanic())
		if err != nil {
			logger.Warnf("failed to sign orderer: %v", err)
			continue
		}
		sigs = append(sigs, &sig)
	}

	// 3. Assemble the final Metadata structure at the SIGNATURES index
	meta.Metadata[common.BlockMetadataIndex_SIGNATURES] = protoutil.MarshalOrPanic(&common.Metadata{
		Value:      ordererMetadataBytes,
		Signatures: sigs,
	})
	return block
}
