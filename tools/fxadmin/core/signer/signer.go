/*
Copyright IBM Corp. All Rights Reserved.

SPDX-License-Identifier: Apache-2.0
*/

// Package signer loads the admin signing identity from a local MSP directory.
// The identity signs the seek and broadcast envelopes sent to the orderer.
package signer

import (
	"github.com/cockroachdb/errors"

	"github.com/hyperledger/fabric-x-common/msp"
)

// New loads the local MSP at mspDir under the given MSP ID and returns its
// default signing identity.
//
//nolint:ireturn // msp.SigningIdentity is the interface the MSP loader returns.
func New(mspID, mspDir string) (msp.SigningIdentity, error) {
	if mspID == "" || mspDir == "" {
		return nil, errors.New("MSP ID and directory are required")
	}
	localMSP, err := msp.LoadLocalMspDir(msp.DirLoadParameters{
		MspName: mspID,
		MspDir:  mspDir,
	})
	if err != nil {
		return nil, errors.Wrap(err, "failed to load local MSP")
	}
	signer, err := localMSP.GetDefaultSigningIdentity()
	if err != nil {
		return nil, errors.Wrap(err, "failed to load default signing identity")
	}
	return signer, nil
}
