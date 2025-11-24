/*
Copyright IBM Corp. All Rights Reserved.

SPDX-License-Identifier: Apache-2.0
*/

package ordererext

import (
	"fmt"

	"github.com/hyperledger/fabric-x-common/api/protocommon"
	"github.com/hyperledger/fabric-x-common/api/protoetcdraft"
	"github.com/hyperledger/fabric-x-common/api/protomsp"
	"github.com/hyperledger/fabric-x-common/api/protoorderer"
	"github.com/hyperledger/fabric-x-common/api/protosmartbft"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/emptypb"
)

type DynamicOrdererGroup struct {
	*protocommon.ConfigGroup
}

func (dcg *DynamicOrdererGroup) Underlying() proto.Message {
	return dcg.ConfigGroup
}

func (dcg *DynamicOrdererGroup) DynamicMapFields() []string {
	return []string{"values", "groups"}
}

func (dcg *DynamicOrdererGroup) DynamicMapFieldProto(name string, key string, base proto.Message) (proto.Message, error) {
	switch name {
	case "groups":
		cg, ok := base.(*protocommon.ConfigGroup)
		if !ok {
			return nil, fmt.Errorf("ConfigGroup groups can only contain ConfigGroup messages")
		}

		return &DynamicOrdererOrgGroup{
			ConfigGroup: cg,
		}, nil
	case "values":
		cv, ok := base.(*protocommon.ConfigValue)
		if !ok {
			return nil, fmt.Errorf("ConfigGroup values can only contain ConfigValue messages")
		}
		return &DynamicOrdererConfigValue{
			ConfigValue: cv,
			name:        key,
		}, nil
	default:
		return nil, fmt.Errorf("ConfigGroup does not have a dynamic field: %s", name)
	}
}

type ConsensusTypeMetadataFactory interface {
	NewMessage() proto.Message
}

// ConsensuTypeMetadataMap should have consensus implementations register their metadata message factories
var ConsensusTypeMetadataMap = map[string]ConsensusTypeMetadataFactory{}

type ConsensusType struct {
	*protoorderer.ConsensusType
}

func (ct *ConsensusType) Underlying() proto.Message {
	return ct.ConsensusType
}

func (ct *ConsensusType) VariablyOpaqueFields() []string {
	return []string{"metadata"}
}

func (ct *ConsensusType) VariablyOpaqueFieldProto(name string) (proto.Message, error) {
	if name != "metadata" {
		return nil, fmt.Errorf("not a valid opaque field: %s", name)
	}
	switch ct.Type {
	case "etcdraft":
		return &protoetcdraft.ConfigMetadata{}, nil
	case "BFT":
		return &protosmartbft.Options{}, nil
	default:
		return &emptypb.Empty{}, nil
	}
}

type DynamicOrdererOrgGroup struct {
	*protocommon.ConfigGroup
}

func (dcg *DynamicOrdererOrgGroup) Underlying() proto.Message {
	return dcg.ConfigGroup
}

func (dcg *DynamicOrdererOrgGroup) DynamicMapFields() []string {
	return []string{"groups", "values"}
}

func (dcg *DynamicOrdererOrgGroup) DynamicMapFieldProto(name string, key string, base proto.Message) (proto.Message, error) {
	switch name {
	case "groups":
		return nil, fmt.Errorf("the orderer orgs do not support sub-groups")
	case "values":
		cv, ok := base.(*protocommon.ConfigValue)
		if !ok {
			return nil, fmt.Errorf("ConfigGroup values can only contain ConfigValue messages")
		}

		return &DynamicOrdererOrgConfigValue{
			ConfigValue: cv,
			name:        key,
		}, nil
	default:
		return nil, fmt.Errorf("not a dynamic orderer map field: %s", name)
	}
}

type DynamicOrdererConfigValue struct {
	*protocommon.ConfigValue
	name string
}

func (docv *DynamicOrdererConfigValue) Underlying() proto.Message {
	return docv.ConfigValue
}

func (docv *DynamicOrdererConfigValue) StaticallyOpaqueFields() []string {
	return []string{"value"}
}

func (docv *DynamicOrdererConfigValue) StaticallyOpaqueFieldProto(name string) (proto.Message, error) {
	if name != "value" {
		return nil, fmt.Errorf("not a marshaled field: %s", name)
	}
	switch docv.name {
	case "ConsensusType":
		return &protoorderer.ConsensusType{}, nil
	case "BatchSize":
		return &protoorderer.BatchSize{}, nil
	case "BatchTimeout":
		return &protoorderer.BatchTimeout{}, nil
	case "KafkaBrokers":
		return &protoorderer.KafkaBrokers{}, nil
	case "ChannelRestrictions":
		return &protoorderer.ChannelRestrictions{}, nil
	case "Capabilities":
		return &protocommon.Capabilities{}, nil
	case "Orderers":
		return &protocommon.Orderers{}, nil
	default:
		return nil, fmt.Errorf("unknown Orderer ConfigValue name: %s", docv.name)
	}
}

type DynamicOrdererOrgConfigValue struct {
	*protocommon.ConfigValue
	name string
}

func (doocv *DynamicOrdererOrgConfigValue) Underlying() proto.Message {
	return doocv.ConfigValue
}

func (doocv *DynamicOrdererOrgConfigValue) StaticallyOpaqueFields() []string {
	return []string{"value"}
}

func (doocv *DynamicOrdererOrgConfigValue) StaticallyOpaqueFieldProto(name string) (proto.Message, error) {
	if name != "value" {
		return nil, fmt.Errorf("not a marshaled field: %s", name)
	}
	switch doocv.name {
	case "MSP":
		return &protomsp.MSPConfig{}, nil
	case "Endpoints":
		return &protocommon.OrdererAddresses{}, nil
	default:
		return nil, fmt.Errorf("unknown Orderer Org ConfigValue name: %s", doocv.name)
	}
}
