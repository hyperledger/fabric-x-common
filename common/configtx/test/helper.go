/*
Copyright IBM Corp. All Rights Reserved.

SPDX-License-Identifier: Apache-2.0
*/

package test

import (
	cb "github.com/hyperledger/fabric-protos-go-apiv2/common"
	mspproto "github.com/hyperledger/fabric-protos-go-apiv2/msp"
	pb "github.com/hyperledger/fabric-protos-go-apiv2/peer"

	"github.com/hyperledger/fabric-x-common/common/channelconfig"
	"github.com/hyperledger/fabric-x-common/common/genesis"
	"github.com/hyperledger/fabric-x-common/common/util"
	"github.com/hyperledger/fabric-x-common/core/config/configtest"
	"github.com/hyperledger/fabric-x-common/protoutil"
	"github.com/hyperledger/fabric-x-common/tools/configtxgen"
	"github.com/hyperledger/fabric-x-common/tools/pkg/txflags"
)

var logger = util.MustGetLogger("common.configtx.test")

// MakeGenesisBlock creates a genesis block using the test templates for the given channelID
func MakeGenesisBlock(channelID string) (*cb.Block, error) {
	profile := configtxgen.Load(configtxgen.SampleDevModeSoloProfile, configtest.GetDevConfigDir())
	channelGroup, err := configtxgen.NewChannelGroup(profile)
	if err != nil {
		logger.Panicf("Error creating channel config: %s", err)
	}

	gb := genesis.NewFactoryImpl(channelGroup).Block(channelID)
	if gb == nil {
		return gb, nil
	}

	txsFilter := txflags.NewWithValues(len(gb.Data.Data), pb.TxValidationCode_VALID)
	gb.Metadata.Metadata[cb.BlockMetadataIndex_TRANSACTIONS_FILTER] = txsFilter

	return gb, nil
}

func MakeChannelConfig(channelID string) (*cb.Config, error) {
	profile := configtxgen.Load(configtxgen.SampleDevModeSoloProfile, configtest.GetDevConfigDir())
	channelGroup, err := configtxgen.NewChannelGroup(profile)
	if err != nil {
		return nil, err
	}
	return &cb.Config{ChannelGroup: channelGroup}, nil
}

// MakeGenesisBlockFromMSPs creates a genesis block using the MSPs provided for the given channelID
func MakeGenesisBlockFromMSPs(channelID string, appMSPConf, ordererMSPConf *mspproto.MSPConfig, appOrgID, ordererOrgID string) (*cb.Block, error) {
	profile := configtxgen.Load(configtxgen.SampleDevModeSoloProfile, configtest.GetDevConfigDir())

	channelGroup, err := configtxgen.NewChannelGroup(profile)
	if err != nil {
		logger.Panicf("Error creating channel config: %s", err)
	}

	ordererOrg := protoutil.NewConfigGroup()
	ordererOrg.ModPolicy = channelconfig.AdminsPolicyKey
	ordererOrg.Values[channelconfig.MSPKey] = &cb.ConfigValue{
		Value:     protoutil.MarshalOrPanic(channelconfig.MSPValue(ordererMSPConf).Value()),
		ModPolicy: channelconfig.AdminsPolicyKey,
	}

	endpoints := profile.Orderer.Organizations[0].OrdererEndpoints
	ordererOrgProtos := &cb.OrdererAddresses{
		Addresses: make([]string, len(endpoints)),
	}
	for i, e := range endpoints {
		ordererOrgProtos.Addresses[i] = e.String()
	}
	ordererOrg.Values[channelconfig.EndpointsKey] = &cb.ConfigValue{
		Value:     protoutil.MarshalOrPanic(ordererOrgProtos),
		ModPolicy: channelconfig.AdminsPolicyKey,
	}

	applicationOrg := protoutil.NewConfigGroup()
	applicationOrg.ModPolicy = channelconfig.AdminsPolicyKey
	applicationOrg.Values[channelconfig.MSPKey] = &cb.ConfigValue{
		Value:     protoutil.MarshalOrPanic(channelconfig.MSPValue(appMSPConf).Value()),
		ModPolicy: channelconfig.AdminsPolicyKey,
	}
	applicationOrg.Values[channelconfig.AnchorPeersKey] = &cb.ConfigValue{
		Value:     protoutil.MarshalOrPanic(channelconfig.AnchorPeersValue([]*pb.AnchorPeer{}).Value()),
		ModPolicy: channelconfig.AdminsPolicyKey,
	}

	channelGroup.Groups[channelconfig.OrdererGroupKey].Groups[ordererOrgID] = ordererOrg
	channelGroup.Groups[channelconfig.ApplicationGroupKey].Groups[appOrgID] = applicationOrg

	return genesis.NewFactoryImpl(channelGroup).Block(channelID), nil
}
