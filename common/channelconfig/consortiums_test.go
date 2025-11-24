/*
Copyright IBM Corp. All Rights Reserved.

SPDX-License-Identifier: Apache-2.0
*/
package channelconfig

import (
	"testing"

	cb "github.com/hyperledger/fabric-x-common/api/protocommon"
	"github.com/stretchr/testify/require"
)

func TestConsortiums(t *testing.T) {
	_, err := NewConsortiumsConfig(&cb.ConfigGroup{}, nil)
	require.NoError(t, err)
}
