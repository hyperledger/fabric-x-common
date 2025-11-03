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
)

func TestGenerate(t *testing.T) { //nolint:gocognit // cognitive complexity 30.
	t.Parallel()
	for _, nodeOUs := range []bool{true, false} {
		t.Run(fmt.Sprintf("nodeOUs=%t", nodeOUs), func(t *testing.T) {
			t.Parallel()
			testDir := t.TempDir()
			err := Generate(testDir, defaultConfig(nodeOUs))
			actualTree := getActualTree(t, testDir)
			t.Logf("Actual tree: %s", actualTree)
			require.NoError(t, err)

			dirs := []string{"ordererOrganizations", "peerOrganizations"}
			requireTree(t, testDir, nil, dirs)
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
					requireTree(t, orgPath, nil, expectedDirs)

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
							requireTree(t, nodePath, nil, []string{"msp", "tls"})
							localMsp, err := msp.LoadLocalMspDir(msp.DirLoadParameters{
								MspDir: path.Join(nodePath, "msp"),
							})
							require.NoError(t, err, "Failed to load MSP")
							require.NotNil(t, localMsp, "MSP should not be nil")
						}
					}
				}
			}
		})
	}
}

func TestDefaultConfig(t *testing.T) {
	t.Parallel()
	expected := defaultConfig(true)
	actual, err := ParseConfig(DefaultConfig)
	require.NoError(t, err)
	require.Equal(t, expected, actual)
}

func defaultConfig(nodeOUs bool) *Config {
	ca := NodeSpec{
		CommonName:         "my-ca",
		Hostname:           "my-ca.com",
		PublicKeyAlgorithm: ECDSA,
	}
	temp := NodeTemplate{
		Count:              1,
		Start:              2,
		Hostname:           "{{.Prefix}}{{.Index}}",
		SANS:               []string{"{{.Hostname}}.alt.{{.Domain}}"},
		PublicKeyAlgorithm: ECDSA,
	}
	return &Config{
		OrdererOrgs: []OrgSpec{
			{
				Name:          "ordering-org-1",
				Domain:        "ordering-org-1.com",
				EnableNodeOUs: nodeOUs,
				CA:            ca,
				Template:      temp,
				Specs: []NodeSpec{
					{
						CommonName:         "orderer-1",
						Hostname:           "orderer-1.com",
						PublicKeyAlgorithm: ECDSA,
					},
				},
			},
		},
		PeerOrgs: []OrgSpec{
			{
				Name:          "peer-org-1",
				Domain:        "peer-org-1.com",
				EnableNodeOUs: nodeOUs,
				CA:            ca,
				Template:      temp,
				Specs: []NodeSpec{
					{
						CommonName:         "peer-1",
						Hostname:           "peer-1.com",
						PublicKeyAlgorithm: ECDSA,
					},
				},
				Users: UsersSpec{
					Count:              1,
					PublicKeyAlgorithm: ECDSA,
				},
			},
			{
				Name:          "peer-org-2",
				Domain:        "peer-org-2.com",
				EnableNodeOUs: nodeOUs,
				CA:            ca,
				Template: NodeTemplate{
					Count:              1,
					PublicKeyAlgorithm: ECDSA,
				},
				Specs: []NodeSpec{
					{
						CommonName:         "peer-2",
						Hostname:           "peer-2.com",
						PublicKeyAlgorithm: ECDSA,
					},
				},
				Users: UsersSpec{
					Count:              1,
					PublicKeyAlgorithm: ECDSA,
					Specs:              []UserSpec{{Name: "testuser"}},
				},
			},
		},
	}
}
