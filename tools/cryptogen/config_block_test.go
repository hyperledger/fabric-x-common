/*
Copyright IBM Corp. All Rights Reserved.

SPDX-License-Identifier: Apache-2.0
*/

package cryptogen

import (
	"crypto/x509"
	"encoding/pem"
	"path"
	"path/filepath"
	"testing"

	"github.com/hyperledger/fabric-lib-go/bccsp/factory"
	"github.com/stretchr/testify/require"

	"github.com/hyperledger/fabric-x-common/api/types"
	"github.com/hyperledger/fabric-x-common/common/channelconfig"
	"github.com/hyperledger/fabric-x-common/msp"
	"github.com/hyperledger/fabric-x-common/protoutil"
	"github.com/hyperledger/fabric-x-common/tools/test"
)

func TestMakeConfig(t *testing.T) {
	t.Parallel()
	target := t.TempDir()

	armaData := []byte("fake-arma-data")
	chanName := "my-chan"

	key, err := generatePrivateKey(target, ECDSA)
	require.NoError(t, err)
	certBytes, err := x509.MarshalPKIXPublicKey(getPublicKey(key))
	require.NoError(t, err)
	metaKeyBytes := pem.EncodeToMemory(&pem.Block{Type: CertType, Bytes: certBytes})

	block, err := CreateDefaultConfigBlockWithCrypto(ConfigBlockParameters{
		TargetPath: target,
		ChannelID:  chanName,
		Organizations: []OrganizationParameters{
			{ // Joint org with two ordering parties.
				Name:   "org-1",
				Domain: "org-1.com",
				OrdererEndpoints: []OrdererEndpoint{
					{Address: "localhost:6001", API: []string{types.Broadcast}},
					{Address: "localhost:7001", API: []string{types.Deliver}},
				},
				ConsenterNodes: []Node{
					{Party: "party-1", CommonName: "consenter-1", Hostname: "localhost"},
					{Party: "party-2", CommonName: "consenter-2", Hostname: "localhost"},
				},
				OrdererNodes: []Node{
					{Party: "party-1", CommonName: "router-1", Hostname: "localhost"},
					{Party: "party-1", CommonName: "assembler-1", Hostname: "localhost"},
					{Party: "party-1", CommonName: "batcher-1", Hostname: "localhost"},
					{Party: "party-2", CommonName: "router-2", Hostname: "localhost"},
					{Party: "party-2", CommonName: "assembler-2", Hostname: "localhost"},
					{Party: "party-2", CommonName: "batcher-2", Hostname: "localhost"},
				},
				PeerNodes: []Node{
					{CommonName: "committer", Hostname: "localhost"},
					{CommonName: "coordinator", Hostname: "localhost"},
					{CommonName: "verifier", Hostname: "localhost"},
					{CommonName: "vc", Hostname: "localhost"},
					{CommonName: "query", Hostname: "localhost"},
					{CommonName: "endorser", Hostname: "localhost"},
				},
			},
			{ // Ordering org with a single party.
				Name:   "org-2",
				Domain: "org-2.com",
				OrdererEndpoints: []OrdererEndpoint{
					{Address: "localhost:6002", API: []string{types.Broadcast}},
					{Address: "localhost:7002", API: []string{types.Deliver}},
				},
				ConsenterNodes: []Node{
					{CommonName: "consenter", Hostname: "localhost"},
				},
				OrdererNodes: []Node{
					{CommonName: "router", Hostname: "localhost"},
					{CommonName: "assembler", Hostname: "localhost"},
					{CommonName: "batcher", Hostname: "localhost"},
				},
			},
			{ // Peer org.
				Name:   "org-3",
				Domain: "org-3.com",
				PeerNodes: []Node{
					{CommonName: "committer", Hostname: "localhost"},
					{CommonName: "coordinator", Hostname: "localhost"},
					{CommonName: "verifier", Hostname: "localhost"},
					{CommonName: "vc", Hostname: "localhost"},
					{CommonName: "query", Hostname: "localhost"},
					{CommonName: "endorser", Hostname: "localhost"},
				},
			},
		},
		ArmaMetaBytes:                armaData,
		MetaNamespaceVerificationKey: metaKeyBytes,
	})
	require.NoError(t, err)

	t.Log(test.GetTree(t, target))

	var expectedDirs []string //nolint:prealloc // Hard to estimate size.

	org1Dir := filepath.Join(GenericOrganizationsDir, "org-1")
	org2Dir := filepath.Join(OrdererOrganizationsDir, "org-2")
	org3Dir := filepath.Join(PeerOrganizationsDir, "org-3")
	// Add all users.
	for _, orgDir := range []string{org1Dir, org2Dir, org3Dir} {
		for _, n := range []string{"client", "Admin"} {
			expectedDirs = append(expectedDirs, filepath.Join(orgDir, "users", n+"@"+path.Base(orgDir)+".com", "msp"))
		}
	}
	// Add all committer nodes.
	for _, orgDir := range []string{org1Dir, org3Dir} {
		for _, n := range []string{"committer", "coordinator", "vc", "query", "endorser"} {
			expectedDirs = append(expectedDirs, filepath.Join(orgDir, "peers", n, "msp"))
		}
	}
	// Add all org-1 orderers.
	for _, n := range []string{
		"party-1/router-1", "party-2/router-2",
		"party-1/assembler-1", "party-2/assembler-2",
		"party-1/batcher-1", "party-2/batcher-2",
		"party-1/consenter-1", "party-2/consenter-2",
	} {
		expectedDirs = append(expectedDirs, filepath.Join(org1Dir, "orderers", n, "msp"))
	}
	// Add all org-2 orderers.
	for _, n := range []string{"router", "assembler", "batcher", "consenter"} {
		expectedDirs = append(expectedDirs, filepath.Join(org2Dir, "orderers", n, "msp"))
	}

	test.RequireTree(t, target, []string{"config-block.pb.bin"}, expectedDirs)

	require.NotNil(t, block)
	require.NotNil(t, block.Data)
	require.NotEmpty(t, block.Data.Data)
	envelope, err := protoutil.ExtractEnvelope(block, 0)
	require.NoError(t, err)

	bundle, err := channelconfig.NewBundleFromEnvelope(envelope, factory.GetDefault())
	require.NoError(t, err)
	oc, ok := bundle.OrdererConfig()
	require.True(t, ok)

	orgMap := oc.Organizations()
	require.Len(t, orgMap, 2)

	var endpoints []*types.OrdererEndpoint
	for orgID, org := range orgMap {
		require.Equal(t, orgID, org.MSPID())
		require.Equal(t, orgID, org.Name())
		endpointsStr := org.Endpoints()
		for _, eStr := range endpointsStr {
			e, parseErr := types.ParseOrdererEndpoint(eStr)
			require.NoError(t, parseErr)
			e.MspID = orgID
			endpoints = append(endpoints, e)
		}
	}
	require.Len(t, endpoints, 4)
	require.ElementsMatch(t, endpoints, []*types.OrdererEndpoint{
		{
			Host:  "localhost",
			Port:  6001,
			ID:    0,
			MspID: "org-1",
			API:   []string{types.Broadcast},
		},
		{
			Host:  "localhost",
			Port:  7001,
			ID:    0,
			MspID: "org-1",
			API:   []string{types.Deliver},
		},
		{
			Host:  "localhost",
			Port:  6002,
			ID:    1,
			MspID: "org-2",
			API:   []string{types.Broadcast},
		},
		{
			Host:  "localhost",
			Port:  7002,
			ID:    1,
			MspID: "org-2",
			API:   []string{types.Deliver},
		},
	})

	require.Equal(t, armaData, oc.ConsensusMetadata())

	requireSign(t, bundle, "Admins", msp.DirLoadParameters{
		MspName: "org-1",
		MspDir:  path.Join(target, org1Dir, UsersDir, "Admin@org-1.com", MSPDir),
	}, msp.DirLoadParameters{
		MspName: "org-2",
		MspDir:  path.Join(target, org2Dir, UsersDir, "Admin@org-2.com", MSPDir),
	}, msp.DirLoadParameters{
		MspName: "org-3",
		MspDir:  path.Join(target, org3Dir, UsersDir, "Admin@org-3.com", MSPDir),
	})
	requireSign(t, bundle, "Writers", msp.DirLoadParameters{
		MspName: "org-1",
		MspDir:  path.Join(target, org1Dir, UsersDir, "client@org-1.com", MSPDir),
	}, msp.DirLoadParameters{
		MspName: "org-2",
		MspDir:  path.Join(target, org2Dir, UsersDir, "client@org-2.com", MSPDir),
	}, msp.DirLoadParameters{
		MspName: "org-3",
		MspDir:  path.Join(target, org3Dir, UsersDir, "client@org-3.com", MSPDir),
	})
	requireSign(t, bundle, "Application/Endorsement", msp.DirLoadParameters{
		MspName: "org-1",
		MspDir:  path.Join(target, org1Dir, PeerNodesDir, "endorser", MSPDir),
	}, msp.DirLoadParameters{
		MspName: "org-3",
		MspDir:  path.Join(target, org3Dir, PeerNodesDir, "endorser", MSPDir),
	})
	requireSign(t, bundle, "Orderer/BlockValidation", msp.DirLoadParameters{
		MspName: "org-1",
		MspDir:  path.Join(target, org1Dir, OrdererNodesDir, "party-1", "consenter-1", MSPDir),
	}, msp.DirLoadParameters{
		MspName: "org-2",
		MspDir:  path.Join(target, org2Dir, OrdererNodesDir, "consenter", MSPDir),
	})
	requireSign(t, bundle, "Orderer/BlockValidation", msp.DirLoadParameters{
		MspName: "org-1",
		MspDir:  path.Join(target, org1Dir, OrdererNodesDir, "party-2", "consenter-2", MSPDir),
	}, msp.DirLoadParameters{
		MspName: "org-2",
		MspDir:  path.Join(target, org2Dir, OrdererNodesDir, "consenter", MSPDir),
	})
}

func requireSign(t *testing.T, bundle *channelconfig.Bundle, policyName string, users ...msp.DirLoadParameters) {
	t.Helper()
	policy, ok := bundle.PolicyManager().GetPolicy(policyName)
	require.Truef(t, ok, "policy %s not found", policyName)
	require.NotNil(t, policy)

	data := []byte("data")
	signedData := make([]*protoutil.SignedData, len(users))
	for i, u := range users {
		mspUser, err := msp.LoadLocalMspDir(u)
		require.NoError(t, err)
		require.NotNil(t, mspUser)

		si, err := mspUser.GetDefaultSigningIdentity()
		require.NoError(t, err)
		siID, err := si.Serialize()
		require.NoError(t, err)
		sig, err := si.Sign(data)
		require.NoError(t, err)
		signedData[i] = &protoutil.SignedData{
			Data:      data,
			Identity:  siID,
			Signature: sig,
		}
	}

	err := policy.EvaluateSignedData(signedData)
	require.NoError(t, err)
}
