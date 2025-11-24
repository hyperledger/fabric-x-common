/*
Copyright IBM Corp. All Rights Reserved.

SPDX-License-Identifier: Apache-2.0
*/

package peerext

import (
	"fmt"

	"github.com/hyperledger/fabric-x-common/api/protocommon"
	"github.com/hyperledger/fabric-x-common/api/protopeer"
	"google.golang.org/protobuf/proto"
)

type TransactionAction struct { // nothing was testing this
	*protopeer.TransactionAction
}

func (ta *TransactionAction) Underlying() proto.Message {
	return ta.TransactionAction
}

func (ta *TransactionAction) StaticallyOpaqueFields() []string {
	return []string{"header", "payload"}
}

func (ta *TransactionAction) StaticallyOpaqueFieldProto(name string) (proto.Message, error) {
	switch name {
	case ta.StaticallyOpaqueFields()[0]:
		return &protocommon.SignatureHeader{}, nil
	case ta.StaticallyOpaqueFields()[1]:
		return &protopeer.ChaincodeActionPayload{}, nil
	default:
		return nil, fmt.Errorf("not a marshaled field: %s", name)
	}
}

type ChaincodeActionPayload struct {
	*protopeer.ChaincodeActionPayload
}

func (cap *ChaincodeActionPayload) Underlying() proto.Message {
	return cap.ChaincodeActionPayload
}

func (cap *ChaincodeActionPayload) StaticallyOpaqueFields() []string {
	return []string{"chaincode_proposal_payload"}
}

func (cap *ChaincodeActionPayload) StaticallyOpaqueFieldProto(name string) (proto.Message, error) {
	if name != cap.StaticallyOpaqueFields()[0] {
		return nil, fmt.Errorf("not a marshaled field: %s", name)
	}
	return &protopeer.ChaincodeProposalPayload{}, nil
}

type ChaincodeEndorsedAction struct {
	*protopeer.ChaincodeEndorsedAction
}

func (cae *ChaincodeEndorsedAction) Underlying() proto.Message {
	return cae.ChaincodeEndorsedAction
}

func (cae *ChaincodeEndorsedAction) StaticallyOpaqueFields() []string {
	return []string{"proposal_response_payload"}
}

func (cae *ChaincodeEndorsedAction) StaticallyOpaqueFieldProto(name string) (proto.Message, error) {
	if name != cae.StaticallyOpaqueFields()[0] {
		return nil, fmt.Errorf("not a marshaled field: %s", name)
	}
	return &protopeer.ProposalResponsePayload{}, nil
}
