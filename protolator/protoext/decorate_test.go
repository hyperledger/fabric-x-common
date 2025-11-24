/*
Copyright IBM Corp. All Rights Reserved.

SPDX-License-Identifier: Apache-2.0
*/

package protoext

import (
	"testing"

	"github.com/hyperledger/fabric-protos-go-apiv2/ledger/rwset"
	"github.com/hyperledger/fabric-x-common/api/protocommon"
	"github.com/hyperledger/fabric-x-common/api/protomsp"
	"github.com/hyperledger/fabric-x-common/api/protoorderer"
	"github.com/hyperledger/fabric-x-common/api/protopeer"
	"github.com/hyperledger/fabric-x-common/protolator/protoext/commonext"
	"github.com/hyperledger/fabric-x-common/protolator/protoext/ledger/rwsetext"
	"github.com/hyperledger/fabric-x-common/protolator/protoext/mspext"
	"github.com/hyperledger/fabric-x-common/protolator/protoext/ordererext"
	"github.com/hyperledger/fabric-x-common/protolator/protoext/peerext"
	. "github.com/onsi/gomega"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/protoadapt"
)

type GenericProtoMessage struct {
	GenericField string
}

func (g *GenericProtoMessage) Reset() {
	panic("not implemented")
}

func (g *GenericProtoMessage) String() string {
	return "not implemented"
}

func (g *GenericProtoMessage) ProtoMessage() {
	panic("not implemented")
}

func TestDecorate(t *testing.T) {
	tests := []struct {
		testSpec       string
		msg            proto.Message
		expectedReturn proto.Message
	}{
		{
			testSpec: "common.BlockData",
			msg: &protocommon.BlockData{
				Data: [][]byte{
					[]byte("data-bytes"),
				},
			},
			expectedReturn: &commonext.BlockData{
				BlockData: &protocommon.BlockData{
					Data: [][]byte{
						[]byte("data-bytes"),
					},
				},
			},
		},
		{
			testSpec: "common.Config",
			msg: &protocommon.Config{
				Sequence: 5,
			},
			expectedReturn: &commonext.Config{
				Config: &protocommon.Config{
					Sequence: 5,
				},
			},
		},
		{
			testSpec: "common.ConfigSignature",
			msg: &protocommon.ConfigSignature{
				SignatureHeader: []byte("signature-header-bytes"),
			},
			expectedReturn: &commonext.ConfigSignature{
				ConfigSignature: &protocommon.ConfigSignature{
					SignatureHeader: []byte("signature-header-bytes"),
				},
			},
		},
		{
			testSpec: "common.ConfigUpdate",
			msg: &protocommon.ConfigUpdate{
				ChannelId: "testchannel",
			},
			expectedReturn: &commonext.ConfigUpdate{
				ConfigUpdate: &protocommon.ConfigUpdate{
					ChannelId: "testchannel",
				},
			},
		},
		{
			testSpec: "common.ConfigUpdateEnvelope",
			msg: &protocommon.ConfigUpdateEnvelope{
				ConfigUpdate: []byte("config-update-bytes"),
			},
			expectedReturn: &commonext.ConfigUpdateEnvelope{
				ConfigUpdateEnvelope: &protocommon.ConfigUpdateEnvelope{
					ConfigUpdate: []byte("config-update-bytes"),
				},
			},
		},
		{
			testSpec: "common.Envelope",
			msg: &protocommon.Envelope{
				Payload: []byte("payload-bytes"),
			},
			expectedReturn: &commonext.Envelope{
				Envelope: &protocommon.Envelope{
					Payload: []byte("payload-bytes"),
				},
			},
		},
		{
			testSpec: "common.Header",
			msg: &protocommon.Header{
				ChannelHeader: []byte("channel-header-bytes"),
			},
			expectedReturn: &commonext.Header{
				Header: &protocommon.Header{
					ChannelHeader: []byte("channel-header-bytes"),
				},
			},
		},
		{
			testSpec: "common.ChannelHeader",
			msg: &protocommon.ChannelHeader{
				Type: 5,
			},
			expectedReturn: &commonext.ChannelHeader{
				ChannelHeader: &protocommon.ChannelHeader{
					Type: 5,
				},
			},
		},
		{
			testSpec: "common.SignatureHeader",
			msg: &protocommon.SignatureHeader{
				Creator: []byte("creator-bytes"),
			},
			expectedReturn: &commonext.SignatureHeader{
				SignatureHeader: &protocommon.SignatureHeader{
					Creator: []byte("creator-bytes"),
				},
			},
		},
		{
			testSpec: "common.Payload",
			msg: &protocommon.Payload{
				Header: &protocommon.Header{ChannelHeader: []byte("channel-header-bytes")},
			},
			expectedReturn: &commonext.Payload{
				Payload: &protocommon.Payload{
					Header: &protocommon.Header{ChannelHeader: []byte("channel-header-bytes")},
				},
			},
		},
		{
			testSpec: "common.Policy",
			msg: &protocommon.Policy{
				Type: 5,
			},
			expectedReturn: &commonext.Policy{
				Policy: &protocommon.Policy{
					Type: 5,
				},
			},
		},
		{
			testSpec: "msp.MSPConfig",
			msg: &protomsp.MSPConfig{
				Type: 5,
			},
			expectedReturn: &mspext.MSPConfig{
				MSPConfig: &protomsp.MSPConfig{
					Type: 5,
				},
			},
		},
		{
			testSpec: "msp.MSPPrincipal",
			msg: &protomsp.MSPPrincipal{
				Principal: []byte("principal-bytes"),
			},
			expectedReturn: &mspext.MSPPrincipal{
				MSPPrincipal: &protomsp.MSPPrincipal{
					Principal: []byte("principal-bytes"),
				},
			},
		},
		{
			testSpec: "orderer.ConsensusType",
			msg: &protoorderer.ConsensusType{
				Type: "etcdraft",
			},
			expectedReturn: &ordererext.ConsensusType{
				ConsensusType: &protoorderer.ConsensusType{
					Type: "etcdraft",
				},
			},
		},
		{
			testSpec: "peer.ChaincodeAction",
			msg: &protopeer.ChaincodeAction{
				Results: []byte("results-bytes"),
			},
			expectedReturn: &peerext.ChaincodeAction{
				ChaincodeAction: &protopeer.ChaincodeAction{
					Results: []byte("results-bytes"),
				},
			},
		},
		{
			testSpec: "peer.ChaincodeActionPayload",
			msg: &protopeer.ChaincodeActionPayload{
				ChaincodeProposalPayload: []byte("chaincode-proposal-payload-bytes"),
			},
			expectedReturn: &peerext.ChaincodeActionPayload{
				ChaincodeActionPayload: &protopeer.ChaincodeActionPayload{
					ChaincodeProposalPayload: []byte("chaincode-proposal-payload-bytes"),
				},
			},
		},
		{
			testSpec: "peer.ChaincodeEndorsedAction",
			msg: &protopeer.ChaincodeEndorsedAction{
				ProposalResponsePayload: []byte("proposal-response-payload-bytes"),
			},
			expectedReturn: &peerext.ChaincodeEndorsedAction{
				ChaincodeEndorsedAction: &protopeer.ChaincodeEndorsedAction{
					ProposalResponsePayload: []byte("proposal-response-payload-bytes"),
				},
			},
		},
		{
			testSpec: "peer.ChaincodeProposalPayload",
			msg: &protopeer.ChaincodeProposalPayload{
				Input: []byte("input-bytes"),
			},
			expectedReturn: &peerext.ChaincodeProposalPayload{
				ChaincodeProposalPayload: &protopeer.ChaincodeProposalPayload{
					Input: []byte("input-bytes"),
				},
			},
		},
		{
			testSpec: "peer.ProposalResponsePayload",
			msg: &protopeer.ProposalResponsePayload{
				ProposalHash: []byte("proposal-hash-bytes"),
			},
			expectedReturn: &peerext.ProposalResponsePayload{
				ProposalResponsePayload: &protopeer.ProposalResponsePayload{
					ProposalHash: []byte("proposal-hash-bytes"),
				},
			},
		},
		{
			testSpec: "peer.TransactionAction",
			msg: &protopeer.TransactionAction{
				Header: []byte("header-bytes"),
			},
			expectedReturn: &peerext.TransactionAction{
				TransactionAction: &protopeer.TransactionAction{
					Header: []byte("header-bytes"),
				},
			},
		},
		{
			testSpec: "rwset.TxReadWriteSet",
			msg: &rwset.TxReadWriteSet{
				NsRwset: []*rwset.NsReadWriteSet{
					{
						Namespace: "namespace",
					},
				},
			},
			expectedReturn: &rwsetext.TxReadWriteSet{
				TxReadWriteSet: &rwset.TxReadWriteSet{
					NsRwset: []*rwset.NsReadWriteSet{
						{
							Namespace: "namespace",
						},
					},
				},
			},
		},
		{
			testSpec: "default",
			msg: protoadapt.MessageV2Of(&GenericProtoMessage{
				GenericField: "test",
			}),
			expectedReturn: protoadapt.MessageV2Of(&GenericProtoMessage{
				GenericField: "test",
			}),
		},
	}

	for _, tt := range tests {
		t.Run(tt.testSpec, func(t *testing.T) {
			gt := NewGomegaWithT(t)
			decoratedMsg := Decorate(tt.msg)
			gt.Expect(proto.Equal(decoratedMsg, tt.expectedReturn)).To(BeTrue())
		})
	}
}
