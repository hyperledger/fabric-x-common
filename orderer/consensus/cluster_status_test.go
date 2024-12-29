/*
Copyright IBM Corp. 2017 All Rights Reserved.

SPDX-License-Identifier: Apache-2.0
*/

package consensus_test

import (
	"testing"

	"github.com/stretchr/testify/require"
	"github.ibm.com/decentralized-trust-research/fabricx-config/orderer/common/types"
	"github.ibm.com/decentralized-trust-research/fabricx-config/orderer/consensus"
)

func TestStaticStatusReporter(t *testing.T) {
	staticSR := &consensus.StaticStatusReporter{
		ConsensusRelation: types.ConsensusRelationOther,
		Status:            types.StatusActive,
	}

	var sr consensus.StatusReporter = staticSR // make sure it implements this interface
	cRel, status := sr.StatusReport()
	require.Equal(t, types.ConsensusRelationOther, cRel)
	require.Equal(t, types.StatusActive, status)
}
