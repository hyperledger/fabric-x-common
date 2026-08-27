/*
Copyright IBM Corp. All Rights Reserved.

SPDX-License-Identifier: Apache-2.0
*/

// Package ordererconn extracts the orderer connection information — channel ID,
// router and assembler endpoints, and the orderer organizations' TLS CA
// certificates — from a Fabric-X config block.
package ordererconn

import (
	"net"
	"strconv"

	"github.com/cockroachdb/errors"
	"github.com/hyperledger/fabric-lib-go/bccsp"
	cb "github.com/hyperledger/fabric-protos-go-apiv2/common"
	"google.golang.org/protobuf/proto"

	"github.com/hyperledger/fabric-x-common/api/ordererpb"
	"github.com/hyperledger/fabric-x-common/common/channelconfig"
	"github.com/hyperledger/fabric-x-common/protoutil"
	"github.com/hyperledger/fabric-x-common/tools/configtxgen"
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

	if consensusType := ordererConfig.ConsensusType(); consensusType != configtxgen.Arma {
		return nil, errors.Newf("unsupported consensus type %q: expected %q", consensusType, configtxgen.Arma)
	}

	sharedConfig, err := unmarshalSharedConfig(ordererConfig.ConsensusMetadata())
	if err != nil {
		return nil, err
	}

	return infoFromSharedConfig(bundle.ConfigtxValidator().ChannelID(), sharedConfig)
}

// unmarshalSharedConfig parses the ARMA shared config from consensus metadata.
func unmarshalSharedConfig(metadata []byte) (*ordererpb.SharedConfig, error) {
	sharedConfig := &ordererpb.SharedConfig{}
	if err := proto.Unmarshal(metadata, sharedConfig); err != nil {
		return nil, errors.Wrap(err, "failed to unmarshal ARMA shared config from consensus metadata")
	}
	return sharedConfig, nil
}

// infoFromSharedConfig collects each party's router and assembler endpoints and
// TLS CA certificates from an ARMA shared config into an Info for the given
// channel.
func infoFromSharedConfig(channelID string, shared *ordererpb.SharedConfig) (*Info, error) {
	info := &Info{ChannelID: channelID, NumParties: len(shared.GetPartiesConfig())}
	for _, party := range shared.GetPartiesConfig() {
		if router := party.GetRouterConfig(); router != nil {
			info.RouterEndpoints = append(info.RouterEndpoints,
				net.JoinHostPort(router.GetHost(), strconv.Itoa(int(router.GetPort()))))
		}
		if assembler := party.GetAssemblerConfig(); assembler != nil {
			info.AssemblerEndpoints = append(info.AssemblerEndpoints,
				net.JoinHostPort(assembler.GetHost(), strconv.Itoa(int(assembler.GetPort()))))
		}
		info.TLSCACerts = append(info.TLSCACerts, party.GetTLSCACerts()...)
	}

	if len(info.RouterEndpoints) == 0 {
		return nil, errors.New("config block has no router endpoints")
	}
	if len(info.AssemblerEndpoints) == 0 {
		return nil, errors.New("config block has no assembler endpoints")
	}
	return info, nil
}
