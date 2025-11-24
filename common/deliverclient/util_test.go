/*
Copyright IBM Corp. All Rights Reserved.

SPDX-License-Identifier: Apache-2.0
*/

package deliverclient_test

import (
	"testing"

	"github.com/hyperledger/fabric-x-common/api/protocommon"
	"github.com/stretchr/testify/require"

	"github.com/hyperledger/fabric-x-common/common/deliverclient"
	"github.com/hyperledger/fabric-x-common/protoutil"
)

func TestConfigFromBlockBadInput(t *testing.T) {
	for _, testCase := range []struct {
		name          string
		block         *protocommon.Block
		expectedError string
	}{
		{
			name:          "nil block",
			expectedError: "empty block",
			block:         nil,
		},
		{
			name:          "nil block data",
			expectedError: "empty block",
			block:         &protocommon.Block{},
		},
		{
			name:          "no data in block",
			expectedError: "empty block",
			block:         &protocommon.Block{Data: &protocommon.BlockData{}},
		},
		{
			name:          "invalid payload",
			expectedError: "error unmarshalling Envelope",
			block:         &protocommon.Block{Data: &protocommon.BlockData{Data: [][]byte{{1, 2, 3}}}},
		},
		{
			name:          "bad genesis block",
			expectedError: "invalid config envelope",
			block: &protocommon.Block{
				Header: &protocommon.BlockHeader{}, Data: &protocommon.BlockData{Data: [][]byte{protoutil.MarshalOrPanic(&protocommon.Envelope{
					Payload: protoutil.MarshalOrPanic(&protocommon.Payload{
						Data: []byte{1, 2, 3},
					}),
				})}},
			},
		},
		{
			name:          "invalid envelope in block",
			expectedError: "error unmarshalling Envelope",
			block:         &protocommon.Block{Data: &protocommon.BlockData{Data: [][]byte{{1, 2, 3}}}},
		},
		{
			name:          "invalid payload in block envelope",
			expectedError: "error unmarshalling Payload",
			block: &protocommon.Block{Data: &protocommon.BlockData{Data: [][]byte{protoutil.MarshalOrPanic(&protocommon.Envelope{
				Payload: []byte{1, 2, 3},
			})}}},
		},
		{
			name:          "invalid channel header",
			expectedError: "error unmarshalling ChannelHeader",
			block: &protocommon.Block{
				Header: &protocommon.BlockHeader{Number: 1},
				Data: &protocommon.BlockData{Data: [][]byte{protoutil.MarshalOrPanic(&protocommon.Envelope{
					Payload: protoutil.MarshalOrPanic(&protocommon.Payload{
						Header: &protocommon.Header{
							ChannelHeader: []byte{1, 2, 3},
						},
					}),
				})}},
			},
		},
		{
			name:          "invalid config block",
			expectedError: "invalid config envelope",
			block: &protocommon.Block{
				Header: &protocommon.BlockHeader{},
				Data: &protocommon.BlockData{Data: [][]byte{protoutil.MarshalOrPanic(&protocommon.Envelope{
					Payload: protoutil.MarshalOrPanic(&protocommon.Payload{
						Data: []byte{1, 2, 3},
						Header: &protocommon.Header{
							ChannelHeader: protoutil.MarshalOrPanic(&protocommon.ChannelHeader{
								Type: int32(protocommon.HeaderType_CONFIG),
							}),
						},
					}),
				})}},
			},
		},
	} {
		t.Run(testCase.name, func(t *testing.T) {
			conf, err := deliverclient.ConfigFromBlock(testCase.block)
			require.Nil(t, conf)
			require.Error(t, err)
			require.Contains(t, err.Error(), testCase.expectedError)
		})
	}
}
