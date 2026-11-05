/*
Copyright IBM Corp. All Rights Reserved.

SPDX-License-Identifier: Apache-2.0
*/

package ordererconn_test

import (
	"testing"

	"github.com/hyperledger/fabric-lib-go/bccsp/factory"
	cb "github.com/hyperledger/fabric-protos-go-apiv2/common"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/proto"

	"github.com/hyperledger/fabric-x-common/api/ordererpb"
	"github.com/hyperledger/fabric-x-common/api/types"
	"github.com/hyperledger/fabric-x-common/tools/configtxgen"
	"github.com/hyperledger/fabric-x-common/tools/cryptogen"
	"github.com/hyperledger/fabric-x-common/tools/fxadmin/core/client/test"
	"github.com/hyperledger/fabric-x-common/tools/fxadmin/core/ordererconn"
)

// armaChannel is the channel used by the test config blocks and updates.
const armaChannel = "arma"

// Endpoints of the parties' routers and assemblers.
const (
	router1    = "router1.example.com:8013"
	router2    = "router2.example.com:8014"
	assembler1 = "assembler1.example.com:8011"
	assembler2 = "assembler2.example.com:8012"
)

func TestLoad(t *testing.T) {
	t.Parallel()

	t.Run("extracts channel id, router and assembler endpoints and TLS CA certs", func(t *testing.T) {
		t.Parallel()
		sharedConfig := &ordererpb.SharedConfig{
			PartiesConfig: []*ordererpb.PartyConfig{
				withTLSCACerts(test.ArmaParty(t, 1, router1, assembler1), "ca-1"),
				withTLSCACerts(test.ArmaParty(t, 2, router2, assembler2), "ca-2a", "ca-2b"),
			},
		}
		block := newConfigBlock(t, armaChannel, sharedConfig)

		info, err := ordererconn.Load(block, factory.GetDefault())
		require.NoError(t, err)
		require.Equal(t, armaChannel, info.ChannelID)
		require.Equal(t, 2, info.NumParties)
		require.Equal(t, []string{router1, router2}, info.RouterEndpoints)
		require.Equal(t, []string{assembler1, assembler2}, info.AssemblerEndpoints)
		require.Equal(t, [][]byte{[]byte("ca-1"), []byte("ca-2a"), []byte("ca-2b")}, info.TLSCACerts)
	})

	t.Run("errors on a block that is not a config block", func(t *testing.T) {
		t.Parallel()
		_, err := ordererconn.Load(&cb.Block{}, factory.GetDefault())
		require.Error(t, err)
	})
}

// TestLoadRejectsAnInvalidSharedConfig verifies that Load errors on a shared
// config without parties, and on a party whose router or assembler is missing
// or has an invalid endpoint, even when the other parties are valid.
func TestLoadRejectsAnInvalidSharedConfig(t *testing.T) {
	t.Parallel()

	t.Run("no parties", func(t *testing.T) {
		t.Parallel()
		block := newConfigBlock(t, armaChannel, &ordererpb.SharedConfig{})

		_, err := ordererconn.Load(block, factory.GetDefault())
		require.ErrorContains(t, err, "the ARMA shared config has no parties")
	})

	// Each case breaks the router or the assembler of party 2; party 1 is valid.
	for _, tc := range []struct {
		name    string
		router  bool // break the router; otherwise the assembler
		node    *ordererpb.NodeConfig
		missing bool // no router or assembler config at all
		wantErr string
	}{
		{name: "party without a router", router: true, missing: true, wantErr: "router of party 2: not configured"},
		{
			name: "party without an assembler", missing: true,
			wantErr: "assembler of party 2: not configured",
		},
		{name: "router without a node config", router: true, wantErr: "router of party 2: not configured"},
		{name: "assembler without a node config", wantErr: "assembler of party 2: not configured"},
		{
			name: "router with an empty host", router: true, node: &ordererpb.NodeConfig{Port: 8014},
			wantErr: "router of party 2: empty host",
		},
		{
			name: "assembler with port 0", node: &ordererpb.NodeConfig{Host: "assembler2"},
			wantErr: "assembler of party 2: invalid port 0",
		},
		{
			name: "router with a port above 65535", router: true,
			node:    &ordererpb.NodeConfig{Host: "router2", Port: 70000},
			wantErr: "router of party 2: invalid port 70000",
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			party := test.ArmaParty(t, 2, router2, assembler2)
			switch {
			case tc.router && tc.missing:
				party.RouterConfig = nil
			case tc.router:
				party.RouterConfig.NodeConfig = tc.node
			case tc.missing:
				party.AssemblerConfig = nil
			default:
				party.AssemblerConfig.NodeConfig = tc.node
			}
			sharedConfig := &ordererpb.SharedConfig{
				PartiesConfig: []*ordererpb.PartyConfig{test.ArmaParty(t, 1, router1, assembler1), party},
			}
			block := newConfigBlock(t, armaChannel, sharedConfig)

			_, err := ordererconn.Load(block, factory.GetDefault())
			require.ErrorContains(t, err, tc.wantErr)
		})
	}
}

// newConfigBlock builds a real ARMA config block whose consensus metadata is
// the marshaled shared config, so Load can parse it end to end.
func newConfigBlock(t *testing.T, channelID string, sharedConfig *ordererpb.SharedConfig) *cb.Block {
	t.Helper()
	meta, err := proto.Marshal(sharedConfig)
	require.NoError(t, err)

	block, err := cryptogen.CreateOrExtendConfigBlockWithCrypto(cryptogen.ConfigBlockParameters{
		TargetPath:  t.TempDir(),
		BaseProfile: configtxgen.SampleFabricX,
		ChannelID:   channelID,
		Organizations: []cryptogen.OrganizationParameters{{
			Name:   "orderer-org-1",
			Domain: "orderer-org-1.com",
			OrdererEndpoints: []*types.OrdererEndpoint{
				{ID: 1, Host: "localhost", Port: 7050, API: []string{types.Broadcast}},
			},
			ConsenterNodes: []cryptogen.Node{{CommonName: "consenter", Hostname: "consenter"}},
			OrdererNodes:   []cryptogen.Node{{CommonName: "orderer-node", Hostname: "orderer-node"}},
		}},
		ArmaMetaBytes: meta,
	})
	require.NoError(t, err)
	return block
}

// withTLSCACerts sets the party's TLS CA certificates and returns the party.
func withTLSCACerts(party *ordererpb.PartyConfig, certs ...string) *ordererpb.PartyConfig {
	for _, cert := range certs {
		party.TLSCACerts = append(party.TLSCACerts, []byte(cert))
	}
	return party
}
