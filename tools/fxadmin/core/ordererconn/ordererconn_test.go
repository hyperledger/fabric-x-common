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
	"github.com/hyperledger/fabric-x-common/tools/fxadmin/core/ordererconn"
)

func TestLoad(t *testing.T) {
	t.Parallel()

	t.Run("extracts channel id, router and assembler endpoints and TLS CA certs", func(t *testing.T) {
		t.Parallel()
		sharedConfig := &ordererpb.SharedConfig{
			PartiesConfig: []*ordererpb.PartyConfig{
				{
					PartyID:         1,
					TLSCACerts:      [][]byte{[]byte("ca-1")},
					RouterConfig:    &ordererpb.RouterNodeConfig{Host: "router1.example.com", Port: 8013},
					AssemblerConfig: &ordererpb.AssemblerNodeConfig{Host: "assembler1.example.com", Port: 8011},
				},
				{
					PartyID:         2,
					TLSCACerts:      [][]byte{[]byte("ca-2a"), []byte("ca-2b")},
					RouterConfig:    &ordererpb.RouterNodeConfig{Host: "router2.example.com", Port: 8014},
					AssemblerConfig: &ordererpb.AssemblerNodeConfig{Host: "assembler2.example.com", Port: 8012},
				},
			},
		}
		block := newConfigBlock(t, "arma", sharedConfig)

		info, err := ordererconn.Load(block, factory.GetDefault())
		require.NoError(t, err)
		require.Equal(t, "arma", info.ChannelID)
		require.Equal(t, []string{"router1.example.com:8013", "router2.example.com:8014"}, info.RouterEndpoints)
		require.Equal(t,
			[]string{"assembler1.example.com:8011", "assembler2.example.com:8012"},
			info.AssemblerEndpoints)
		require.Equal(t, [][]byte{[]byte("ca-1"), []byte("ca-2a"), []byte("ca-2b")}, info.TLSCACerts)
	})

	t.Run("errors when no party has a router", func(t *testing.T) {
		t.Parallel()
		sharedConfig := &ordererpb.SharedConfig{
			PartiesConfig: []*ordererpb.PartyConfig{{
				PartyID:         1,
				TLSCACerts:      [][]byte{[]byte("ca-1")},
				AssemblerConfig: &ordererpb.AssemblerNodeConfig{Host: "assembler1.example.com", Port: 8011},
			}},
		}
		block := newConfigBlock(t, "arma", sharedConfig)

		_, err := ordererconn.Load(block, factory.GetDefault())
		require.ErrorContains(t, err, "no router endpoints")
	})

	t.Run("errors when no party has an assembler", func(t *testing.T) {
		t.Parallel()
		sharedConfig := &ordererpb.SharedConfig{
			PartiesConfig: []*ordererpb.PartyConfig{{
				PartyID:      1,
				TLSCACerts:   [][]byte{[]byte("ca-1")},
				RouterConfig: &ordererpb.RouterNodeConfig{Host: "router1.example.com", Port: 8013},
			}},
		}
		block := newConfigBlock(t, "arma", sharedConfig)

		_, err := ordererconn.Load(block, factory.GetDefault())
		require.ErrorContains(t, err, "no assembler endpoints")
	})

	t.Run("errors on a block that is not a config block", func(t *testing.T) {
		t.Parallel()
		_, err := ordererconn.Load(&cb.Block{}, factory.GetDefault())
		require.Error(t, err)
	})
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
