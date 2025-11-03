/*
Copyright IBM Corp. All Rights Reserved.

SPDX-License-Identifier: Apache-2.0
*/

package cryptogen

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v2"

	"github.com/hyperledger/fabric-x-common/msp"
)

const (
	mspTestCAName = "ca.example.com"
	mspTestName   = "peer0"
	mspFailedName = "test/fail"
)

func TestGenerateLocalMSP(t *testing.T) {
	t.Parallel()
	for _, nodeOUs := range []bool{true, false} {
		t.Run(
			fmt.Sprintf("nodeOUs=%t", nodeOUs), func(t *testing.T) {
				t.Parallel()
				expectedFiles := func(tlsPrefix string) []string {
					ret := []string{
						filepath.Join("msp", "cacerts", mspTestCAName+"-cert.pem"),
						filepath.Join("msp", "tlscacerts", mspTestCAName+"-cert.pem"),
						filepath.Join("msp", "signcerts", mspTestName+"-cert.pem"),
						filepath.Join("tls", "ca.crt"),
						filepath.Join("tls", tlsPrefix+".key"),
						filepath.Join("tls", tlsPrefix+".crt"),
					}
					if nodeOUs {
						ret = append(ret, filepath.Join("msp", "config.yaml"))
					} else {
						ret = append(ret, filepath.Join("msp", "admincerts", mspTestName+"-cert.pem"))
					}
					return ret
				}
				expectedDirs := []string{
					filepath.Join("msp", "keystore"),
				}

				t.Run(
					"valid server", func(t *testing.T) {
						t.Parallel()
						testDir := t.TempDir()
						// generate local MSP for nodeType=NodeTypePeer
						tree := NewMspTree(testDir)

						err := tree.GenerateLocalMSP(newMSPParameters(t, testDir, NodeTypePeer, nodeOUs))
						require.NoError(t, err, "Failed to generate local MSP. Tree")
						requireTree(t, testDir, expectedFiles("server"), expectedDirs)
						localMsp, err := msp.LoadLocalMspDir(msp.DirLoadParameters{MspDir: tree.MSP})
						require.NoError(t, err, "Failed to load MSP")
						require.NotNil(t, localMsp, "MSP should not be nil")
					},
				)

				t.Run(
					"valid client", func(t *testing.T) {
						t.Parallel()
						testDir := t.TempDir()
						// generate local MSP for nodeType=NodeTypeClient
						tree := NewMspTree(testDir)
						err := tree.GenerateLocalMSP(newMSPParameters(t, testDir, NodeTypeClient, nodeOUs))
						require.NoError(t, err, "Failed to generate local MSP")
						requireTree(t, testDir, expectedFiles("client"), expectedDirs)
						localMsp, err := msp.LoadLocalMspDir(msp.DirLoadParameters{MspDir: tree.MSP})
						require.NoError(t, err, "Failed to load MSP")
						require.NotNil(t, localMsp, "MSP should not be nil")
					},
				)

				t.Run(
					"bad TLS CA name", func(t *testing.T) {
						t.Parallel()
						testDir := t.TempDir()
						p := newMSPParameters(t, testDir, NodeTypeClient, nodeOUs)
						p.TLSCa.Name = mspFailedName
						tree := NewMspTree(testDir)
						err := tree.GenerateLocalMSP(p)
						require.Error(t, err, "Should have failed with CA name 'test/fail'")
					},
				)

				t.Run(
					"bad sign CA name", func(t *testing.T) {
						t.Parallel()
						testDir := t.TempDir()
						p := newMSPParameters(t, testDir, NodeTypeOrderer, nodeOUs)
						p.SignCa.Name = mspFailedName
						tree := NewMspTree(testDir)
						err := tree.GenerateLocalMSP(p)
						require.Error(t, err, "Should have failed with CA name 'test/fail'")
					},
				)
			},
		)
	}
}

func TestGenerateVerifyingMSP(t *testing.T) {
	t.Parallel()
	for _, nodeOUs := range []bool{true, false} {
		t.Run(
			fmt.Sprintf("nodeOUs=%t", nodeOUs), func(t *testing.T) {
				t.Run(
					"valid", func(t *testing.T) {
						t.Parallel()
						testDir := t.TempDir()
						tree := NewMspTree(testDir)
						newMSPParameters(t, testDir, NodeTypeAdmin, nodeOUs)
						err := tree.GenerateVerifyingMSP(newMSPParameters(t, testDir, NodeTypeAdmin, nodeOUs))
						require.NoError(t, err, "Failed to generate verifying MSP")

						// check to see that the right files were generated/saved
						expectedFiles := []string{
							filepath.Join("msp", "cacerts", mspTestCAName+"-cert.pem"),
							filepath.Join("msp", "tlscacerts", mspTestCAName+"-cert.pem"),
						}
						if nodeOUs {
							expectedFiles = append(expectedFiles, filepath.Join("msp", "config.yaml"))
						} else {
							expectedFiles = append(
								expectedFiles, filepath.Join("msp", "admincerts", mspTestCAName+"-cert.pem"),
							)
						}
						requireTree(t, testDir, expectedFiles, nil)
						verifyingMsp, err := msp.LoadVerifyingMspDir(msp.DirLoadParameters{MspDir: tree.MSP})
						require.NoError(t, err, "Failed to load MSP")
						require.NotNil(t, verifyingMsp, "MSP should not be nil")
					},
				)

				t.Run(
					"bad CA name", func(t *testing.T) {
						t.Parallel()
						testDir := t.TempDir()
						p := newMSPParameters(t, testDir, NodeTypeAdmin, nodeOUs)
						p.TLSCa.Name = mspFailedName
						tree := NewMspTree(testDir)
						err := tree.GenerateVerifyingMSP(p)
						require.Error(t, err, "Should have failed with CA name 'test/fail'")
					},
				)

				t.Run(
					"bad sign CA name", func(t *testing.T) {
						t.Parallel()
						testDir := t.TempDir()
						p := newMSPParameters(t, testDir, NodeTypeAdmin, nodeOUs)
						p.SignCa.Name = mspFailedName
						tree := NewMspTree(testDir)
						err := tree.GenerateVerifyingMSP(p)
						require.Error(t, err, "Should have failed with CA name 'test/fail'")
					},
				)
			},
		)
	}
}

func TestExportConfig(t *testing.T) {
	t.Parallel()
	testDir := t.TempDir()
	configFile := filepath.Join(testDir, "config.yaml")
	caFile := "ca.pem"

	err := exportConfig(testDir, caFile, true)
	require.NoError(t, err)

	configBytes, err := os.ReadFile(configFile)
	require.NoError(t, err, "failed to read config file")

	config := &msp.Configuration{}
	err = yaml.Unmarshal(configBytes, config)
	require.NoError(t, err, "ailed to unmarshal config")
	require.True(t, config.NodeOUs.Enable)
	require.Equal(t, caFile, config.NodeOUs.ClientOUIdentifier.Certificate)
	require.Equal(t, clientOU, config.NodeOUs.ClientOUIdentifier.OrganizationalUnitIdentifier)
	require.Equal(t, caFile, config.NodeOUs.PeerOUIdentifier.Certificate)
	require.Equal(t, peerOU, config.NodeOUs.PeerOUIdentifier.OrganizationalUnitIdentifier)
	require.Equal(t, caFile, config.NodeOUs.AdminOUIdentifier.Certificate)
	require.Equal(t, adminOU, config.NodeOUs.AdminOUIdentifier.OrganizationalUnitIdentifier)
	require.Equal(t, caFile, config.NodeOUs.OrdererOUIdentifier.Certificate)
	require.Equal(t, ordererOU, config.NodeOUs.OrdererOUIdentifier.OrganizationalUnitIdentifier)
}

func newMSPParameters(t *testing.T, rootDir string, nodeType int, nodeOUs bool) NodeParameters {
	t.Helper()
	signCA := defaultCA(t, mspTestCAName, path.Join(rootDir, "ca"))
	tlsCA := defaultCA(t, mspTestCAName, path.Join(rootDir, "tlsca"))
	return NodeParameters{
		Name:      mspTestName,
		Type:      nodeType,
		KeyAlg:    ECDSA,
		SignCa:    signCA,
		TLSCa:     tlsCA,
		EnableOUs: nodeOUs,
	}
}

func requireTree(t *testing.T, root string, expectedFiles, expectedDirs []string) {
	t.Helper()
	actualTree := getActualTree(t, root)

	for _, file := range expectedFiles {
		require.FileExistsf(t, path.Join(root, file), "File '%s' Does not exist. Tree: %s", file, actualTree)
	}
	for _, dir := range expectedDirs {
		require.DirExistsf(t, path.Join(root, dir), "Directory '%s' Does not exist. Tree: %s", dir, actualTree)
	}
}

func getActualTree(t *testing.T, root string) string {
	t.Helper()
	var stdout bytes.Buffer
	cmd := exec.Command("tree")
	cmd.Dir = root
	cmd.Stdout = &stdout
	require.NoError(t, cmd.Run(), "Error running tree")
	return stdout.String()
}
