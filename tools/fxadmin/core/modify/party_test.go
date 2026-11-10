/*
Copyright IBM Corp. All Rights Reserved.

SPDX-License-Identifier: Apache-2.0
*/

package modify_test

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"testing"

	cb "github.com/hyperledger/fabric-protos-go-apiv2/common"
	"github.com/stretchr/testify/require"
	"go.yaml.in/yaml/v3"
	"google.golang.org/protobuf/proto"

	"github.com/hyperledger/fabric-x-common/api/msppb"
	"github.com/hyperledger/fabric-x-common/api/ordererpb"
	"github.com/hyperledger/fabric-x-common/api/types"
	"github.com/hyperledger/fabric-x-common/common/channelconfig"
	"github.com/hyperledger/fabric-x-common/common/policies"
	"github.com/hyperledger/fabric-x-common/protolator"
	"github.com/hyperledger/fabric-x-common/protolator/protoext/ordererext"
	"github.com/hyperledger/fabric-x-common/protoutil"
	"github.com/hyperledger/fabric-x-common/tools/configtxgen"
	"github.com/hyperledger/fabric-x-common/tools/cryptogen"
	"github.com/hyperledger/fabric-x-common/tools/fxadmin/core/helpers"
	"github.com/hyperledger/fabric-x-common/tools/fxadmin/core/modify"
	"github.com/hyperledger/fabric-x-common/utils/test"
)

const (
	// existingOrdererOrg is the orderer org of party 1, the only party of the
	// block the tests edit.
	existingOrdererOrg = "orderer-org-1"
	// joiningOrdererOrg is the orderer org of the party the tests add.
	joiningOrdererOrg = "orderer-org-2"
	// joiningPartyID is MaxPartyID + 1 of the block the tests edit.
	joiningPartyID = 2
	// Hosts of the joining party's router and assembler.
	joiningRouterHost    = "router.org2"
	joiningAssemblerHost = "assembler.org2"
	// consenterNodeName names the consenter node of each generated org.
	consenterNodeName = "consenter"
	// nodeConfigField is the NodeConfig of a node config, and tlsCertField the
	// server TLS certificate field in it, as `fxadmin decode` renders them.
	nodeConfigField = "node_config"
	tlsCertField    = "server_tls_cert"
	// testChannelID is the channel of the config blocks the tests generate.
	testChannelID = "mychannel"
)

// TestPartyAdd adds a party and its orderer org from a party definition, and
// checks every place of the block that holds a party: the shared config's
// PartiesConfig and MaxPartyID, the Orderers consenter mapping, the orderer org
// group, and the BlockValidation policy. Nothing else in the config changes,
// and the edited block still loads as a channel config.
func TestPartyAdd(t *testing.T) {
	t.Parallel()

	baseBlock := newArmaConfigBlock(t)
	party, org := newJoiningParty(t)
	blockPath := writeNodeTestBlock(t, baseBlock)
	definitionDir, definition := renderPartyDefinition(t, party, joiningOrdererOrg, org)
	partyPath := writePartyDefinition(t, definitionDir, definition)

	require.NoError(t, modify.New().PartyAdd(partyPath, blockPath))

	editedBlock, err := os.ReadFile(blockPath)
	require.NoError(t, err)
	base := nodeTestOrderer(t, baseBlock)
	edited := nodeTestOrderer(t, editedBlock)

	// The shared config lists the party, and MaxPartyID is its PartyID.
	wantShared := nodeTestSharedConfig(t, base)
	wantShared.PartiesConfig = append(wantShared.PartiesConfig, party)
	wantShared.MaxPartyID = joiningPartyID
	test.RequireProtoEqual(t, wantShared, nodeTestSharedConfig(t, edited))

	// The consenter mapping lists the party's consenter node under its org's MSP ID.
	consenterNode := party.GetConsenterConfig().GetNodeConfig()
	wantConsenters := append(consentersFromGroup(t, base), &cb.Consenter{
		Id:            joiningPartyID,
		Host:          consenterNode.GetHost(),
		Port:          consenterNode.GetPort(),
		MspId:         joiningOrdererOrg,
		Identity:      consenterNode.GetSignCert(),
		ClientTlsCert: consenterNode.GetClientTlsCert(),
		ServerTlsCert: consenterNode.GetServerTlsCert(),
	})
	test.RequireProtoEqual(t,
		&cb.Orderers{ConsenterMapping: wantConsenters},
		&cb.Orderers{ConsenterMapping: consentersFromGroup(t, edited)})

	// The org is added exactly as defined.
	test.RequireProtoEqual(t, org, edited.GetGroups()[joiningOrdererOrg])

	// BlockValidation is a BFT quorum over all the consenters, with the same
	// mod_policy and version as before.
	requireBlockValidation(t, base, edited, wantConsenters)

	// Apart from those places, the whole config is unchanged.
	restored := nodeTestConfigOf(t, editedBlock)
	restoredOrderer := restored.GetChannelGroup().GetGroups()[channelconfig.OrdererGroupKey]
	restoredOrderer.Values[channelconfig.ConsensusTypeKey] = base.GetValues()[channelconfig.ConsensusTypeKey]
	restoredOrderer.Values[channelconfig.OrderersKey] = base.GetValues()[channelconfig.OrderersKey]
	restoredOrderer.Policies[policies.BlockValidationPolicyKey] = base.GetPolicies()[policies.BlockValidationPolicyKey]
	delete(restoredOrderer.Groups, joiningOrdererOrg)
	test.RequireProtoEqual(t, nodeTestConfigOf(t, baseBlock), restored)

	// The edited block loads as a channel config that includes the new org.
	material, err := channelconfig.LoadConfigBlockMaterialFromFile(blockPath)
	require.NoError(t, err)
	mspIDs := make([]string, 0, len(material.OrdererOrganizations))
	for _, ordererOrg := range material.OrdererOrganizations {
		mspIDs = append(mspIDs, ordererOrg.MspID)
	}
	require.ElementsMatch(t, []string{existingOrdererOrg, joiningOrdererOrg}, mspIDs)

	// The block was written in place.
	info, err := os.Stat(blockPath)
	require.NoError(t, err)
	require.Equal(t, os.FileMode(nodeTestBlockMode), info.Mode().Perm())
}

// TestPartyAddRejects covers party definitions that must fail, and asserts
// that a failed add leaves the block file unchanged.
func TestPartyAddRejects(t *testing.T) {
	t.Parallel()

	baseBlock := newArmaConfigBlock(t)
	party, org := newJoiningParty(t)

	// A well-formed party definition that the block rejects.
	for _, tc := range []struct {
		name                 string
		partyID              uint32
		orgName              string
		withoutConsenterNode bool
		withoutOrgMSP        bool
		wantErr              string
	}{
		{
			name:    "orderer org already in the block",
			partyID: joiningPartyID,
			orgName: existingOrdererOrg,
			wantErr: `orderer org "orderer-org-1" already exists`,
		},
		{
			name:    "missing org Name",
			partyID: joiningPartyID,
			orgName: "",
			wantErr: "the Org of party definition",
		},
		{
			name:                 "party without a consenter node",
			partyID:              joiningPartyID,
			orgName:              joiningOrdererOrg,
			withoutConsenterNode: true,
			wantErr:              "party 2 has no ConsenterConfig",
		},
		{
			name:          "org without an MSP",
			partyID:       joiningPartyID,
			orgName:       joiningOrdererOrg,
			withoutOrgMSP: true,
			wantErr:       "the edited config is not a valid channel config",
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			caseParty := proto.CloneOf(party)
			caseParty.PartyID = tc.partyID
			if tc.withoutConsenterNode {
				caseParty.ConsenterConfig = nil
			}
			caseOrg := proto.CloneOf(org)
			if tc.withoutOrgMSP {
				delete(caseOrg.Values, channelconfig.MSPKey)
			}
			blockPath := writeNodeTestBlock(t, baseBlock)
			definitionDir, definition := renderPartyDefinition(t, caseParty, tc.orgName, caseOrg)
			partyPath := writePartyDefinition(t, definitionDir, definition)

			err := modify.New().PartyAdd(partyPath, blockPath)
			require.ErrorContains(t, err, tc.wantErr)
			requireArmaBlockUnchanged(t, blockPath, baseBlock)
		})
	}

	// A malformed party definition.
	for _, tc := range []struct {
		name       string
		definition string
		wantErr    string
	}{
		{
			name:       "empty definition",
			definition: "",
			wantErr:    "has no Party",
		},
		{
			name:       "missing Party",
			definition: "Org: {Name: " + joiningOrdererOrg + "}\n",
			wantErr:    "has no Party",
		},
		{
			name:       "missing Org",
			definition: "Party: {PartyID: 2}\n",
			wantErr:    "has no Org",
		},
		{
			name:       "not YAML",
			definition: "Party: [2\n",
			wantErr:    "failed to parse party definition",
		},
		{
			name:       "Party key that is not a PartyConfig field",
			definition: "Party: {PartyID: 2, PartyName: two}\nOrg: {Name: " + joiningOrdererOrg + "}\n",
			wantErr:    "failed to decode the Party",
		},
		{
			name:       "Org key that is not a config group field",
			definition: "Party: {PartyID: 2}\nOrg: {Name: " + joiningOrdererOrg + ", polcies: {}}\n",
			wantErr:    "failed to decode the Org",
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			blockPath := writeNodeTestBlock(t, baseBlock)
			partyPath := writePartyDefinition(t, t.TempDir(), tc.definition)

			err := modify.New().PartyAdd(partyPath, blockPath)
			require.ErrorContains(t, err, tc.wantErr)
			requireArmaBlockUnchanged(t, blockPath, baseBlock)
		})
	}
}

// TestPartyAddRejectsInvalidHosts verifies that a party is not added if one of
// its nodes has a host that is not an IP address or a host name, as `modify
// party node --host` checks it, nor if its org lists an endpoint that does not
// parse or whose host is invalid.
func TestPartyAddRejectsInvalidHosts(t *testing.T) {
	t.Parallel()

	baseBlock := newArmaConfigBlock(t)
	party, org := newJoiningParty(t)

	// Hosts of the party's nodes.
	for _, tc := range []struct {
		name    string
		role    string
		host    string
		wantErr string
	}{
		{
			name:    "router host with an underscore",
			role:    nodeTestRouter,
			host:    "router_2.org2",
			wantErr: `the router of party 2: invalid host "router_2.org2"`,
		},
		{
			name:    "router host that is a malformed IPv4 address",
			role:    nodeTestRouter,
			host:    "300.1.1.1",
			wantErr: `the router of party 2: invalid host "300.1.1.1": not a valid IPv4 address`,
		},
		{
			name:    "batcher host with non-English letters",
			role:    nodeTestBatcher,
			host:    "ראוטר.org2",
			wantErr: `the batcher of shard 1 of party 2: invalid host "ראוטר.org2"`,
		},
		{
			name:    "consenter host with an IPv6 zone",
			role:    nodeTestConsenter,
			host:    "fe80::1%eth0",
			wantErr: `the consenter of party 2: invalid host "fe80::1%eth0"`,
		},
		{
			name:    "assembler host with a port",
			role:    nodeTestAssembler,
			host:    "assembler.org2:7061",
			wantErr: `the assembler of party 2: invalid host "assembler.org2:7061"`,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			caseParty := proto.CloneOf(party)
			setPartyTestNodeHost(t, caseParty, tc.role, tc.host)
			blockPath := writeNodeTestBlock(t, baseBlock)
			definitionDir, definition := renderPartyDefinition(t, caseParty, joiningOrdererOrg, org)
			partyPath := writePartyDefinition(t, definitionDir, definition)

			err := modify.New().PartyAdd(partyPath, blockPath)
			require.ErrorContains(t, err, tc.wantErr)
			requireArmaBlockUnchanged(t, blockPath, baseBlock)
		})
	}

	// Addresses of the org's Endpoints value.
	for _, tc := range []struct {
		name     string
		endpoint string
		wantErr  string
	}{
		{
			name:     "org endpoint with an empty label in its host",
			endpoint: "id=2,msp-id=orderer-org-2,broadcast,router..org2:7060",
			wantErr: `invalid orderer org "orderer-org-2": endpoint ` +
				`"id=2,msp-id=orderer-org-2,broadcast,router..org2:7060": invalid host "router..org2"`,
		},
		{
			name:     "org endpoint that does not parse",
			endpoint: "id=2,bogus=x,router.org2:7060",
			wantErr:  `invalid orderer org "orderer-org-2": failed to parse endpoint "id=2,bogus=x,router.org2:7060"`,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			caseOrg := proto.CloneOf(org)
			caseOrg.Values[channelconfig.EndpointsKey].Value = protoutil.MarshalOrPanic(
				&cb.OrdererAddresses{Addresses: []string{tc.endpoint}})
			blockPath := writeNodeTestBlock(t, baseBlock)
			definitionDir, definition := renderPartyDefinition(t, party, joiningOrdererOrg, caseOrg)
			partyPath := writePartyDefinition(t, definitionDir, definition)

			err := modify.New().PartyAdd(partyPath, blockPath)
			require.ErrorContains(t, err, tc.wantErr)
			requireArmaBlockUnchanged(t, blockPath, baseBlock)
		})
	}
}

// TestPartyAddReadsCertificateFiles verifies how the certificate and CRL paths
// of a party definition are read: an absolute path is used as is, each file is
// stored as the one PEM block it starts with (so a private key after a
// certificate is not stored), and the org's revocation list and OU identifier
// certificates are read from files too.
func TestPartyAddReadsCertificateFiles(t *testing.T) {
	t.Parallel()

	party, org := newJoiningParty(t)
	definitionDir, rendered := renderPartyDefinition(t, party, joiningOrdererOrg, org)
	certDir := t.TempDir()

	// The router's TLS certificate, by an absolute path, followed in its file by a private key.
	routerTLSCert := party.GetRouterConfig().GetNodeConfig().GetServerTlsCert()
	privateKey := []byte("-----BEGIN PRIVATE KEY-----\nU0VDUkVUS0VZ\n-----END PRIVATE KEY-----\n")
	routerTLSCertPath := filepath.Join(certDir, "router-tls-and-key.pem")
	require.NoError(t, os.WriteFile(routerTLSCertPath, slices.Concat(routerTLSCert, privateKey), 0o600))

	// A CRL and an OU identifier's certificate for the org's MSP, by paths
	// relative to the definition.
	crl := readNodeTestFile(t, "../../../../msp/testdata/revocation/crls/crl.pem")
	writeDefinitionFile(t, definitionDir, "msp/crl.pem", crl)
	caCert := nodeTestFabricMSPConfig(t, org).GetRootCerts()[0]
	writeDefinitionFile(t, definitionDir, "msp/ou-ca.pem", caCert)

	definition := parseDefinition(t, rendered)
	routerNode := childMap(t, childMap(t, childMap(t, definition, "Party"), "RouterConfig"), nodeConfigField)
	routerNode[tlsCertField] = routerTLSCertPath
	mspConfig, ok := mspConfigMapOf(childMap(t, definition, "Org"))
	require.True(t, ok)
	mspConfig["revocation_list"] = []any{"msp/crl.pem"}
	// The org's admin certificate is in the client OU, so the MSP accepts it.
	mspConfig["organizational_unit_identifiers"] = []any{
		map[string]any{"certificate": "msp/ou-ca.pem", "organizational_unit_identifier": "client"},
	}
	partyPath := writePartyDefinition(t, definitionDir, marshalDefinition(t, definition))
	blockPath := writeNodeTestBlock(t, newArmaConfigBlock(t))

	require.NoError(t, modify.New().PartyAdd(partyPath, blockPath))

	editedBlock, err := os.ReadFile(blockPath)
	require.NoError(t, err)
	require.NotContains(t, string(editedBlock), "PRIVATE KEY")
	edited := nodeTestOrderer(t, editedBlock)
	parties := nodeTestSharedConfig(t, edited).GetPartiesConfig()
	require.Equal(t, routerTLSCert, parties[len(parties)-1].GetRouterConfig().GetNodeConfig().GetServerTlsCert())
	editedMSP := nodeTestFabricMSPConfig(t, edited.GetGroups()[joiningOrdererOrg])
	require.Equal(t, [][]byte{crl}, editedMSP.GetRevocationList())
	require.Len(t, editedMSP.GetOrganizationalUnitIdentifiers(), 1)
	require.Equal(t, caCert, editedMSP.GetOrganizationalUnitIdentifiers()[0].GetCertificate())
}

// TestPartyAddRejectsCertificateFiles covers certificate paths that must fail,
// and asserts that a failed add leaves the block file unchanged.
func TestPartyAddRejectsCertificateFiles(t *testing.T) {
	t.Parallel()

	baseBlock := newArmaConfigBlock(t)
	party, org := newJoiningParty(t)

	for _, tc := range []struct {
		name string
		// fileContent is written as the router's TLS certificate file; nil leaves no file.
		fileContent []byte
		// pathValue replaces the router's server_tls_cert path, when set.
		pathValue any
		wantErr   string
	}{
		{
			name:    "missing file",
			wantErr: "Party: RouterConfig: node_config: server_tls_cert: failed to read",
		},
		{
			name:        "file that is not PEM",
			fileContent: []byte("not a certificate"),
			wantErr:     "holds no PEM block",
		},
		{
			name:      "a number where a path is expected",
			pathValue: 5,
			wantErr:   "server_tls_cert: want a file path or a list of file paths",
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			definitionDir, rendered := renderPartyDefinition(t, party, joiningOrdererOrg, org)
			definition := parseDefinition(t, rendered)
			router := childMap(t, childMap(t, childMap(t, definition, "Party"), "RouterConfig"), nodeConfigField)
			router[tlsCertField] = "router-tls.pem"
			if tc.fileContent != nil {
				writeDefinitionFile(t, definitionDir, "router-tls.pem", tc.fileContent)
			}
			if tc.pathValue != nil {
				router[tlsCertField] = tc.pathValue
			}
			partyPath := writePartyDefinition(t, definitionDir, marshalDefinition(t, definition))
			blockPath := writeNodeTestBlock(t, baseBlock)

			err := modify.New().PartyAdd(partyPath, blockPath)
			require.ErrorContains(t, err, tc.wantErr)
			requireArmaBlockUnchanged(t, blockPath, baseBlock)
		})
	}
}

// TestPartyAddRejectsInvalidOrgFiles verifies that an org MSP file that is not
// a valid certificate or CRL is rejected when the edited config is checked, by
// the MSP, and the block file is left unchanged.
func TestPartyAddRejectsInvalidOrgFiles(t *testing.T) {
	t.Parallel()

	baseBlock := newArmaConfigBlock(t)
	party, org := newJoiningParty(t)
	privateKey := []byte("-----BEGIN PRIVATE KEY-----\nU0VDUkVUS0VZ\n-----END PRIVATE KEY-----\n")
	crl := readNodeTestFile(t, "../../../../msp/testdata/revocation/crls/crl.pem")
	caCert := nodeTestFabricMSPConfig(t, org).GetRootCerts()[0]
	// errNotACertificate is how the MSP rejects a file that is not a certificate.
	errNotACertificate := "failed to parse x509 cert"

	for _, tc := range []struct {
		name        string
		field       string
		fileContent []byte
		wantErr     string
	}{
		{
			name:        "private key as a root certificate",
			field:       "root_certs",
			fileContent: privateKey,
			wantErr:     errNotACertificate,
		},
		{
			name:        "CRL as an admin certificate",
			field:       "admins",
			fileContent: crl,
			wantErr:     errNotACertificate,
		},
		{
			name:        "certificate as a CRL",
			field:       "revocation_list",
			fileContent: caCert,
			wantErr:     "could not parse RevocationList",
		},
		{
			name:        "private key as an intermediate certificate",
			field:       "intermediate_certs",
			fileContent: privateKey,
			wantErr:     errNotACertificate,
		},
		{
			name:        "private key as a TLS intermediate certificate",
			field:       "tls_intermediate_certs",
			fileContent: privateKey,
			wantErr:     errNotACertificate,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			definitionDir, rendered := renderPartyDefinition(t, party, joiningOrdererOrg, org)
			definition := parseDefinition(t, rendered)
			writeDefinitionFile(t, definitionDir, "msp/file.pem", tc.fileContent)
			mspConfig, ok := mspConfigMapOf(childMap(t, definition, "Org"))
			require.True(t, ok)
			mspConfig[tc.field] = []any{"msp/file.pem"}
			partyPath := writePartyDefinition(t, definitionDir, marshalDefinition(t, definition))
			blockPath := writeNodeTestBlock(t, baseBlock)

			err := modify.New().PartyAdd(partyPath, blockPath)
			require.ErrorContains(t, err, "the edited config is not a valid channel config")
			require.ErrorContains(t, err, tc.wantErr)
			requireArmaBlockUnchanged(t, blockPath, baseBlock)
		})
	}
}

// TestPartyAddRejectsNonConfigBlock verifies that a block file that is not a
// config block is rejected without being rewritten.
func TestPartyAddRejectsNonConfigBlock(t *testing.T) {
	t.Parallel()

	notABlock := []byte("not a block")
	blockPath := writeNodeTestBlock(t, notABlock)
	party, org := newJoiningParty(t)
	definitionDir, definition := renderPartyDefinition(t, party, joiningOrdererOrg, org)
	partyPath := writePartyDefinition(t, definitionDir, definition)

	err := modify.New().PartyAdd(partyPath, blockPath)
	require.ErrorContains(t, err, "failed to unmarshal config block")
	requireArmaBlockUnchanged(t, blockPath, notABlock)
}

// newArmaConfigBlock returns a marshaled ARMA config block with one party,
// party 1 of existingOrdererOrg, in both its shared config and its consenter
// mapping; its MaxPartyID is 1.
func newArmaConfigBlock(t *testing.T) []byte {
	t.Helper()
	shared, err := proto.Marshal(&ordererpb.SharedConfig{
		PartiesConfig: []*ordererpb.PartyConfig{{
			PartyID: 1,
			ConsenterConfig: &ordererpb.ConsenterNodeConfig{
				NodeConfig: &ordererpb.NodeConfig{Host: consenterNodeName, Port: 8080},
			},
		}},
		MaxPartyID: 1,
	})
	require.NoError(t, err)
	block, err := cryptogen.CreateOrExtendConfigBlockWithCrypto(cryptogen.ConfigBlockParameters{
		TargetPath:  t.TempDir(),
		BaseProfile: configtxgen.SampleFabricX,
		ChannelID:   testChannelID,
		Organizations: []cryptogen.OrganizationParameters{{
			Name:             existingOrdererOrg,
			Domain:           existingOrdererOrg + ".com",
			OrdererEndpoints: []*types.OrdererEndpoint{{ID: 1, Host: "localhost", Port: 7050}},
			ConsenterNodes:   []cryptogen.Node{{CommonName: consenterNodeName, Hostname: consenterNodeName}},
			OrdererNodes:     []cryptogen.Node{{CommonName: "orderer-node", Hostname: "orderer-node"}},
		}},
		ArmaMetaBytes: shared,
	})
	require.NoError(t, err)

	// A genesis block's versions are all 0. Give BlockValidation another
	// version, so a test can tell that the edit keeps it.
	configBlock, err := helpers.DecodeConfigBlock(block)
	require.NoError(t, err)
	orderer := configBlock.Config().GetChannelGroup().GetGroups()[channelconfig.OrdererGroupKey]
	orderer.GetPolicies()[policies.BlockValidationPolicyKey].Version = nodeTestPolicyVersion
	require.NoError(t, configBlock.Encode())
	return protoutil.MarshalOrPanic(configBlock.Block)
}

// newJoiningParty returns the shared config entry and the orderer org group of
// a party joining as party joiningPartyID with joiningOrdererOrg. They are
// taken from a config block generated for that org alone: the org group as
// that block stores it, with its endpoints, and the party's certificates from
// the org's MSP and consenter.
func newJoiningParty(t *testing.T) (*ordererpb.PartyConfig, *cb.ConfigGroup) {
	t.Helper()
	block, err := cryptogen.CreateOrExtendConfigBlockWithCrypto(cryptogen.ConfigBlockParameters{
		TargetPath:  t.TempDir(),
		BaseProfile: configtxgen.SampleFabricX,
		ChannelID:   testChannelID,
		Organizations: []cryptogen.OrganizationParameters{{
			Name:   joiningOrdererOrg,
			Domain: joiningOrdererOrg + ".com",
			OrdererEndpoints: []*types.OrdererEndpoint{
				{ID: joiningPartyID, Host: joiningRouterHost, Port: 7060, API: []string{types.Broadcast}},
				{ID: joiningPartyID, Host: joiningAssemblerHost, Port: 7061, API: []string{types.Deliver}},
			},
			ConsenterNodes: []cryptogen.Node{{CommonName: consenterNodeName, Hostname: "consenter.org2"}},
			OrdererNodes: []cryptogen.Node{
				{CommonName: "router", Hostname: joiningRouterHost},
				{CommonName: "assembler", Hostname: joiningAssemblerHost},
			},
		}},
	})
	require.NoError(t, err)
	orderer := nodeTestOrderer(t, protoutil.MarshalOrPanic(block))
	org := orderer.GetGroups()[joiningOrdererOrg]
	require.NotNil(t, org)
	consenters := consentersFromGroup(t, orderer)
	require.Len(t, consenters, 1)
	consenter := consenters[0]
	orgMSP := nodeTestFabricMSPConfig(t, org)

	certs := &ordererpb.NodeConfig{
		ClientTlsCert: consenter.GetClientTlsCert(),
		ServerTlsCert: consenter.GetServerTlsCert(),
		SignCert:      consenter.GetIdentity(),
	}
	return &ordererpb.PartyConfig{
		PartyID:      joiningPartyID,
		CACerts:      orgMSP.GetRootCerts(),
		TLSCACerts:   orgMSP.GetTlsRootCerts(),
		RouterConfig: &ordererpb.RouterNodeConfig{NodeConfig: joiningNodeConfig(certs, joiningRouterHost, 7060)},
		BatchersConfig: []*ordererpb.BatcherNodeConfig{{
			NodeConfig: joiningNodeConfig(certs, "batcher1.org2", 7062), ShardId: 1,
		}},
		ConsenterConfig: &ordererpb.ConsenterNodeConfig{
			NodeConfig: joiningNodeConfig(certs, consenter.GetHost(), consenter.GetPort()),
		},
		AssemblerConfig: &ordererpb.AssemblerNodeConfig{
			NodeConfig: joiningNodeConfig(certs, joiningAssemblerHost, 7061),
		},
	}, org
}

// joiningNodeConfig returns a node config of the joining party at host and
// port, with the certificates of certs.
func joiningNodeConfig(certs *ordererpb.NodeConfig, host string, port uint32) *ordererpb.NodeConfig {
	node := proto.CloneOf(certs)
	node.Host, node.Port = host, port
	return node
}

// renderPartyDefinition renders a party definition as YAML in a fresh
// directory, and returns the directory and the definition. The party and the
// org group are in the protolator form `fxadmin decode` renders them, the org
// with its Name key, and each certificate is written to a file in the directory
// and given by its path relative to the directory, as a party definition there
// names them. An empty orgName leaves the Name key out.
func renderPartyDefinition(
	t *testing.T, party *ordererpb.PartyConfig, orgName string, org *cb.ConfigGroup,
) (dir, definition string) {
	t.Helper()
	dir = t.TempDir()
	partyMap := protolatorMapOf(t, party)
	orgMap := protolatorMapOf(t, &ordererext.DynamicOrdererOrgGroup{ConfigGroup: org})
	files := &certificateFiles{t: t, dir: dir}
	files.replaceWithPaths(partyMap, "CACerts", "TLSCACerts")
	nodeMaps := make([]map[string]any, 0, 4)
	for _, node := range []string{"RouterConfig", "ConsenterConfig", "AssemblerConfig"} {
		if nodeMap, ok := partyMap[node].(map[string]any); ok {
			nodeMaps = append(nodeMaps, nodeMap)
		}
	}
	if batchers, ok := partyMap["BatchersConfig"].([]any); ok {
		for _, batcher := range batchers {
			batcherMap, ok := batcher.(map[string]any)
			require.True(t, ok)
			nodeMaps = append(nodeMaps, batcherMap)
		}
	}
	for _, nodeMap := range nodeMaps {
		files.replaceWithPaths(childMap(t, nodeMap, nodeConfigField), "client_tls_cert", tlsCertField, "sign_cert")
	}
	if mspConfig, ok := mspConfigMapOf(orgMap); ok {
		files.replaceWithPaths(mspConfig, "root_certs", "intermediate_certs", "admins",
			"tls_root_certs", "tls_intermediate_certs", "known_certs")
	}
	if orgName != "" {
		orgMap["Name"] = orgName
	}
	content, err := yaml.Marshal(map[string]any{
		"Party": partyMap,
		"Org":   orgMap,
	})
	require.NoError(t, err)
	return dir, string(content)
}

// certificateFiles writes the certificates of a rendered party definition to
// files in dir, one file per certificate.
type certificateFiles struct {
	t     *testing.T
	dir   string
	count int
}

// replaceWithPaths replaces the base64 certificate, or each certificate of the
// list, held by each of the given fields of holder with the path, relative to
// dir, of a file the certificate is written to.
func (f *certificateFiles) replaceWithPaths(holder map[string]any, fields ...string) {
	f.t.Helper()
	for _, field := range fields {
		switch value := holder[field].(type) {
		case string:
			holder[field] = f.write(value)
		case []any:
			for i, cert := range value {
				base64Cert, ok := cert.(string)
				require.True(f.t, ok)
				value[i] = f.write(base64Cert)
			}
		default:
			// The field is not set, so it holds no certificate.
		}
	}
}

// write writes a base64 certificate, as protolator renders it, to a new file in
// a subdirectory of dir and returns the file's path relative to dir.
func (f *certificateFiles) write(base64Cert string) string {
	f.t.Helper()
	cert, err := base64.StdEncoding.DecodeString(base64Cert)
	require.NoError(f.t, err)
	f.count++
	relPath := filepath.Join("certs", fmt.Sprintf("cert-%d.pem", f.count))
	require.NoError(f.t, os.MkdirAll(filepath.Join(f.dir, "certs"), 0o750))
	require.NoError(f.t, os.WriteFile(filepath.Join(f.dir, relPath), cert, 0o600))
	return relPath
}

// parseDefinition parses a rendered party definition, so a test can change it.
func parseDefinition(t *testing.T, definition string) map[string]any {
	t.Helper()
	parsed := map[string]any{}
	require.NoError(t, yaml.Unmarshal([]byte(definition), &parsed))
	return parsed
}

// marshalDefinition renders a parsed party definition back as YAML.
func marshalDefinition(t *testing.T, definition map[string]any) string {
	t.Helper()
	content, err := yaml.Marshal(definition)
	require.NoError(t, err)
	return string(content)
}

// childMap returns the map at key of parent.
func childMap(t *testing.T, parent map[string]any, key string) map[string]any {
	t.Helper()
	child, ok := parent[key].(map[string]any)
	require.True(t, ok, "%q is not a map", key)
	return child
}

// writeDefinitionFile writes content as the file at relPath in dir, creating
// its directory.
func writeDefinitionFile(t *testing.T, dir, relPath string, content []byte) {
	t.Helper()
	path := filepath.Join(dir, relPath)
	require.NoError(t, os.MkdirAll(filepath.Dir(path), 0o750))
	require.NoError(t, os.WriteFile(path, content, 0o600))
}

// mspConfigMapOf returns the Fabric MSP config map of a rendered org group.
func mspConfigMapOf(org map[string]any) (map[string]any, bool) {
	values, ok := org["values"].(map[string]any)
	if !ok {
		return nil, false
	}
	mspValue, ok := values["MSP"].(map[string]any)
	if !ok {
		return nil, false
	}
	value, ok := mspValue["value"].(map[string]any)
	if !ok {
		return nil, false
	}
	config, ok := value["config"].(map[string]any)
	return config, ok
}

// protolatorMapOf returns msg in the protolator JSON form, as a parsed map.
func protolatorMapOf(t *testing.T, msg proto.Message) map[string]any {
	t.Helper()
	var rendered bytes.Buffer
	require.NoError(t, protolator.DeepMarshalJSON(&rendered, msg))
	parsed := map[string]any{}
	require.NoError(t, json.Unmarshal(rendered.Bytes(), &parsed))
	return parsed
}

// writePartyDefinition writes the definition as party.yaml in dir and returns
// its path.
func writePartyDefinition(t *testing.T, dir, definition string) string {
	t.Helper()
	path := filepath.Join(dir, "party.yaml")
	require.NoError(t, os.WriteFile(path, []byte(definition), 0o600))
	return path
}

// consentersFromGroup returns the consenter mapping of an Orderer group.
func consentersFromGroup(t *testing.T, orderer *cb.ConfigGroup) []*cb.Consenter {
	t.Helper()
	orderers := &cb.Orderers{}
	require.NoError(t, proto.Unmarshal(orderer.GetValues()[channelconfig.OrderersKey].GetValue(), orderers))
	return orderers.GetConsenterMapping()
}

// requireBlockValidation asserts that the edited Orderer group's BlockValidation
// policy is a BFT quorum of signatures, one rule per consenter, over exactly the
// consenters' identities, and keeps the base policy's mod_policy and version.
func requireBlockValidation(t *testing.T, base, edited *cb.ConfigGroup, consenters []*cb.Consenter) {
	t.Helper()
	basePolicy := base.GetPolicies()[policies.BlockValidationPolicyKey]
	policy := edited.GetPolicies()[policies.BlockValidationPolicyKey]
	require.Equal(t, basePolicy.GetModPolicy(), policy.GetModPolicy())
	require.Equal(t, basePolicy.GetVersion(), policy.GetVersion())
	require.Equal(t, int32(cb.Policy_SIGNATURE), policy.GetPolicy().GetType())

	envelope := &cb.SignaturePolicyEnvelope{}
	require.NoError(t, proto.Unmarshal(policy.GetPolicy().GetValue(), envelope))
	n := len(consenters)
	nOutOf := envelope.GetRule().GetNOutOf()
	require.EqualValues(t, policies.ComputeBFTQuorum(n, (n-1)/3), nOutOf.GetN())
	require.Len(t, nOutOf.GetRules(), n)

	want := make([]string, 0, n)
	for _, consenter := range consenters {
		identity := msppb.NewIdentity(consenter.GetMspId(), consenter.GetIdentity())
		want = append(want, string(protoutil.MarshalOrPanic(identity)))
	}
	got := make([]string, 0, len(envelope.GetIdentities()))
	for _, identity := range envelope.GetIdentities() {
		got = append(got, string(identity.GetPrincipal()))
	}
	require.ElementsMatch(t, want, got)
}

// setPartyTestNodeHost sets the host of the party's node of the given role, its
// first batcher for a batcher.
func setPartyTestNodeHost(t *testing.T, party *ordererpb.PartyConfig, role, host string) {
	t.Helper()
	switch role {
	case nodeTestRouter:
		party.RouterConfig.NodeConfig.Host = host
	case nodeTestBatcher:
		require.NotEmpty(t, party.GetBatchersConfig())
		party.BatchersConfig[0].NodeConfig.Host = host
	case nodeTestConsenter:
		party.ConsenterConfig.NodeConfig.Host = host
	case nodeTestAssembler:
		party.AssemblerConfig.NodeConfig.Host = host
	default:
		require.Failf(t, "unknown node role", "role %q", role)
	}
}

// requireArmaBlockUnchanged asserts that the block file at path still holds want.
func requireArmaBlockUnchanged(t *testing.T, path string, want []byte) {
	t.Helper()
	content, err := os.ReadFile(path)
	require.NoError(t, err)
	require.Equal(t, want, content, "a failed add must leave the block unchanged")
}
