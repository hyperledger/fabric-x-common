/*
Copyright IBM Corp. All Rights Reserved.

SPDX-License-Identifier: Apache-2.0
*/

package orderers_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/hyperledger/fabric-x-common/common/deliverclient/orderers"
	"github.com/hyperledger/fabric-x-common/common/util"
)

func TestCreateConnectionSource(t *testing.T) {
	factory := &orderers.ConnectionSourceFactory{}
	require.NotNil(t, factory)
	require.Nil(t, factory.Overrides)
	lg := util.MustGetLogger("test")
	connSource := factory.CreateConnectionSource(lg, "")
	require.NotNil(t, connSource)

	overrides := make(map[string]*orderers.Endpoint)
	overrides["127.0.0.1:1111"] = &orderers.Endpoint{
		Address:   "127.0.0.1:2222",
		RootCerts: [][]byte{{1, 2, 3, 4}, {5, 6, 7, 8}},
		Refreshed: make(chan struct{}),
	}
	factory = &orderers.ConnectionSourceFactory{Overrides: overrides}
	require.NotNil(t, factory)
	require.Len(t, factory.Overrides, 1)
	connSource = factory.CreateConnectionSource(lg, "")
	require.NotNil(t, connSource)
}
