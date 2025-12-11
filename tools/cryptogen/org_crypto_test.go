/*
Copyright IBM Corp. All Rights Reserved.

SPDX-License-Identifier: Apache-2.0
*/

package cryptogen

import (
	"fmt"
	"os"
	"path"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/hyperledger/fabric-x-common/msp"
	"github.com/hyperledger/fabric-x-common/sampleconfig"
	"github.com/hyperledger/fabric-x-common/tools/test"
)

func TestGenerate(t *testing.T) { //nolint:gocognit // cognitive complexity 30.
	t.Parallel()
	for _, nodeOUs := range []bool{true, false} {
		t.Run(fmt.Sprintf("nodeOUs=%t", nodeOUs), func(t *testing.T) {
			t.Parallel()
			testDir := t.TempDir()
			err := Generate(testDir, defaultConfig(nodeOUs))
			actualTree := test.GetTree(t, testDir)
			t.Logf("Actual tree: %s", actualTree)
			require.NoError(t, err)

			dirs := []string{"ordererOrganizations", "peerOrganizations", "organizations"}
			test.RequireTree(t, testDir, nil, dirs)
			for _, dir := range dirs {
				orgDirs, err := os.ReadDir(filepath.Join(testDir, dir))
				require.NoError(t, err, "Actual Tree: %s", actualTree)

				for _, orgDir := range orgDirs {
					require.True(t, orgDir.IsDir(), "Only org dirs expected")
					orgPath := path.Join(testDir, dir, orgDir.Name())

					nodesDirs := []string{"users"}
					switch dir {
					case "ordererOrganizations":
						nodesDirs = append(nodesDirs, "orderers")
					default: // peer
						nodesDirs = append(nodesDirs, "peers")
					}

					expectedDirs := append([]string{"msp", "ca", "tlsca"}, nodesDirs...)
					test.RequireTree(t, orgPath, nil, expectedDirs)

					verifyingMsp, err := msp.LoadVerifyingMspDir(msp.DirLoadParameters{
						MspDir: path.Join(orgPath, "msp"),
					})
					require.NoError(t, err, "Failed to load MSP")
					require.NotNil(t, verifyingMsp, "MSP should not be nil")

					for _, nodeDirName := range nodesDirs {
						nodesPath := path.Join(orgPath, nodeDirName)
						nodes, err := os.ReadDir(nodesPath)
						require.NoError(t, err)

						for _, nodeDir := range nodes {
							nodePath := path.Join(nodesPath, nodeDir.Name())
							test.RequireTree(t, nodePath, nil, []string{"msp", "tls"})
							localMsp, err := msp.LoadLocalMspDir(msp.DirLoadParameters{
								MspDir: path.Join(nodePath, "msp"),
							})
							require.NoErrorf(t, err, "Failed to load MSP: %s", nodePath)
							require.NotNilf(t, localMsp, "MSP should not be nil: %s", nodePath)
						}
					}
				}
			}
		})
	}
}

func TestDefaultConfig(t *testing.T) {
	t.Parallel()
	expected := defaultConfig(false)
	actual, err := ParseConfig(sampleconfig.DefaultCryptoConfig)
	require.NoError(t, err)
	require.Equal(t, expected, actual)
}

func defaultConfig(enableNodeOUs bool) *Config {
	temp := NodeTemplate{
		Count:              1,
		Start:              1,
		Hostname:           "{{.Prefix}}-{{.Index}}.{{.Domain}}",
		SANS:               []string{"{{.Hostname}}.alt.{{.Domain}}"},
		PublicKeyAlgorithm: ECDSA,
	}

	return &Config{
		OrdererOrgs: []OrgSpec{
			{
				Name:          "SampleOrg",
				Domain:        "sample-org.com",
				EnableNodeOUs: enableNodeOUs,
				CA: NodeSpec{
					Hostname: "ca.sample-org.com", CommonName: "SampleOrgCA", PublicKeyAlgorithm: ECDSA,
				},
				Template: temp,
				Specs: []NodeSpec{{
					Hostname:           "orderer-2.sample-org.com",
					CommonName:         "orderer-2.sample-org.com",
					PublicKeyAlgorithm: ECDSA,
				}},
			}, {
				Name:          "Org1",
				Domain:        "ordering-org-1.com",
				EnableNodeOUs: enableNodeOUs,
				CA: NodeSpec{
					Hostname: "ca.ordering-org-1.com", CommonName: "Org1CA", PublicKeyAlgorithm: ECDSA,
				},
				Template: temp,
			}, {
				Name:          "Org2",
				Domain:        "ordering-org-21.com",
				EnableNodeOUs: enableNodeOUs,
				CA: NodeSpec{
					Hostname: "ca.ordering-org-2.com", CommonName: "Org2CA", PublicKeyAlgorithm: ECDSA,
				},
				Template: temp,
			},
		},
		PeerOrgs: []OrgSpec{
			{
				Name:          "PeerOrg1",
				Domain:        "peer-org-1.com",
				EnableNodeOUs: enableNodeOUs,
				CA: NodeSpec{
					Hostname: "ca.peer-org-1.com", CommonName: "PeerOrg1CA", PublicKeyAlgorithm: ECDSA,
				},
				Template: temp,
				Users:    UsersSpec{Count: 1, PublicKeyAlgorithm: "ecdsa"},
			}, {
				Name:          "PeerOrg2",
				Domain:        "peer-org-2.com",
				EnableNodeOUs: enableNodeOUs,
				CA: NodeSpec{
					Hostname: "ca.peer-org-2.com", CommonName: "PeerOrg2CA", PublicKeyAlgorithm: ECDSA,
				},
				Template: temp,
				Users:    UsersSpec{Count: 1, PublicKeyAlgorithm: "ecdsa", Specs: []UserSpec{{Name: "testuser"}}},
			},
		},
		GenericOrgs: []OrgSpec{
			{
				Name:          "JointOrg",
				Domain:        "joint-org.com",
				EnableNodeOUs: enableNodeOUs,
				CA: NodeSpec{
					Hostname: "ca.joint-org.com", CommonName: "JointOrgCA", PublicKeyAlgorithm: ECDSA,
				},
				Specs: []NodeSpec{
					{
						Hostname:           "router-1.joint-org.com",
						CommonName:         "router-1.joint-org.com",
						OrganizationalUnit: OrdererOU,
						PublicKeyAlgorithm: ECDSA,
						Party:              "party-1",
					},
					{
						Hostname:           "assembler-1.joint-org.com",
						CommonName:         "assembler-1.joint-org.com",
						OrganizationalUnit: OrdererOU,
						PublicKeyAlgorithm: ECDSA,
						Party:              "party-1",
					},
					{
						Hostname:           "router-2.joint-org.com",
						CommonName:         "router-2.joint-org.com",
						OrganizationalUnit: OrdererOU,
						PublicKeyAlgorithm: ECDSA,
						Party:              "party-2",
					},
					{
						Hostname:           "assembler-2.joint-org.com",
						CommonName:         "assembler-2.joint-org.com",
						OrganizationalUnit: OrdererOU,
						PublicKeyAlgorithm: ECDSA,
						Party:              "party-2",
					},
					{
						Hostname:           "endorser.joint-org.com",
						CommonName:         "endorser.joint-org.com",
						OrganizationalUnit: PeerOU,
						PublicKeyAlgorithm: ECDSA,
					},
					{
						Hostname:           "committer.joint-org.com",
						CommonName:         "committer.joint-org.com",
						OrganizationalUnit: PeerOU,
						PublicKeyAlgorithm: ECDSA,
					},
				},
				Users: UsersSpec{
					Specs: []UserSpec{{Name: "client", PublicKeyAlgorithm: ECDSA}},
				},
			},
		},
	}
}
