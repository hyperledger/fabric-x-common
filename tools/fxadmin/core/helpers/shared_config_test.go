/*
Copyright IBM Corp. All Rights Reserved.

SPDX-License-Identifier: Apache-2.0
*/

package helpers_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/hyperledger/fabric-x-common/api/ordererpb"
	"github.com/hyperledger/fabric-x-common/protoutil"
	"github.com/hyperledger/fabric-x-common/tools/configtxgen"
	"github.com/hyperledger/fabric-x-common/tools/fxadmin/core/helpers"
	"github.com/hyperledger/fabric-x-common/utils/test"
)

// TestUnmarshalArmaSharedConfig covers the decoding of the ARMA shared config
// from an orderer's consensus metadata, and the rejection of another consensus
// type and of metadata that is not a shared config.
func TestUnmarshalArmaSharedConfig(t *testing.T) {
	t.Parallel()

	sharedConfig := &ordererpb.SharedConfig{
		PartiesConfig: []*ordererpb.PartyConfig{{PartyID: 1}},
		MaxPartyID:    1,
	}
	metadata := protoutil.MarshalOrPanic(sharedConfig)

	for _, tc := range []struct {
		name          string
		consensusType string
		metadata      []byte
		wantErr       string
	}{
		{
			name:          "ARMA",
			consensusType: configtxgen.Arma,
			metadata:      metadata,
		},
		{
			name:          "another consensus type",
			consensusType: "etcdraft",
			metadata:      metadata,
			wantErr:       `unsupported consensus type "etcdraft": expected "arma"`,
		},
		{
			name:          "metadata that is not a shared config",
			consensusType: configtxgen.Arma,
			metadata:      []byte("not a shared config"),
			wantErr:       "failed to unmarshal ARMA shared config from consensus metadata",
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got, err := helpers.UnmarshalArmaSharedConfig(tc.consensusType, tc.metadata)
			if tc.wantErr != "" {
				require.ErrorContains(t, err, tc.wantErr)
				return
			}
			require.NoError(t, err)
			test.RequireProtoEqual(t, sharedConfig, got)
		})
	}
}
