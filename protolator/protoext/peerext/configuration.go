/*
Copyright IBM Corp. All Rights Reserved.

SPDX-License-Identifier: Apache-2.0
*/

package peerext

import (
	"fmt"

	"github.com/hyperledger/fabric-x-common/api/protocommon"
	"github.com/hyperledger/fabric-x-common/api/protomsp"
	"github.com/hyperledger/fabric-x-common/api/protopeer"
	"google.golang.org/protobuf/proto"
)

type DynamicApplicationGroup struct {
	*protocommon.ConfigGroup
}

func (dag *DynamicApplicationGroup) Underlying() proto.Message {
	return dag.ConfigGroup
}

func (dag *DynamicApplicationGroup) DynamicMapFields() []string {
	return []string{"groups", "values"}
}

func (dag *DynamicApplicationGroup) DynamicMapFieldProto(name string, key string, base proto.Message) (proto.Message, error) {
	switch name {
	case "groups":
		cg, ok := base.(*protocommon.ConfigGroup)
		if !ok {
			return nil, fmt.Errorf("ConfigGroup groups can only contain ConfigGroup messages")
		}

		return &DynamicApplicationOrgGroup{
			ConfigGroup: cg,
		}, nil
	case "values":
		cv, ok := base.(*protocommon.ConfigValue)
		if !ok {
			return nil, fmt.Errorf("ConfigGroup values can only contain ConfigValue messages")
		}
		return &DynamicApplicationConfigValue{
			ConfigValue: cv,
			name:        key,
		}, nil
	default:
		return nil, fmt.Errorf("ConfigGroup does not have a dynamic field: %s", name)
	}
}

type DynamicApplicationOrgGroup struct {
	*protocommon.ConfigGroup
}

func (dag *DynamicApplicationOrgGroup) Underlying() proto.Message {
	return dag.ConfigGroup
}

func (dag *DynamicApplicationOrgGroup) DynamicMapFields() []string {
	return []string{"groups", "values"}
}

func (dag *DynamicApplicationOrgGroup) DynamicMapFieldProto(name string, key string, base proto.Message) (proto.Message, error) {
	switch name {
	case "groups":
		return nil, fmt.Errorf("The application orgs do not support sub-groups")
	case "values":
		cv, ok := base.(*protocommon.ConfigValue)
		if !ok {
			return nil, fmt.Errorf("ConfigGroup values can only contain ConfigValue messages")
		}

		return &DynamicApplicationOrgConfigValue{
			ConfigValue: cv,
			name:        key,
		}, nil
	default:
		return nil, fmt.Errorf("Not a dynamic application map field: %s", name)
	}
}

type DynamicApplicationConfigValue struct {
	*protocommon.ConfigValue
	name string
}

func (ccv *DynamicApplicationConfigValue) Underlying() proto.Message {
	return ccv.ConfigValue
}

func (ccv *DynamicApplicationConfigValue) StaticallyOpaqueFields() []string {
	return []string{"value"}
}

func (ccv *DynamicApplicationConfigValue) StaticallyOpaqueFieldProto(name string) (proto.Message, error) {
	if name != "value" {
		return nil, fmt.Errorf("Not a marshaled field: %s", name)
	}
	switch ccv.name {
	case "Capabilities":
		return &protocommon.Capabilities{}, nil
	case "ACLs":
		return &protopeer.ACLs{}, nil
	default:
		return nil, fmt.Errorf("Unknown Application ConfigValue name: %s", ccv.name)
	}
}

type DynamicApplicationOrgConfigValue struct {
	*protocommon.ConfigValue
	name string
}

func (daocv *DynamicApplicationOrgConfigValue) Underlying() proto.Message {
	return daocv.ConfigValue
}

func (daocv *DynamicApplicationOrgConfigValue) StaticallyOpaqueFields() []string {
	return []string{"value"}
}

func (daocv *DynamicApplicationOrgConfigValue) StaticallyOpaqueFieldProto(name string) (proto.Message, error) {
	if name != "value" {
		return nil, fmt.Errorf("Not a marshaled field: %s", name)
	}
	switch daocv.name {
	case "MSP":
		return &protomsp.MSPConfig{}, nil
	case "AnchorPeers":
		return &protopeer.AnchorPeers{}, nil
	default:
		return nil, fmt.Errorf("Unknown Application Org ConfigValue name: %s", daocv.name)
	}
}
