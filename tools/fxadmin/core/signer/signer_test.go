/*
Copyright IBM Corp. All Rights Reserved.

SPDX-License-Identifier: Apache-2.0
*/

package signer_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/hyperledger/fabric-x-common/tools/fxadmin/core/signer"
)

func TestNewRejectsMissingArguments(t *testing.T) {
	t.Parallel()

	for _, tc := range []struct {
		name   string
		mspID  string
		mspDir string
	}{
		{name: "empty MSP ID", mspID: "", mspDir: "/crypto/org1/msp"},
		{name: "empty MSP dir", mspID: "org1", mspDir: ""},
		{name: "both empty", mspID: "", mspDir: ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			_, err := signer.New(tc.mspID, tc.mspDir)
			require.ErrorContains(t, err, "MSP ID and directory are required")
		})
	}
}
