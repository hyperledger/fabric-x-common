/*
Copyright IBM Corp. All Rights Reserved.

SPDX-License-Identifier: Apache-2.0
*/

package protoutil_test

import (
	"testing"

	"github.com/hyperledger/fabric-x-common/api/protocommon"
	"github.com/stretchr/testify/require"

	"github.com/hyperledger/fabric-x-common/protoutil"
)

func TestNewConfigGroup(t *testing.T) {
	require.Equal(t,
		&protocommon.ConfigGroup{
			Groups:   make(map[string]*protocommon.ConfigGroup),
			Values:   make(map[string]*protocommon.ConfigValue),
			Policies: make(map[string]*protocommon.ConfigPolicy),
		},
		protoutil.NewConfigGroup(),
	)
}
