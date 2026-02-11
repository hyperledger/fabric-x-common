/*
Copyright IBM Corp. All Rights Reserved.

SPDX-License-Identifier: Apache-2.0
*/

package configtxgen

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/hyperledger/fabric-lib-go/bccsp/factory"
	"github.com/stretchr/testify/require"

	"github.com/hyperledger/fabric-x-common/api/types"
	"github.com/hyperledger/fabric-x-common/common/channelconfig"
	"github.com/hyperledger/fabric-x-common/core/config/configtest"
	"github.com/hyperledger/fabric-x-common/protoutil"
)

func TestInspectMissing(t *testing.T) {
	t.Parallel()
	require.EqualError(t, DoInspectBlock("NonSenseBlockFileThatDoesn'tActuallyExist"), "could not read block NonSenseBlockFileThatDoesn'tActuallyExist")
}

func TestInspectBlock(t *testing.T) {
	t.Parallel()
	blockDest := filepath.Join(t.TempDir(), "block")

	config := Load(SampleAppChannelInsecureSoloProfile, configtest.GetDevConfigDir())

	require.NoError(t, DoOutputBlock(config, "foo", blockDest), "Good block generation request")
	require.NoError(t, DoInspectBlock(blockDest), "Good block inspection request")
}

func TestInspectBlockErr(t *testing.T) {
	t.Parallel()
	config := Load(SampleAppChannelInsecureSoloProfile, configtest.GetDevConfigDir())

	require.EqualError(t, DoOutputBlock(config, "foo", ""), "error writing genesis block: open : no such file or directory")
	require.EqualError(t, DoInspectBlock(""), "could not read block ")
}

func TestMissingOrdererSection(t *testing.T) {
	t.Parallel()
	blockDest := filepath.Join(t.TempDir(), "block")

	config := Load(SampleAppChannelInsecureSoloProfile, configtest.GetDevConfigDir())
	config.Orderer = nil

	require.EqualError(t, DoOutputBlock(config, "foo", blockDest), "refusing to generate block which is missing orderer section")
}

func TestApplicationChannelGenesisBlock(t *testing.T) {
	t.Parallel()
	blockDest := filepath.Join(t.TempDir(), "block")

	config := Load(SampleAppChannelInsecureSoloProfile, configtest.GetDevConfigDir())

	require.NoError(t, DoOutputBlock(config, "foo", blockDest))
}

func TestApplicationChannelMissingApplicationSection(t *testing.T) {
	t.Parallel()
	blockDest := filepath.Join(t.TempDir(), "block")

	config := Load(SampleAppChannelInsecureSoloProfile, configtest.GetDevConfigDir())
	config.Application = nil

	require.EqualError(t, DoOutputBlock(config, "foo", blockDest), "refusing to generate application channel block which is missing application section")
}

func TestMissingConsortiumValue(t *testing.T) {
	t.Parallel()
	configTxDest := filepath.Join(t.TempDir(), "configtx")

	config := Load(SampleSingleMSPChannelProfile, configtest.GetDevConfigDir())
	config.Consortium = ""

	require.EqualError(t, DoOutputChannelCreateTx(config, nil, "foo", configTxDest), "config update generation failure: cannot define a new channel with no Consortium value")
}

func TestUnsuccessfulChannelTxFileCreation(t *testing.T) {
	t.Parallel()
	configTxDest := filepath.Join(t.TempDir(), "configtx")

	config := Load(SampleSingleMSPChannelProfile, configtest.GetDevConfigDir())
	require.NoError(t, os.WriteFile(configTxDest, []byte{}, 0o440))
	require.EqualError(t, DoOutputChannelCreateTx(config, nil, "foo", configTxDest), fmt.Sprintf("error writing channel create tx: open %s: permission denied", configTxDest))
}

func TestMissingApplicationValue(t *testing.T) {
	t.Parallel()
	configTxDest := filepath.Join(t.TempDir(), "configtx")

	config := Load(SampleSingleMSPChannelProfile, configtest.GetDevConfigDir())
	config.Application = nil

	require.EqualError(t, DoOutputChannelCreateTx(config, nil, "foo", configTxDest), "could not generate default config template: channel template configs must contain an application section")
}

func TestInspectMissingConfigTx(t *testing.T) {
	t.Parallel()
	require.EqualError(t, DoInspectChannelCreateTx("ChannelCreateTxFileWhichDoesn'tReallyExist"), "could not read channel create tx: open ChannelCreateTxFileWhichDoesn'tReallyExist: no such file or directory")
}

func TestInspectConfigTx(t *testing.T) {
	t.Parallel()
	configTxDest := filepath.Join(t.TempDir(), "configtx")

	config := Load(SampleSingleMSPChannelProfile, configtest.GetDevConfigDir())

	require.NoError(t, DoOutputChannelCreateTx(config, nil, "foo", configTxDest), "Good outputChannelCreateTx generation request")
	require.NoError(t, DoInspectChannelCreateTx(configTxDest), "Good configtx inspection request")
}

func TestPrintOrg(t *testing.T) {
	t.Parallel()
	require.NoError(t, factory.InitFactories(nil))
	config := LoadTopLevel(configtest.GetDevConfigDir())

	require.NoError(t, DoPrintOrg(config, SampleOrgName), "Good org to print")

	err := DoPrintOrg(config, SampleOrgName+".wrong")
	require.Error(t, err, "Bad org name")
	require.Regexp(t, "organization [^ ]* not found", err.Error())

	config.Organizations[0] = &Organization{Name: "FakeOrg", ID: "FakeOrg"}
	err = DoPrintOrg(config, "FakeOrg")
	require.Error(t, err, "Fake org")
	require.Regexp(t, "bad org definition", err.Error())
}

func createBftOrdererConfig() *Profile {
	// Load the BFT config from the sample, and use some TLS CA Cert as crypto material
	return Load(SampleAppChannelSmartBftProfile, configtest.GetDevConfigDir())
}

func TestBftOrdererTypeWithoutV3CapabilitiesShouldRaiseAnError(t *testing.T) {
	t.Parallel()
	// ### Arrange
	blockDest := filepath.Join(t.TempDir(), "block")
	config := createBftOrdererConfig()
	config.Capabilities["V3_0"] = false

	// ### Act & Assert
	require.EqualError(
		t,
		DoOutputBlock(config, "testChannelId", blockDest),
		"could not create bootstrapper: could not create channel group: "+
			"could not create orderer group: "+
			"orderer type BFT must be used with V3_0 channel capability: map[V3_0:false]",
	)
}

func TestBftOrdererTypeWithV3CapabilitiesShouldNotRaiseAnError(t *testing.T) {
	t.Parallel()
	// ### Arrange
	blockDest := filepath.Join(t.TempDir(), "block")
	config := createBftOrdererConfig()
	config.Capabilities["V3_0"] = true

	// ### Act & Assert
	require.NoError(t, DoOutputBlock(config, "testChannelId", blockDest))
}

func TestFabricXGenesisBlock(t *testing.T) {
	t.Parallel()

	allAPI := []string{types.Broadcast, types.Deliver}

	for _, tc := range []struct {
		sample            string
		expectedEndpoints []*types.OrdererEndpoint
	}{
		{
			sample: SampleFabricX,
			expectedEndpoints: []*types.OrdererEndpoint{
				{MspID: "SampleOrg", ID: 0, API: allAPI[:1], Host: "orderer-1", Port: 7050},
				{MspID: "SampleOrg", ID: 0, API: allAPI[1:], Host: "orderer-1", Port: 7060},
				{MspID: "SampleOrg", ID: 1, API: allAPI, Host: "orderer-2", Port: 7050},
				{MspID: "SampleOrg", ID: 2, API: nil, Host: "orderer-3", Port: 7050},
			},
		}, {
			sample: TwoOrgsSampleFabricX,
			expectedEndpoints: []*types.OrdererEndpoint{
				{MspID: "Org1", ID: 0, API: allAPI[:1], Host: "localhost", Port: 7050},
				{MspID: "Org1", ID: 0, API: allAPI[1:], Host: "localhost", Port: 7060},
				{MspID: "Org2", ID: 1, API: allAPI[:1], Host: "localhost", Port: 7051},
				{MspID: "Org2", ID: 1, API: allAPI[1:], Host: "localhost", Port: 7061},
			},
		},
	} {
		t.Run(tc.sample, func(t *testing.T) {
			t.Parallel()
			blockDest := filepath.Join(t.TempDir(), "block")
			config := Load(tc.sample, configtest.GetDevConfigDir())
			armaPath := filepath.Join(configtest.GetDevConfigDir(), "arma_shared_config.pbbin")
			config.Orderer.Arma.Path = armaPath
			require.NoError(t, DoOutputBlock(config, "foo", blockDest))

			configBlock, err := ReadBlock(blockDest)
			require.NoError(t, err)
			require.NotNil(t, configBlock)

			envelope, err := protoutil.ExtractEnvelope(configBlock, 0)
			require.NoError(t, err)
			require.NotNil(t, envelope)
			bundle, err := channelconfig.NewBundleFromEnvelope(envelope, factory.GetDefault())
			require.NoError(t, err)
			require.NotNil(t, bundle)

			oc, ok := bundle.OrdererConfig()
			require.True(t, ok)
			require.NotNil(t, oc)

			var endpoints []*types.OrdererEndpoint
			for orgID, org := range oc.Organizations() {
				endpointsStr := org.Endpoints()
				for _, eStr := range endpointsStr {
					t.Log(eStr)
					e, parseErr := types.ParseOrdererEndpoint(eStr)
					require.NoError(t, parseErr)
					e.MspID = orgID
					endpoints = append(endpoints, e)
				}
			}
			require.ElementsMatch(t, tc.expectedEndpoints, endpoints)
		})
	}
}
