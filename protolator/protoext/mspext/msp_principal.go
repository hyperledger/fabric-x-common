/*
Copyright IBM Corp. All Rights Reserved.

SPDX-License-Identifier: Apache-2.0
*/

package mspext

import (
	"fmt"

	"github.com/hyperledger/fabric-x-common/api/protomsp"
	"google.golang.org/protobuf/proto"
)

type MSPPrincipal struct{ *protomsp.MSPPrincipal }

func (mp *MSPPrincipal) Underlying() proto.Message {
	return mp.MSPPrincipal
}

func (mp *MSPPrincipal) VariablyOpaqueFields() []string {
	return []string{"principal"}
}

func (mp *MSPPrincipal) VariablyOpaqueFieldProto(name string) (proto.Message, error) {
	if name != mp.VariablyOpaqueFields()[0] {
		return nil, fmt.Errorf("not a marshaled field: %s", name)
	}
	switch mp.PrincipalClassification {
	case protomsp.MSPPrincipal_ROLE:
		return &protomsp.MSPRole{}, nil
	case protomsp.MSPPrincipal_ORGANIZATION_UNIT:
		return &protomsp.OrganizationUnit{}, nil
	case protomsp.MSPPrincipal_IDENTITY:
		return &protomsp.SerializedIdentity{}, nil
	default:
		return nil, fmt.Errorf("unable to decode MSP type: %v", mp.PrincipalClassification)
	}
}
