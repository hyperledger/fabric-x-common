/*
Copyright IBM Corp. All Rights Reserved.

SPDX-License-Identifier: Apache-2.0
*/

// Package seek builds the SeekInfo messages the fxadmin commands use to pull
// blocks from the orderer: the newest block, a specific block number, or a block
// named by a CLI reference.
package seek

import (
	"math"
	"strconv"

	"github.com/cockroachdb/errors"
	ab "github.com/hyperledger/fabric-protos-go-apiv2/orderer"
)

// LatestReference is the block reference that selects the newest block.
const LatestReference = "latest"

// Newest builds a SeekInfo that delivers the current newest block.
func Newest() *ab.SeekInfo {
	newest := &ab.SeekPosition{Type: &ab.SeekPosition_Newest{Newest: &ab.SeekNewest{}}}
	maxBlock := &ab.SeekPosition{Type: &ab.SeekPosition_Specified{Specified: &ab.SeekSpecified{Number: math.MaxUint64}}}
	return &ab.SeekInfo{
		Start:         newest,
		Stop:          maxBlock,
		Behavior:      ab.SeekInfo_BLOCK_UNTIL_READY,
		ErrorResponse: ab.SeekInfo_BEST_EFFORT,
	}
}

// ByNumber builds a SeekInfo that delivers exactly the block at number.
func ByNumber(number uint64) *ab.SeekInfo {
	at := &ab.SeekPosition{Type: &ab.SeekPosition_Specified{Specified: &ab.SeekSpecified{Number: number}}}
	return &ab.SeekInfo{
		Start:         at,
		Stop:          at,
		Behavior:      ab.SeekInfo_BLOCK_UNTIL_READY,
		ErrorResponse: ab.SeekInfo_BEST_EFFORT,
	}
}

// ForReference maps a block reference ("latest" or a block number) to a SeekInfo.
func ForReference(reference string) (*ab.SeekInfo, error) {
	if reference == LatestReference {
		return Newest(), nil
	}
	number, err := strconv.ParseUint(reference, 10, 64)
	if err != nil {
		return nil, errors.Newf("invalid block reference %q: expected %q or a block number", reference, LatestReference)
	}
	return ByNumber(number), nil
}
