/*
Copyright IBM Corp. All Rights Reserved.

SPDX-License-Identifier: Apache-2.0
*/

package protoext

import (
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
	"google.golang.org/protobuf/proto"
)

// Docorate will add additional capabilities to some protobuf messages that
// enable proper JSON marshalling and unmarshalling in protolator.
func Decorate(msg proto.Message) proto.Message {
	switch m := msg.(type) {
	case *protocommon.BlockData:
		return &commonext.BlockData{BlockData: m}
	case *protocommon.Config:
		return &commonext.Config{Config: m}
	case *protocommon.ConfigSignature:
		return &commonext.ConfigSignature{ConfigSignature: m}
	case *protocommon.ConfigUpdate:
		return &commonext.ConfigUpdate{ConfigUpdate: m}
	case *protocommon.ConfigUpdateEnvelope:
		return &commonext.ConfigUpdateEnvelope{ConfigUpdateEnvelope: m}
	case *protocommon.Envelope:
		return &commonext.Envelope{Envelope: m}
	case *protocommon.Header:
		return &commonext.Header{Header: m}
	case *protocommon.ChannelHeader:
		return &commonext.ChannelHeader{ChannelHeader: m}
	case *protocommon.SignatureHeader:
		return &commonext.SignatureHeader{SignatureHeader: m}
	case *protocommon.Payload:
		return &commonext.Payload{Payload: m}
	case *protocommon.Policy:
		return &commonext.Policy{Policy: m}

	case *protomsp.MSPConfig:
		return &mspext.MSPConfig{MSPConfig: m}
	case *protomsp.MSPPrincipal:
		return &mspext.MSPPrincipal{MSPPrincipal: m}

	case *protoorderer.ConsensusType:
		return &ordererext.ConsensusType{ConsensusType: m}

	case *protopeer.ChaincodeAction:
		return &peerext.ChaincodeAction{ChaincodeAction: m}
	case *protopeer.ChaincodeActionPayload:
		return &peerext.ChaincodeActionPayload{ChaincodeActionPayload: m}
	case *protopeer.ChaincodeEndorsedAction:
		return &peerext.ChaincodeEndorsedAction{ChaincodeEndorsedAction: m}
	case *protopeer.ChaincodeProposalPayload:
		return &peerext.ChaincodeProposalPayload{ChaincodeProposalPayload: m}
	case *protopeer.ProposalResponsePayload:
		return &peerext.ProposalResponsePayload{ProposalResponsePayload: m}
	case *protopeer.TransactionAction:
		return &peerext.TransactionAction{TransactionAction: m}

	case *rwset.TxReadWriteSet:
		return &rwsetext.TxReadWriteSet{TxReadWriteSet: m}

	default:
		return msg
	}
}
