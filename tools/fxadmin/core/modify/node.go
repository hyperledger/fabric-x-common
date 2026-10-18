/*
Copyright IBM Corp. All Rights Reserved.

SPDX-License-Identifier: Apache-2.0
*/

package modify

import (
	"crypto/x509"
	"encoding/pem"
	"fmt"
	"os"

	"github.com/cockroachdb/errors"
	cb "github.com/hyperledger/fabric-protos-go-apiv2/common"
	ab "github.com/hyperledger/fabric-protos-go-apiv2/orderer"
	"google.golang.org/protobuf/proto"

	"github.com/hyperledger/fabric-x-common/api/ordererpb"
	"github.com/hyperledger/fabric-x-common/api/types"
	"github.com/hyperledger/fabric-x-common/common/channelconfig"
	"github.com/hyperledger/fabric-x-common/common/policies"
	"github.com/hyperledger/fabric-x-common/tools/fxadmin/core/modify/change"
)

// The roles of an ARMA party's nodes, as `modify party node --role` names them.
const (
	nodeRoleRouter    = "router"
	nodeRoleBatcher   = "batcher"
	nodeRoleConsenter = "consenter"
	nodeRoleAssembler = "assembler"
)

// armaOrdererType is the ConsensusType of an ARMA ordering service, whose
// consensus metadata is the ARMA shared config.
const armaOrdererType = "arma"

// nodeChange is a `modify party node` request with its certificate files read:
// the node it selects and the values to set. An empty value leaves its field
// unchanged.
type nodeChange struct {
	party    uint32
	role     string
	shard    uint32
	host     string
	port     uint32
	tlsCert  []byte
	signCert []byte
}

// nodeFields points at the endpoint and certificate fields of one node's entry
// in the shared config, fields that every node config has.
type nodeFields struct {
	host     *string
	port     *uint32
	tlsCert  *[]byte
	signCert *[]byte
}

// decodedConsensusType is the Orderer group's ConsensusType config value with
// its two encoded layers decoded: the ConsensusType it holds, and the ARMA
// shared config that is the ConsensusType's metadata. It keeps every layer, so
// that encode can write a changed shared config back into the config value.
// Orderer group
// └─ values["ConsensusType"]        configValue    *cb.ConfigValue
//
//	├─ version, mod_policy
//	└─ value  (bytes) ─────────► consensusType  *ab.ConsensusType
//	                                ├─ type: "arma", state
//	                                └─ metadata (bytes) ─► sharedConfig  *ordererpb.SharedConfig
//	                                                         ├─ PartiesConfig
//	                                                         ├─ ConsensusConfig, BatchingConfig
//	                                                         └─ MaxPartyID
type decodedConsensusType struct {
	configValue   *cb.ConfigValue
	consensusType *ab.ConsensusType
	sharedConfig  *ordererpb.SharedConfig
}

// PartyNode implements `fxadmin modify party node`. It sets the given fields of
// one node of a party (its router, its consenter, its assembler, or its batcher
// of a shard) in the party's entry of the ARMA shared config, and carries the
// change to the other places of the block that hold the same node:
//   - for the consenter, its entry in the Orderers consenter mapping and, for a
//     new signing certificate, the BlockValidation policy;
//   - for the router and the assembler, a new host or port to the party's
//     broadcast or deliver endpoint in its orderer org's Endpoints value.
//
// The config block at the request's BlockPath is rewritten in place.
func (*Handler) PartyNode(ch change.Node) error {
	logger.Debugf("modify party node: party=%d role=%s shard=%d block=%s", ch.Party, ch.Role, ch.Shard, ch.BlockPath)

	nc, err := readNodeChange(ch)
	if err != nil {
		return err
	}
	block, err := readConfigBlock(ch.BlockPath)
	if err != nil {
		return err
	}
	orderer := block.Config().GetChannelGroup().GetGroups()[channelconfig.OrdererGroupKey]
	if orderer == nil {
		return errors.Newf("the config has no %s group", channelconfig.OrdererGroupKey)
	}
	if err := changeNode(orderer, nc); err != nil {
		return err
	}

	if err := writeConfigBlock(block, ch.BlockPath); err != nil {
		return err
	}
	logger.Infof("changed the %s of party %d in %s", nodeName(nc), nc.party, ch.BlockPath)
	return nil
}

// readNodeChange checks that the request changes at least one field and reads
// its certificate files.
func readNodeChange(ch change.Node) (*nodeChange, error) {
	if ch.Host == "" && ch.Port == 0 && ch.TLSCert == "" && ch.SignCert == "" {
		return nil, errors.New("nothing to change: no host, port, TLS certificate or signing certificate is given")
	}
	nc := &nodeChange{party: ch.Party, role: ch.Role, shard: ch.Shard, host: ch.Host, port: ch.Port}
	var err error
	if ch.TLSCert != "" {
		if nc.tlsCert, err = readCertificateFile(ch.TLSCert); err != nil {
			return nil, err
		}
	}
	if ch.SignCert != "" {
		if nc.signCert, err = readCertificateFile(ch.SignCert); err != nil {
			return nil, err
		}
	}
	return nc, nil
}

// readCertificateFile reads a PEM-encoded X.509 certificate file and returns
// its content, which is what the shared config stores.
func readCertificateFile(path string) ([]byte, error) {
	content, err := os.ReadFile(path)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to read certificate %q", path)
	}
	block, _ := pem.Decode(content)
	if block == nil || block.Type != "CERTIFICATE" {
		return nil, errors.Newf("%q is not a PEM-encoded certificate", path)
	}
	if _, err := x509.ParseCertificate(block.Bytes); err != nil {
		return nil, errors.Wrapf(err, "failed to parse certificate %q", path)
	}
	return content, nil
}

// changeNode sets the node's fields in the shared config and carries the change
// to the other places of the Orderer group that hold the node.
func changeNode(orderer *cb.ConfigGroup, nc *nodeChange) error {
	consensusType, err := decodeConsensusType(orderer)
	if err != nil {
		return err
	}
	var party *ordererpb.PartyConfig
	for _, p := range consensusType.sharedConfig.GetPartiesConfig() {
		if p.GetPartyID() == nc.party {
			party = p
		}
	}
	if party == nil {
		return errors.Newf("the shared config has no party %d", nc.party)
	}
	fields, err := nodeFieldsOf(party, nc)
	if err != nil {
		return err
	}
	setNodeFields(fields, nc)
	if err = consensusType.encode(); err != nil {
		return err
	}

	switch nc.role {
	case nodeRoleConsenter:
		return syncConsenter(orderer, nc, fields)
	case nodeRoleRouter:
		return syncPartyEndpoint(orderer, nc, types.Broadcast, fields)
	case nodeRoleAssembler:
		return syncPartyEndpoint(orderer, nc, types.Deliver, fields)
	default:
		return nil
	}
}

// nodeFieldsOf returns the fields of the node the change selects in the
// party's shared config entry.
func nodeFieldsOf(party *ordererpb.PartyConfig, nc *nodeChange) (*nodeFields, error) {
	missing := errors.Newf("party %d has no %s", nc.party, nodeName(nc))
	switch nc.role {
	case nodeRoleRouter:
		if c := party.GetRouterConfig(); c != nil {
			return &nodeFields{host: &c.Host, port: &c.Port, tlsCert: &c.TlsCert, signCert: &c.SignCert}, nil
		}
	case nodeRoleConsenter:
		if c := party.GetConsenterConfig(); c != nil {
			return &nodeFields{host: &c.Host, port: &c.Port, tlsCert: &c.TlsCert, signCert: &c.SignCert}, nil
		}
	case nodeRoleAssembler:
		if c := party.GetAssemblerConfig(); c != nil {
			return &nodeFields{host: &c.Host, port: &c.Port, tlsCert: &c.TlsCert, signCert: &c.SignCert}, nil
		}
	case nodeRoleBatcher:
		for _, c := range party.GetBatchersConfig() {
			if c.GetShardID() == nc.shard {
				return &nodeFields{host: &c.Host, port: &c.Port, tlsCert: &c.TlsCert, signCert: &c.SignCert}, nil
			}
		}
	default:
		return nil, errors.Newf("unknown node role %q", nc.role)
	}
	return nil, missing
}

// setNodeFields sets each field the change gives a value for.
func setNodeFields(fields *nodeFields, nc *nodeChange) {
	if nc.host != "" {
		*fields.host = nc.host
	}
	if nc.port != 0 {
		*fields.port = nc.port
	}
	if nc.tlsCert != nil {
		*fields.tlsCert = nc.tlsCert
	}
	if nc.signCert != nil {
		*fields.signCert = nc.signCert
	}
}

// syncConsenter applies the consenter's changed fields to its entry in the
// Orderers consenter mapping, which the orderer requires to match the
// consenter's shared config entry: its endpoint, its TLS certificate as both
// the client and the server certificate, and its signing certificate as its
// identity. The BlockValidation policy lists the consenters' identities, so a
// new signing certificate also recomputes it.
func syncConsenter(orderer *cb.ConfigGroup, nc *nodeChange, fields *nodeFields) error {
	value := orderer.GetValues()[channelconfig.OrderersKey]
	if value == nil {
		return errors.Newf("the %s group has no %s value", channelconfig.OrdererGroupKey, channelconfig.OrderersKey)
	}
	orderers := &cb.Orderers{}
	if err := proto.Unmarshal(value.GetValue(), orderers); err != nil {
		return errors.Wrap(err, "failed to unmarshal the Orderers value")
	}
	var consenter *cb.Consenter
	for _, c := range orderers.GetConsenterMapping() {
		if c.GetId() == nc.party {
			consenter = c
		}
	}
	if consenter == nil {
		return errors.Newf("the consenter mapping has no consenter of party %d", nc.party)
	}

	if nc.host != "" || nc.port != 0 {
		consenter.Host, consenter.Port = *fields.host, *fields.port
	}
	if nc.tlsCert != nil {
		consenter.ClientTlsCert, consenter.ServerTlsCert = *fields.tlsCert, *fields.tlsCert
	}
	if nc.signCert != nil {
		consenter.Identity = *fields.signCert
	}
	content, err := proto.Marshal(orderers)
	if err != nil {
		return errors.Wrap(err, "failed to marshal the Orderers value")
	}
	value.Value = content

	if nc.signCert == nil {
		return nil
	}
	return recomputeBlockValidation(orderer, orderers.GetConsenterMapping())
}

// recomputeBlockValidation recomputes the Orderer group's BlockValidation
// policy over the consenters, as configtxgen builds it for an ARMA orderer: a
// BFT quorum of the consenters' signatures. The policy keeps its mod_policy and
// version.
func recomputeBlockValidation(orderer *cb.ConfigGroup, consenters []*cb.Consenter) error {
	current := orderer.GetPolicies()[policies.BlockValidationPolicyKey]
	if current == nil {
		return errors.Newf("the %s group has no %s policy", channelconfig.OrdererGroupKey,
			policies.BlockValidationPolicyKey)
	}
	policies.EncodeBFTBlockVerificationPolicy(consenters, orderer)
	orderer.Policies[policies.BlockValidationPolicyKey].Version = current.GetVersion()
	return nil
}

// endpointMove is the move of a party's endpoint that serves api to the new
// address of the node behind it.
type endpointMove struct {
	party uint32
	api   string
	host  string
	port  int
	// node names the node, for messages.
	node string
}

// syncPartyEndpoint moves the party's endpoint that serves api (broadcast for
// the router, deliver for the assembler) to the node's new host and port. The
// endpoint is the one in an orderer org's Endpoints value with the party's ID;
// every other endpoint is left as it is.
func syncPartyEndpoint(orderer *cb.ConfigGroup, nc *nodeChange, api string, fields *nodeFields) error {
	if nc.host == "" && nc.port == 0 {
		return nil
	}
	move := &endpointMove{party: nc.party, api: api, host: *fields.host, port: int(*fields.port), node: nodeName(nc)}
	found := 0
	for orgName, org := range orderer.GetGroups() {
		moved, err := movePartyEndpoint(org, orgName, move)
		if err != nil {
			return err
		}
		found += moved
	}
	if found != 1 {
		return errors.Newf("the orderer orgs have %d %s endpoints of party %d, want 1", found, api, nc.party)
	}
	return nil
}

// movePartyEndpoint moves the party's endpoints that serve the move's API in one
// orderer org's Endpoints value, and returns how many it moved. Such an endpoint
// must serve that API alone, because an endpoint that serves both APIs cannot
// follow a single node.
func movePartyEndpoint(org *cb.ConfigGroup, orgName string, move *endpointMove) (int, error) {
	value := org.GetValues()[channelconfig.EndpointsKey]
	if value == nil {
		return 0, nil
	}
	addresses := &cb.OrdererAddresses{}
	if err := proto.Unmarshal(value.GetValue(), addresses); err != nil {
		return 0, errors.Wrapf(err, "failed to unmarshal the Endpoints value of orderer org %q", orgName)
	}
	moved := 0
	for i, address := range addresses.GetAddresses() {
		endpoint, err := types.ParseOrdererEndpoint(address)
		if err != nil || endpoint.ID != move.party || !endpoint.SupportsAPI(move.api) {
			continue
		}
		if len(endpoint.API) != 1 {
			return 0, errors.Newf("endpoint %q of orderer org %q serves more than %s, so it cannot follow the %s",
				address, orgName, move.api, move.node)
		}
		endpoint.Host, endpoint.Port = move.host, move.port
		addresses.Addresses[i] = endpoint.String()
		moved++
	}
	if moved == 0 {
		return 0, nil
	}
	content, err := proto.Marshal(addresses)
	if err != nil {
		return 0, errors.Wrapf(err, "failed to marshal the Endpoints value of orderer org %q", orgName)
	}
	value.Value = content
	return moved, nil
}

// decodeConsensusType decodes the Orderer group's ConsensusType config value
// down to its ARMA shared config. It errors if the orderer type is not ARMA.
func decodeConsensusType(orderer *cb.ConfigGroup) (*decodedConsensusType, error) {
	configValue := orderer.GetValues()[channelconfig.ConsensusTypeKey]
	if configValue == nil {
		return nil, errors.Newf("the %s group has no %s value", channelconfig.OrdererGroupKey,
			channelconfig.ConsensusTypeKey)
	}
	consensusType := &ab.ConsensusType{}
	if err := proto.Unmarshal(configValue.GetValue(), consensusType); err != nil {
		return nil, errors.Wrap(err, "failed to unmarshal the ConsensusType value")
	}
	if consensusType.GetType() != armaOrdererType {
		return nil, errors.Newf("the orderer type is %q, want %q", consensusType.GetType(), armaOrdererType)
	}
	sharedConfig := &ordererpb.SharedConfig{}
	if err := proto.Unmarshal(consensusType.GetMetadata(), sharedConfig); err != nil {
		return nil, errors.Wrap(err, "failed to unmarshal the shared config")
	}
	return &decodedConsensusType{
		configValue:   configValue,
		consensusType: consensusType,
		sharedConfig:  sharedConfig,
	}, nil
}

// encode writes the shared config, with any change made to it, back through the
// ConsensusType into the config value.
func (d *decodedConsensusType) encode() error {
	metadata, err := proto.Marshal(d.sharedConfig)
	if err != nil {
		return errors.Wrap(err, "failed to marshal the shared config")
	}
	d.consensusType.Metadata = metadata
	value, err := proto.Marshal(d.consensusType)
	if err != nil {
		return errors.Wrap(err, "failed to marshal the ConsensusType value")
	}
	d.configValue.Value = value
	return nil
}

// nodeName names the node a change selects, for messages.
func nodeName(nc *nodeChange) string {
	if nc.role == nodeRoleBatcher {
		return fmt.Sprintf("batcher of shard %d", nc.shard)
	}
	return nc.role
}
