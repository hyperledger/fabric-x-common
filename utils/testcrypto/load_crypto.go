/*
Copyright IBM Corp. All Rights Reserved.

SPDX-License-Identifier: Apache-2.0
*/

package testcrypto

import (
	"os"
	"path"
	"strings"

	"github.com/cockroachdb/errors"

	"github.com/hyperledger/fabric-x-common/msp"
	"github.com/hyperledger/fabric-x-common/tools/cryptogen"
)

// GetPeersIdentities returns the peers' identities from a crypto path.
func GetPeersIdentities(artifactsPath string) ([]msp.SigningIdentity, error) {
	return GetSigningIdentities(GetPeersMspDirs(artifactsPath)...)
}

// GetConsenterIdentities returns the orderer consenters identities from a crypto path.
func GetConsenterIdentities(artifactsPath string) ([]msp.SigningIdentity, error) {
	return GetSigningIdentities(GetConsenterMspDirs(artifactsPath)...)
}

// GetSigningIdentities loads signing identities from the given MSP directories.
func GetSigningIdentities(mspDirs ...*msp.DirLoadParameters) ([]msp.SigningIdentity, error) {
	identities := make([]msp.SigningIdentity, len(mspDirs))
	for i, mspDir := range mspDirs {
		localMsp, err := msp.LoadLocalMspDir(*mspDir)
		if err != nil {
			return nil, err
		}
		identities[i], err = localMsp.GetDefaultSigningIdentity()
		if err != nil {
			return nil, errors.Wrap(err, "loading signing identity")
		}
	}
	return identities, nil
}

// GetPeersMspDirs returns the peers' MSP directory path.
// It discovers the client user directory by scanning the users/ directory
// for an entry matching "client@*", rather than assuming a specific domain suffix.
func GetPeersMspDirs(artifactsPath string) []*msp.DirLoadParameters {
	peerOrgPath := path.Join(artifactsPath, cryptogen.PeerOrganizationsDir)
	return getNodeMspDirs(peerOrgPath, "users", "client@")
}

// GetConsenterMspDirs returns the orderers' MSP directory path.
func GetConsenterMspDirs(artifactsPath string) []*msp.DirLoadParameters {
	ordererOrgPath := path.Join(artifactsPath, cryptogen.OrdererOrganizationsDir)
	return getNodeMspDirs(ordererOrgPath, "orderers", "consenter")
}

// getNodeMspDirs returns the MSP directory for the given node type.
func getNodeMspDirs(orgsPath, nodeSubDir, nodePrefix string) []*msp.DirLoadParameters {
	orgMspDirs := getMspDirs(orgsPath)
	nodeMspDirs := make([]*msp.DirLoadParameters, 0, len(orgMspDirs))
	for _, mspItem := range orgMspDirs {
		subDir := path.Join(mspItem.MspDir, nodeSubDir)
		entries, _ := os.ReadDir(subDir)
		for _, entry := range entries {
			if !entry.IsDir() || !strings.HasPrefix(entry.Name(), nodePrefix) {
				continue
			}
			nodeMspDirs = append(nodeMspDirs, &msp.DirLoadParameters{
				MspName: mspItem.MspName,
				MspDir:  path.Join(subDir, entry.Name(), "msp"),
			})
			break
		}
	}
	return nodeMspDirs
}

// getMspDirs returns the MSP dir parameter per organization in the path.
func getMspDirs(orgsPath string) []*msp.DirLoadParameters {
	dir, err := os.ReadDir(orgsPath)
	if err != nil {
		return nil
	}
	mspDirs := make([]*msp.DirLoadParameters, 0, len(dir))
	for _, dirEntry := range dir {
		if !dirEntry.IsDir() {
			continue
		}
		dirName := dirEntry.Name()
		orgName := dirName
		if strings.HasSuffix(dirName, ".com") {
			orgName = dirName[:len(dirName)-4]
		}
		mspDirs = append(mspDirs, &msp.DirLoadParameters{
			MspName: orgName,
			MspDir:  path.Join(orgsPath, dirName),
		})
	}
	return mspDirs
}
