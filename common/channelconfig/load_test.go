/*
Copyright IBM Corp. All Rights Reserved.

SPDX-License-Identifier: Apache-2.0
*/

package channelconfig_test

import (
	"fmt"
	"os"
	"path"
	"path/filepath"
	"testing"

	"github.com/hyperledger/fabric-protos-go-apiv2/common"
	"github.com/stretchr/testify/require"

	commontypes "github.com/hyperledger/fabric-x-common/api/types"
	"github.com/hyperledger/fabric-x-common/common/channelconfig"
	"github.com/hyperledger/fabric-x-common/protoutil"
	"github.com/hyperledger/fabric-x-common/tools/configtxgen"
	"github.com/hyperledger/fabric-x-common/tools/cryptogen"
)

func TestLoadConfigBlockFromFile(t *testing.T) {
	t.Parallel()

	for _, tc := range []struct {
		name                    string
		path                    string
		expectedChannelID       string
		expectedOrdererOrgs     int
		expectedApplicationOrgs int
	}{
		{
			name:                    "valid config block file",
			path:                    createConfigBlockPath(t, "channel-0", 2, 1),
			expectedChannelID:       "channel-0",
			expectedOrdererOrgs:     1,
			expectedApplicationOrgs: 2,
		},
		{
			name:                    "valid config block with single peer org",
			path:                    createConfigBlockPath(t, "channel-1", 1, 1),
			expectedChannelID:       "channel-1",
			expectedOrdererOrgs:     1,
			expectedApplicationOrgs: 1,
		},
		{
			name:                    "valid config block with multiple peer orgs and orderers",
			path:                    createConfigBlockPath(t, "channel-2", 3, 2),
			expectedChannelID:       "channel-2",
			expectedOrdererOrgs:     2,
			expectedApplicationOrgs: 3,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			retMaterial, retErr := channelconfig.LoadConfigBlockMaterialFromFile(tc.path)

			require.NoError(t, retErr)
			require.NotNil(t, retMaterial)
			require.Equal(t, tc.expectedChannelID, retMaterial.ChannelID)
			require.NotNil(t, retMaterial.ConfigBlock)
			require.NotNil(t, retMaterial.Bundle)
			require.Len(t, retMaterial.OrdererOrganizations, tc.expectedOrdererOrgs)
			require.Len(t, retMaterial.ApplicationOrganizations, tc.expectedApplicationOrgs)
			for _, org := range retMaterial.OrdererOrganizations {
				require.NotEmpty(t, org.Endpoints)
			}
		})
	}
}

func TestLoadConfigBlockFromFileEdgeCases(t *testing.T) {
	t.Parallel()

	for _, tc := range []struct {
		name          string
		blockPath     string
		expectedError string
	}{
		{
			name:          "empty path",
			blockPath:     "",
			expectedError: "config block path is empty",
		},
		{
			name:          "non-existent file",
			blockPath:     path.Join(t.TempDir(), "file.block"),
			expectedError: "could not read block",
		},
		{
			name:          "nil data",
			blockPath:     createBlockFile(t, &common.Block{}),
			expectedError: "the block is not a config block",
		},
		{
			name: "data block",
			blockPath: createBlockFile(t, &common.Block{
				Data: &common.BlockData{Data: [][]byte{[]byte("transaction data")}},
			}),
			expectedError: "the block is not a config block",
		},
		{
			name: "empty data",
			blockPath: createBlockFile(t, &common.Block{
				Data: &common.BlockData{Data: [][]byte{}},
			}),
			expectedError: "the block is not a config block",
		},
		{
			name: "multiple transactions",
			blockPath: createBlockFile(t, &common.Block{
				Data: &common.BlockData{
					Data: [][]byte{[]byte("transaction 1"), []byte("transaction 2")},
				},
			}),
			expectedError: "the block is not a config block",
		},
		{
			name: "invalid payload",
			blockPath: createBlockFile(t, &common.Block{
				Data: &common.BlockData{
					Data: [][]byte{
						protoutil.MarshalOrPanic(&common.Envelope{
							Payload: []byte("invalid payload"),
						}),
					},
				},
			}),
			expectedError: "the block is not a config block",
		},
		{
			name: "nil header",
			blockPath: createBlockFile(t, &common.Block{
				Data: &common.BlockData{
					Data: [][]byte{
						protoutil.MarshalOrPanic(&common.Envelope{
							Payload: protoutil.MarshalOrPanic(&common.Payload{Header: nil}),
						}),
					},
				},
			}),
			expectedError: "the block is not a config block",
		},
		{
			name: "wrong header type",
			blockPath: createBlockFile(t, &common.Block{
				Data: &common.BlockData{
					Data: [][]byte{
						protoutil.MarshalOrPanic(&common.Envelope{
							Payload: protoutil.MarshalOrPanic(&common.Payload{
								Header: &common.Header{
									ChannelHeader: protoutil.MarshalOrPanic(&common.ChannelHeader{
										Type:      int32(common.HeaderType_ENDORSER_TRANSACTION),
										ChannelId: "test-channel",
									}),
								},
							}),
						}),
					},
				},
			}),
			expectedError: "the block is not a config block",
		},
		{
			name: "invalid config envelope",
			blockPath: createBlockFile(t, &common.Block{
				Data: &common.BlockData{
					Data: [][]byte{
						protoutil.MarshalOrPanic(&common.Envelope{
							Payload: protoutil.MarshalOrPanic(&common.Payload{
								Header: &common.Header{
									ChannelHeader: protoutil.MarshalOrPanic(&common.ChannelHeader{
										Type:      int32(common.HeaderType_CONFIG),
										ChannelId: "test-channel",
									}),
								},
								Data: []byte("invalid config envelope"),
							}),
						}),
					},
				},
			}),
			expectedError: "error unmarshalling config envelope",
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			retMaterial, retErr := channelconfig.LoadConfigBlockMaterialFromFile(tc.blockPath)
			require.ErrorContains(t, retErr, tc.expectedError)
			require.Nil(t, retMaterial)
		})
	}
}

func TestOrganizationMaterialExtraction(t *testing.T) {
	t.Parallel()

	material := createConfigBlockMaterial(t, 3, 2)

	require.Len(t, material.OrdererOrganizations, 2)
	for _, ordererOrg := range material.OrdererOrganizations {
		require.NotEmpty(t, ordererOrg.MspID)
		require.NotEmpty(t, ordererOrg.CACerts)
		require.Len(t, ordererOrg.Endpoints, 1)
		for _, ep := range ordererOrg.Endpoints {
			require.NotEmpty(t, ep.Host)
			require.NotZero(t, ep.Port)
			require.Equal(t, ordererOrg.MspID, ep.MspID)
		}
	}

	require.Len(t, material.ApplicationOrganizations, 3)
	for _, peerOrg := range material.ApplicationOrganizations {
		require.NotEmpty(t, peerOrg.MspID)
		require.NotEmpty(t, peerOrg.CACerts)
	}
}

// Helper methods

func createConfigBlockPath(
	t *testing.T,
	channelID string,
	peerOrgCount uint32,
	ordererOrgCount uint32,
) string {
	t.Helper()
	cryptoDir := t.TempDir()
	orgs := make([]cryptogen.OrganizationParameters, 0, int(peerOrgCount)+int(ordererOrgCount))
	for i := range peerOrgCount {
		orgs = append(orgs, cryptogen.OrganizationParameters{
			Name:      fmt.Sprintf("peer-org-%d", i),
			Domain:    fmt.Sprintf("peer-org-%d.com", i),
			PeerNodes: []cryptogen.Node{{CommonName: "peer-node", Hostname: "peer-node"}},
		})
	}
	for i := range ordererOrgCount {
		domain := fmt.Sprintf("orderer-org-%d.com", i)
		orgs = append(orgs, cryptogen.OrganizationParameters{
			Name:             fmt.Sprintf("orderer-org-%d", i),
			Domain:           domain,
			OrdererEndpoints: []*commontypes.OrdererEndpoint{{ID: i, Host: domain, Port: 7050}},
			ConsenterNodes:   []cryptogen.Node{{CommonName: "consenter", Hostname: "consenter"}},
			OrdererNodes:     []cryptogen.Node{{CommonName: "orderer-node", Hostname: "orderer-node"}},
		})
	}
	_, err := cryptogen.CreateOrExtendConfigBlockWithCrypto(cryptogen.ConfigBlockParameters{
		TargetPath:    cryptoDir,
		BaseProfile:   configtxgen.SampleFabricX,
		ChannelID:     channelID,
		Organizations: orgs,
	})
	require.NoError(t, err)
	return path.Join(cryptoDir, cryptogen.ConfigBlockFileName)
}

func createConfigBlockMaterial(
	t *testing.T,
	peerOrgCount uint32,
	ordererOrgCount uint32,
) *channelconfig.ConfigBlockMaterial {
	t.Helper()
	blockPath := createConfigBlockPath(t, "test-channel", peerOrgCount, ordererOrgCount)
	material, err := channelconfig.LoadConfigBlockMaterialFromFile(blockPath)
	require.NoError(t, err)
	return material
}

func createBlockFile(t *testing.T, block *common.Block) string {
	t.Helper()
	blockPath := filepath.Join(t.TempDir(), "block.block")
	blockBytes, err := protoutil.Marshal(block)
	require.NoError(t, err)
	err = os.WriteFile(blockPath, blockBytes, 0o600)
	require.NoError(t, err)
	return blockPath
}
