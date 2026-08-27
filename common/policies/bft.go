/*
Copyright IBM Corp. All Rights Reserved.

SPDX-License-Identifier: Apache-2.0
*/

package policies

import (
	"math"

	cb "github.com/hyperledger/fabric-protos-go-apiv2/common"
	mspa "github.com/hyperledger/fabric-protos-go-apiv2/msp"

	"github.com/hyperledger/fabric-x-common/api/msppb"
	"github.com/hyperledger/fabric-x-common/common/policydsl"
	"github.com/hyperledger/fabric-x-common/protoutil"
)

const (
	BlockValidationPolicyKey = "BlockValidation"
)

func EncodeBFTBlockVerificationPolicy(consenterProtos []*cb.Consenter, ordererGroup *cb.ConfigGroup) {
	n := len(consenterProtos)
	f := (n - 1) / 3

	var identities []*mspa.MSPPrincipal
	var pols []*cb.SignaturePolicy
	for i, consenter := range consenterProtos {
		pols = append(pols, &cb.SignaturePolicy{
			Type: &cb.SignaturePolicy_SignedBy{
				SignedBy: int32(i),
			},
		})
		identities = append(identities, &mspa.MSPPrincipal{
			PrincipalClassification: mspa.MSPPrincipal_IDENTITY,
			Principal: protoutil.MarshalOrPanic(
				msppb.NewIdentity(consenter.MspId, consenter.Identity)),
		})
	}

	quorumSize := ComputeBFTQuorum(n, f)
	sp := &cb.SignaturePolicyEnvelope{
		Rule:       policydsl.NOutOf(int32(quorumSize), pols),
		Identities: identities,
	}
	ordererGroup.Policies[BlockValidationPolicyKey] = &cb.ConfigPolicy{
		// Inherit modification policy
		ModPolicy: ordererGroup.Policies[BlockValidationPolicyKey].ModPolicy,
		Policy: &cb.Policy{
			Type:  int32(cb.Policy_SIGNATURE),
			Value: protoutil.MarshalOrPanic(sp),
		},
	}
}

func ComputeBFTQuorum(totalNodes, faultyNodes int) int {
	return int(math.Ceil(float64(totalNodes+faultyNodes+1) / 2))
}

// ComputeFTQ computes the F, Threshold and Quorum corresponds to N.
func ComputeFTQ(n uint16) (f, threshold, quorum uint16) {
	f = (n - 1) / 3
	threshold = f + 1
	quorum = uint16(math.Ceil((float64(n) + float64(f) + 1) / 2.0))
	return f, threshold, quorum
}
