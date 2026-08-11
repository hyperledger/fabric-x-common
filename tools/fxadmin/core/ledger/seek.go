/*
Copyright IBM Corp. All Rights Reserved.

SPDX-License-Identifier: Apache-2.0
*/

package ledger

import (
	"math"
	"strconv"

	"github.com/cockroachdb/errors"
	ab "github.com/hyperledger/fabric-protos-go-apiv2/orderer"
)

// latestReference is the block reference that selects the newest block.
const latestReference = "latest"

// seekForReference maps a block reference ("latest" or a block number) to a
// SeekInfo.
func seekForReference(reference string) (*ab.SeekInfo, error) {
	if reference == latestReference {
		return seekNewest(), nil
	}
	number, err := strconv.ParseUint(reference, 10, 64)
	if err != nil {
		return nil, errors.Newf("invalid block reference %q: expected %q or a block number", reference, latestReference)
	}
	return seekByNumber(number), nil
}

// seekNewest builds a SeekInfo that delivers the current newest block.
func seekNewest() *ab.SeekInfo {
	newest := &ab.SeekPosition{Type: &ab.SeekPosition_Newest{Newest: &ab.SeekNewest{}}}
	maxBlock := &ab.SeekPosition{Type: &ab.SeekPosition_Specified{Specified: &ab.SeekSpecified{Number: math.MaxUint64}}}
	return &ab.SeekInfo{
		Start:         newest,
		Stop:          maxBlock,
		Behavior:      ab.SeekInfo_BLOCK_UNTIL_READY,
		ErrorResponse: ab.SeekInfo_BEST_EFFORT,
	}
}

// seekByNumber builds a SeekInfo that delivers exactly one block at the given number.
func seekByNumber(num uint64) *ab.SeekInfo {
	return &ab.SeekInfo{
		Start:         &ab.SeekPosition{Type: &ab.SeekPosition_Specified{Specified: &ab.SeekSpecified{Number: num}}},
		Stop:          &ab.SeekPosition{Type: &ab.SeekPosition_Specified{Specified: &ab.SeekSpecified{Number: num}}},
		Behavior:      ab.SeekInfo_BLOCK_UNTIL_READY,
		ErrorResponse: ab.SeekInfo_BEST_EFFORT,
	}
}
