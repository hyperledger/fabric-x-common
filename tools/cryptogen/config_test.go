/*
Copyright IBM Corp. All Rights Reserved.

SPDX-License-Identifier: Apache-2.0
*/

package cryptogen

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/hyperledger/fabric-x-common/sampleconfig"
)

func TestDefaultConfigParsing(t *testing.T) {
	t.Parallel()
	config, err := ParseConfig(sampleconfig.DefaultCryptoConfig)
	require.NoError(t, err)
	require.NotNil(t, config)
}
