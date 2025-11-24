/*
Copyright IBM Corp. All Rights Reserved.

SPDX-License-Identifier: Apache-2.0
*/

package peerext

import (
	"fmt"

	"github.com/hyperledger/fabric-x-common/api/protopeer"
	"google.golang.org/protobuf/proto"
)

type ProposalResponsePayload struct {
	*protopeer.ProposalResponsePayload
}

func (ppr *ProposalResponsePayload) Underlying() proto.Message {
	return ppr.ProposalResponsePayload
}

func (ppr *ProposalResponsePayload) StaticallyOpaqueFields() []string {
	return []string{"extension"}
}

func (ppr *ProposalResponsePayload) StaticallyOpaqueFieldProto(name string) (proto.Message, error) {
	if name != ppr.StaticallyOpaqueFields()[0] {
		return nil, fmt.Errorf("not a marshaled field: %s", name)
	}
	return &protopeer.ChaincodeAction{}, nil
}
