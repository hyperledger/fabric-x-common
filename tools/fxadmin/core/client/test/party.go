/*
Copyright IBM Corp. All Rights Reserved.

SPDX-License-Identifier: Apache-2.0
*/

package test

import (
	"net"
	"strconv"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/hyperledger/fabric-x-common/api/ordererpb"
)

// ArmaParty returns the ARMA shared config entry of a party whose router and
// assembler listen on the given "host:port" endpoints. An empty endpoint leaves
// that node out of the party.
func ArmaParty(t *testing.T, partyID uint32, routerEndpoint, assemblerEndpoint string) *ordererpb.PartyConfig {
	t.Helper()
	party := &ordererpb.PartyConfig{PartyID: partyID}
	if routerEndpoint != "" {
		party.RouterConfig = &ordererpb.RouterNodeConfig{NodeConfig: nodeConfig(t, routerEndpoint)}
	}
	if assemblerEndpoint != "" {
		party.AssemblerConfig = &ordererpb.AssemblerNodeConfig{NodeConfig: nodeConfig(t, assemblerEndpoint)}
	}
	return party
}

// nodeConfig returns the node config of a node that listens on the given
// "host:port" endpoint and has no certificates.
func nodeConfig(t *testing.T, endpoint string) *ordererpb.NodeConfig {
	t.Helper()
	host, portStr, err := net.SplitHostPort(endpoint)
	require.NoError(t, err)
	port, err := strconv.ParseUint(portStr, 10, 32)
	require.NoError(t, err)
	return &ordererpb.NodeConfig{Host: host, Port: uint32(port)}
}
