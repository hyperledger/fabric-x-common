/*
Copyright IBM Corp. All Rights Reserved.

SPDX-License-Identifier: Apache-2.0
*/

package viperutil

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestGenerateMockPublicPrivateKeyPairPEM(t *testing.T) {
	_, _, err := generateMockPublicPrivateKeyPairPEM(false)
	require.NoError(t, err, "Unable to generate a public/private key pair: %v", err)
}

func TestGenerateMockPublicPrivateKeyPairPEMWhenCASet(t *testing.T) {
	_, _, err := generateMockPublicPrivateKeyPairPEM(true)
	require.NoError(t, err, "Unable to generate a signer certificate: %v", err)
}
