/*
Copyright IBM Corp. All Rights Reserved.

SPDX-License-Identifier: Apache-2.0
*/

package modify

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/cockroachdb/errors"
	cb "github.com/hyperledger/fabric-protos-go-apiv2/common"
	mspproto "github.com/hyperledger/fabric-protos-go-apiv2/msp"
	"go.yaml.in/yaml/v3"
	"google.golang.org/protobuf/proto"

	"github.com/hyperledger/fabric-x-common/api/ordererpb"
	"github.com/hyperledger/fabric-x-common/api/types"
	"github.com/hyperledger/fabric-x-common/common/channelconfig"
	"github.com/hyperledger/fabric-x-common/protolator"
	"github.com/hyperledger/fabric-x-common/protolator/protoext/ordererext"
)

// Keys of a party definition: the party's entry of the shared config, the
// party's orderer org, and, inside the org, the org's name.
const (
	partyDefPartyKey   = "Party"
	partyDefOrgKey     = "Org"
	partyDefOrgNameKey = "Name"
)

// partyDefinition is a decoded party definition: the party's shared config
// entry, and its orderer org's name and config group.
type partyDefinition struct {
	party   *ordererpb.PartyConfig
	orgName string
	org     *cb.ConfigGroup
}

// PartyAdd implements `fxadmin modify party add`. The party definition at
// partyPath holds the party's entry of the ARMA shared config (Party) and the
// config group of its new orderer org (Org), both exactly as the block stores
// them. They are added to the config block at blockPath, which is rewritten in
// place. The org group carries the org's Endpoints value, the router's
// broadcast and the assembler's deliver addresses, which is added as given.
// The command also updates the elements of the block that must agree with the
// new party: the shared config's MaxPartyID, the party's consenter in the
// Orderers value, and the Orderer group's BlockValidation policy over all the
// consenters.
func (*Handler) PartyAdd(partyPath, blockPath string) error {
	logger.Debugf("modify party add: party=%s block=%s", partyPath, blockPath)

	def, err := loadPartyDefinition(partyPath)
	if err != nil {
		return err
	}
	block, err := readAndDecodeConfigBlock(blockPath)
	if err != nil {
		return err
	}
	orderer := block.Config().GetChannelGroup().GetGroups()[channelconfig.OrdererGroupKey]
	if orderer == nil {
		return errors.Newf("the config has no %s group", channelconfig.OrdererGroupKey)
	}
	if err := addParty(orderer, def); err != nil {
		return err
	}

	if err := validateEncodeAndWriteConfigBlock(block, blockPath); err != nil {
		return err
	}
	logger.Infof("added party %d with orderer org %s to %s", def.party.GetPartyID(), def.orgName, blockPath)
	return nil
}

// addParty adds the party and its org to the Orderer group, in each of the
// places the block holds a party:
//   - the shared config (the ConsensusType value's metadata): the party is
//     appended to PartiesConfig, and MaxPartyID is set to its PartyID;
//   - the orderer org: its group is added under the org's name, with its MSP,
//     its policies, and its Endpoints value, which lists the router's broadcast
//     and the assembler's deliver addresses;
//   - the consenter mapping (the Orderers value): the party's consenter is
//     appended, with the endpoint and certificates of its ConsenterConfig and
//     the MSP ID of its org;
//   - the BlockValidation policy: it is recomputed over all the consenters'
//     signing certificates.
func addParty(orderer *cb.ConfigGroup, def *partyDefinition) error {
	if _, exists := orderer.GetGroups()[def.orgName]; exists {
		return errors.Newf("orderer org %q already exists", def.orgName)
	}
	if err := validatePartyHosts(def.party); err != nil {
		return err
	}
	if err := validateOrgEndpoints(def.org); err != nil {
		return fmt.Errorf("invalid orderer org %q: %w", def.orgName, err)
	}
	consenter, err := consenterOf(def.party, orgMSPID(def.org))
	if err != nil {
		return err
	}

	if err = addToSharedConfig(orderer, def.party); err != nil {
		return err
	}
	consenters, err := addConsenter(orderer, consenter)
	if err != nil {
		return err
	}
	if orderer.Groups == nil {
		orderer.Groups = make(map[string]*cb.ConfigGroup)
	}
	orderer.Groups[def.orgName] = def.org
	return recomputeBlockValidation(orderer, consenters)
}

// addToSharedConfig appends the party to the ARMA shared config, the consensus
// metadata of the Orderer group's ConsensusType value, and sets MaxPartyID to
// the party's PartyID. The orderer requires that PartyID to be the previous
// MaxPartyID + 1, and checks it when the config update is submitted.
func addToSharedConfig(orderer *cb.ConfigGroup, party *ordererpb.PartyConfig) error {
	consensusType, err := decodeConsensusType(orderer)
	if err != nil {
		return err
	}
	shared := consensusType.sharedConfig
	shared.PartiesConfig = append(shared.PartiesConfig, party)
	shared.MaxPartyID = party.GetPartyID()
	return consensusType.encode()
}

// consenterOf returns the party's consenter as the Orderers value lists it: the
// endpoint and certificates of the party's consenter node, under the MSP ID of
// the party's org. The orderer requires them to match the party's
// ConsenterConfig, so they are taken from it.
func consenterOf(party *ordererpb.PartyConfig, mspID string) (*cb.Consenter, error) {
	node := party.GetConsenterConfig().GetNodeConfig()
	if node == nil {
		return nil, errors.Newf("party %d has no ConsenterConfig", party.GetPartyID())
	}
	return &cb.Consenter{
		Id:            party.GetPartyID(),
		Host:          node.GetHost(),
		Port:          node.GetPort(),
		MspId:         mspID,
		Identity:      node.GetSignCert(),
		ClientTlsCert: node.GetClientTlsCert(),
		ServerTlsCert: node.GetServerTlsCert(),
	}, nil
}

// addConsenter appends the consenter to the consenter mapping of the Orderer
// group's Orderers value and returns the new mapping.
func addConsenter(orderer *cb.ConfigGroup, consenter *cb.Consenter) ([]*cb.Consenter, error) {
	orderers, err := decodeOrderers(orderer)
	if err != nil {
		return nil, err
	}
	orderers.orderers.ConsenterMapping = append(orderers.orderers.ConsenterMapping, consenter)
	if err := orderers.encode(); err != nil {
		return nil, err
	}
	return orderers.orderers.GetConsenterMapping(), nil
}

// validatePartyHosts checks that the host of each of the party's nodes is an IP
// address or a host name, as `modify party node --host` checks a new host. The
// host of a node that is not given is not checked: the party without a
// consenter, for example, is rejected later with its own error.
// TODO: the orderer does not check the hosts of a party's nodes yet. Remove
// this check once the orderer does, and revisit it when the validation checks
// are integrated into fxadmin.
func validatePartyHosts(party *ordererpb.PartyConfig) error {
	type node struct {
		name string
		host string
	}
	nodes := make([]node, 0, len(party.GetBatchersConfig())+3)
	if router := party.GetRouterConfig().GetNodeConfig(); router != nil {
		nodes = append(nodes, node{name: "router", host: router.GetHost()})
	}
	for _, batcher := range party.GetBatchersConfig() {
		if config := batcher.GetNodeConfig(); config != nil {
			name := fmt.Sprintf("batcher of shard %d", batcher.GetShardId())
			nodes = append(nodes, node{name: name, host: config.GetHost()})
		}
	}
	if consenter := party.GetConsenterConfig().GetNodeConfig(); consenter != nil {
		nodes = append(nodes, node{name: "consenter", host: consenter.GetHost()})
	}
	if assembler := party.GetAssemblerConfig().GetNodeConfig(); assembler != nil {
		nodes = append(nodes, node{name: "assembler", host: assembler.GetHost()})
	}
	for _, n := range nodes {
		if err := validateHost(n.host); err != nil {
			return fmt.Errorf("the %s of party %d: %w", n.name, party.GetPartyID(), err)
		}
	}
	return nil
}

// validateOrgEndpoints checks that each address in the org's Endpoints value
// parses as an orderer endpoint and that its host is an IP address or a host
// name. An org with no Endpoints value is left to the orderer, which requires a
// broadcast and a deliver endpoint.
// TODO: the orderer does not check the hosts of an org's endpoints yet. Remove
// this check once the orderer does, and revisit it when the validation checks
// are integrated into fxadmin.
func validateOrgEndpoints(org *cb.ConfigGroup) error {
	_, addresses, err := decodeOrgEndpoints(org)
	if err != nil {
		return err
	}
	for _, address := range addresses.GetAddresses() {
		endpoint, err := types.ParseOrdererEndpoint(address)
		if err != nil {
			return fmt.Errorf("failed to parse endpoint %q: %w", address, err)
		}
		if err := validateHost(endpoint.Host); err != nil {
			return fmt.Errorf("endpoint %q: %w", address, err)
		}
	}
	return nil
}

// orgMSPID returns the MSP ID of an org group, or "" if it cannot be read.
func orgMSPID(org *cb.ConfigGroup) string {
	mspConfig := &mspproto.MSPConfig{}
	if err := proto.Unmarshal(org.GetValues()[channelconfig.MSPKey].GetValue(), mspConfig); err != nil {
		return ""
	}
	fabricConfig := &mspproto.FabricMSPConfig{}
	if err := proto.Unmarshal(mspConfig.GetConfig(), fabricConfig); err != nil {
		return ""
	}
	return fabricConfig.GetName()
}

// loadPartyDefinition reads the party definition YAML at path. Its Party is the
// party's PartiesConfig entry and its Org the org's config group plus a Name
// key, both in the structure `fxadmin decode` renders, except that each
// certificate and CRL is given as the path of its PEM file instead of its
// content. A relative path is relative to the directory of the definition. The
// files are read in, and the two sections are then decoded as is.
func loadPartyDefinition(path string) (*partyDefinition, error) {
	content, err := os.ReadFile(path)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to read party definition %q", path)
	}
	definition := map[string]any{}
	if err = yaml.Unmarshal(content, &definition); err != nil {
		return nil, errors.Wrapf(err, "failed to parse party definition %q", path)
	}
	party, ok := definition[partyDefPartyKey].(map[string]any)
	if !ok {
		return nil, errors.Newf("party definition %q has no %s", path, partyDefPartyKey)
	}
	org, ok := definition[partyDefOrgKey].(map[string]any)
	if !ok {
		return nil, errors.Newf("party definition %q has no %s", path, partyDefOrgKey)
	}
	orgName, ok := org[partyDefOrgNameKey].(string)
	if !ok || orgName == "" {
		return nil, errors.Newf("the %s of party definition %q has no %s", partyDefOrgKey, path, partyDefOrgNameKey)
	}
	delete(org, partyDefOrgNameKey)
	if err := readCertificateFiles(definition, filepath.Dir(path)); err != nil {
		return nil, fmt.Errorf("party definition %q: %w", path, err)
	}

	def := &partyDefinition{party: &ordererpb.PartyConfig{}, orgName: orgName, org: &cb.ConfigGroup{}}
	if err := decodeProtolatorValue(party, def.party); err != nil {
		return nil, fmt.Errorf("failed to decode the %s of party definition %q: %w", partyDefPartyKey, path, err)
	}
	if err := decodeProtolatorValue(org, &ordererext.DynamicOrdererOrgGroup{ConfigGroup: def.org}); err != nil {
		return nil, fmt.Errorf("failed to decode the %s of party definition %q: %w", partyDefOrgKey, path, err)
	}
	return def, nil
}

// decodeProtolatorValue decodes a value parsed from YAML into msg. protolator
// reads JSON, so the value is re-encoded as JSON first.
func decodeProtolatorValue(value any, msg proto.Message) error {
	content, err := json.Marshal(value)
	if err != nil {
		return errors.Wrap(err, "failed to encode as JSON")
	}
	if err := protolator.DeepUnmarshalJSON(bytes.NewReader(content), msg); err != nil {
		return errors.Wrap(err, "failed to decode")
	}
	return nil
}
