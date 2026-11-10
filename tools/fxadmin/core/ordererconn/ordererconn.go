/*
Copyright IBM Corp. All Rights Reserved.

SPDX-License-Identifier: Apache-2.0
*/

// Package ordererconn extracts the orderer connection information — channel ID,
// router and assembler endpoints, and the orderer organizations' TLS CA
// certificates — from a Fabric-X config block.
package ordererconn

import (
	"fmt"
	"math"
	"net"
	"strconv"

	"github.com/cockroachdb/errors"
	"github.com/hyperledger/fabric-lib-go/bccsp"
	cb "github.com/hyperledger/fabric-protos-go-apiv2/common"

	"github.com/hyperledger/fabric-x-common/api/ordererpb"
	"github.com/hyperledger/fabric-x-common/common/channelconfig"
	"github.com/hyperledger/fabric-x-common/protoutil"
	"github.com/hyperledger/fabric-x-common/tools/fxadmin/core/helpers"
)

// Info holds everything needed to dial the orderer, extracted from a config
// block: the channel ID, the number of parties, the ordered router and
// assembler endpoints, and the aggregated TLS CA certificates used to verify
// the orderer nodes' server certificates.
type Info struct {
	ChannelID          string
	NumParties         int // number of parties in the ARMA shared config
	RouterEndpoints    []string
	AssemblerEndpoints []string
	TLSCACerts         [][]byte // TLS CA certificates of orderer organizations
}

// Load extracts the orderer connection information from a config block. It reads
// the ARMA shared config from the block's consensus metadata and collects
// each party's router and assembler endpoints and its TLS CA certificates.
func Load(block *cb.Block, csp bccsp.BCCSP) (*Info, error) {
	envelope, err := protoutil.ExtractEnvelope(block, 0)
	if err != nil {
		return nil, errors.Wrap(err, "failed to extract envelope from config block")
	}

	bundle, err := channelconfig.NewBundleFromEnvelope(envelope, csp)
	if err != nil {
		return nil, errors.Wrap(err, "failed to build channel config bundle from config block")
	}

	ordererConfig, exists := bundle.OrdererConfig()
	if !exists {
		return nil, errors.New("config block has no orderer configuration")
	}

	sharedConfig, err := helpers.UnmarshalArmaSharedConfig(ordererConfig.ConsensusType(),
		ordererConfig.ConsensusMetadata())
	if err != nil {
		return nil, err
	}

	return infoFromSharedConfig(bundle.ConfigtxValidator().ChannelID(), sharedConfig)
}

// infoFromSharedConfig collects each party's router and assembler endpoints and
// TLS CA certificates from an ARMA shared config into an Info for the given
// channel. It errors if the shared config has no parties, or if a party's
// router or assembler is missing or has an invalid endpoint.
func infoFromSharedConfig(channelID string, shared *ordererpb.SharedConfig) (*Info, error) {
	if len(shared.GetPartiesConfig()) == 0 {
		return nil, errors.New("the ARMA shared config has no parties")
	}
	info := &Info{ChannelID: channelID, NumParties: len(shared.GetPartiesConfig())}
	for _, party := range shared.GetPartiesConfig() {
		router, err := nodeEndpoint(party.GetRouterConfig().GetNodeConfig())
		if err != nil {
			return nil, fmt.Errorf("router of party %d: %w", party.GetPartyID(), err)
		}
		assembler, err := nodeEndpoint(party.GetAssemblerConfig().GetNodeConfig())
		if err != nil {
			return nil, fmt.Errorf("assembler of party %d: %w", party.GetPartyID(), err)
		}
		info.RouterEndpoints = append(info.RouterEndpoints, router)
		info.AssemblerEndpoints = append(info.AssemblerEndpoints, assembler)
		info.TLSCACerts = append(info.TLSCACerts, party.GetTLSCACerts()...)
	}
	return info, nil
}

// nodeEndpoint returns the "host:port" endpoint of a node config. It errors if
// the node is not configured, its host is empty, or its port is not 1-65535.
func nodeEndpoint(node *ordererpb.NodeConfig) (string, error) {
	if node == nil {
		return "", errors.New("not configured")
	}
	if node.GetHost() == "" {
		return "", errors.New("empty host")
	}
	if port := node.GetPort(); port == 0 || port > math.MaxUint16 {
		return "", errors.Newf("invalid port %d", port)
	}
	return net.JoinHostPort(node.GetHost(), strconv.Itoa(int(node.GetPort()))), nil
}
