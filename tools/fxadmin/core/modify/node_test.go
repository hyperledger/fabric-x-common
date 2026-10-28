/*
Copyright IBM Corp. All Rights Reserved.

SPDX-License-Identifier: Apache-2.0
*/

package modify_test

import (
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"testing"

	cb "github.com/hyperledger/fabric-protos-go-apiv2/common"
	mspproto "github.com/hyperledger/fabric-protos-go-apiv2/msp"
	ab "github.com/hyperledger/fabric-protos-go-apiv2/orderer"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/proto"

	"github.com/hyperledger/fabric-x-common/api/msppb"
	"github.com/hyperledger/fabric-x-common/api/ordererpb"
	"github.com/hyperledger/fabric-x-common/api/types"
	"github.com/hyperledger/fabric-x-common/common/channelconfig"
	"github.com/hyperledger/fabric-x-common/common/policies"
	"github.com/hyperledger/fabric-x-common/protoutil"
	"github.com/hyperledger/fabric-x-common/tools/configtxgen"
	"github.com/hyperledger/fabric-x-common/tools/cryptogen"
	"github.com/hyperledger/fabric-x-common/tools/fxadmin/core/helpers"
	"github.com/hyperledger/fabric-x-common/tools/fxadmin/core/modify"
	"github.com/hyperledger/fabric-x-common/tools/fxadmin/core/modify/change"
	"github.com/hyperledger/fabric-x-common/utils/test"
)

const (
	// nodeTestOrg is the orderer org of nodeTestParty, the only party of the
	// block the tests edit.
	nodeTestOrg   = "orderer-org-1"
	nodeTestParty = 1

	// Endpoints of the party's router and assembler.
	nodeTestRouterHost    = "router.org1"
	nodeTestRouterPort    = 7050
	nodeTestAssemblerHost = "assembler.org1"
	nodeTestAssemblerPort = 7060
	// The party's broadcast (router) and deliver (assembler) endpoints, as its
	// org's Endpoints value lists them.
	nodeTestBroadcastEndpoint = "id=1,msp-id=orderer-org-1,broadcast,router.org1:7050"
	nodeTestDeliverEndpoint   = "id=1,msp-id=orderer-org-1,deliver,assembler.org1:7060"

	// Node roles, as `--role` names them.
	nodeTestRouter    = "router"
	nodeTestBatcher   = "batcher"
	nodeTestConsenter = "consenter"
	nodeTestAssembler = "assembler"

	// nodeTestSpareNode is an extra node of nodeTestOrg that no party entry
	// lists, whose certificates are issued by the org's CAs, for use as a node's
	// new certificates.
	nodeTestSpareNode = "spare"

	nodeTestBlockFile = "next.pb"
	nodeTestBlockMode = 0o640
	// nodeTestPolicyVersion is the BlockValidation version of the block the
	// tests edit; not 0, so a test can tell that an edit keeps it.
	nodeTestPolicyVersion = 3
)

// nodeTestConfig is the endpoint and certificate getters that the shared config
// entries of all four node roles have.
type nodeTestConfig interface {
	proto.Message
	GetHost() string
	GetPort() uint32
	GetTlsCert() []byte
	GetSignCert() []byte
}

// nodeTestFields is a node's endpoint and certificates.
type nodeTestFields struct {
	host     string
	port     uint32
	tlsCert  []byte
	signCert []byte
}

// nodeTestCase is a TestPartyNode case: the node to change, the values to set,
// and the org's endpoints after the change.
type nodeTestCase struct {
	name          string
	role          string
	shard         uint32
	host          string
	port          uint32
	tlsCert       bool
	signCert      bool
	wantEndpoints []string
}

// nodeTestCertFiles is the new certificates TestPartyNode sets, with the files
// that hold them.
type nodeTestCertFiles struct {
	tlsCert, signCert         []byte
	tlsCertPath, signCertPath string
}

// TestPartyNode changes one node of a party and checks each place of the block
// that holds the node: the node's entry in the shared config, where only the
// given fields change; the consenter mapping and the BlockValidation policy,
// which follow a consenter; and the org's endpoints, which follow the router and
// the assembler. Nothing else in the config changes.
func TestPartyNode(t *testing.T) {
	t.Parallel()

	material := newNodeTestMaterial(t)
	baseBlock := material.block
	certs := &nodeTestCertFiles{tlsCert: material.newTLSCert, signCert: material.newSignCert}
	certDir := t.TempDir()
	certs.tlsCertPath = writeNodeTestFile(t, certDir, "tls-cert.pem", certs.tlsCert)
	certs.signCertPath = writeNodeTestFile(t, certDir, "sign-cert.pem", certs.signCert)
	baseEndpoints := []string{
		nodeTestBroadcastEndpoint,
		nodeTestDeliverEndpoint,
	}

	for _, tc := range []nodeTestCase{
		{
			name: "router endpoint", role: nodeTestRouter, host: "router2.org1", port: 7150,
			wantEndpoints: []string{
				"id=1,msp-id=orderer-org-1,broadcast,router2.org1:7150",
				nodeTestDeliverEndpoint,
			},
		},
		{
			name: "router certificates", role: nodeTestRouter, tlsCert: true, signCert: true,
			wantEndpoints: baseEndpoints,
		},
		{
			name: "every field of the batcher of shard 2", role: nodeTestBatcher, shard: 2,
			host: "batcher2b.org1", port: 7172, tlsCert: true, signCert: true,
			wantEndpoints: baseEndpoints,
		},
		{
			name: "consenter endpoint", role: nodeTestConsenter, host: "consenter2.org1", port: 7152,
			wantEndpoints: baseEndpoints,
		},
		{
			name: "consenter TLS certificate", role: nodeTestConsenter, tlsCert: true,
			wantEndpoints: baseEndpoints,
		},
		{
			name: "consenter signing certificate", role: nodeTestConsenter, signCert: true,
			wantEndpoints: baseEndpoints,
		},
		{
			name: "assembler port only", role: nodeTestAssembler, port: 7160,
			wantEndpoints: []string{
				nodeTestBroadcastEndpoint,
				"id=1,msp-id=orderer-org-1,deliver,assembler.org1:7160",
			},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			blockPath := writeNodeTestBlock(t, baseBlock)
			base := nodeTestOrderer(t, baseBlock)
			current := nodeTestFieldsOf(nodeTestNode(t, nodeTestSharedConfig(t, base), tc.role, tc.shard))
			request, want := nodeTestRequestOf(&tc, blockPath, current, certs)

			require.NoError(t, modify.New().PartyNode(request))

			editedBlock, err := os.ReadFile(blockPath)
			require.NoError(t, err)
			edited := nodeTestOrderer(t, editedBlock)

			// The node's shared config entry has the new values; the rest of the
			// shared config is unchanged.
			baseShared, editedShared := nodeTestSharedConfig(t, base), nodeTestSharedConfig(t, edited)
			editedNode := nodeTestNode(t, editedShared, tc.role, tc.shard)
			require.Equal(t, want, nodeTestFieldsOf(editedNode))
			proto.Reset(editedNode)
			proto.Merge(editedNode, nodeTestNode(t, baseShared, tc.role, tc.shard))
			test.RequireProtoEqual(t, baseShared, editedShared)

			// The consenter mapping follows the consenter.
			wantConsenter := wantNodeTestConsenter(t, base, tc.role, want)
			test.RequireProtoEqual(t, wantConsenter, nodeTestConsenterOf(t, edited))

			// BlockValidation follows the consenter's signing certificate.
			requireNodeTestBlockValidation(t, edited, wantConsenter)

			// The org's endpoints follow the router and the assembler.
			require.Equal(t, tc.wantEndpoints, nodeTestEndpoints(t, edited))

			// Apart from those places, the whole config is unchanged.
			requireNodeTestUnchangedElsewhere(t, baseBlock, editedBlock)

			// The block was written in place.
			info, err := os.Stat(blockPath)
			require.NoError(t, err)
			require.Equal(t, os.FileMode(nodeTestBlockMode), info.Mode().Perm())
		})
	}
}

// TestPartyNodeMovesOnlyThePartysEndpoint verifies that moving a router changes
// only the party's own broadcast endpoint when its org also lists the endpoints
// of another party.
func TestPartyNodeMovesOnlyThePartysEndpoint(t *testing.T) {
	t.Parallel()

	otherBroadcast := "id=2,msp-id=orderer-org-1,broadcast,router.party2:7055"
	otherDeliver := "id=2,msp-id=orderer-org-1,deliver,assembler.party2:7065"
	block := withNodeTestEndpoints(t, newNodeTestMaterial(t).block,
		nodeTestBroadcastEndpoint,
		nodeTestDeliverEndpoint,
		otherBroadcast,
		otherDeliver,
	)
	blockPath := writeNodeTestBlock(t, block)

	err := modify.New().PartyNode(change.Node{
		Party: nodeTestParty, Role: nodeTestRouter, Port: 7150, BlockPath: blockPath,
	})
	require.NoError(t, err)

	edited, err := os.ReadFile(blockPath)
	require.NoError(t, err)
	require.Equal(t, []string{
		"id=1,msp-id=orderer-org-1,broadcast,router.org1:7150",
		nodeTestDeliverEndpoint,
		otherBroadcast,
		otherDeliver,
	}, nodeTestEndpoints(t, nodeTestOrderer(t, edited)))
}

// TestPartyNodeStoresOnlyTheCertificate verifies that when the TLS certificate
// file also holds the certificate's private key, another certificate, and some
// text, only the TLS certificate is stored in the block.
func TestPartyNodeStoresOnlyTheCertificate(t *testing.T) {
	t.Parallel()

	material := newNodeTestMaterial(t)
	tlsCert, signCert := material.newTLSCert, material.newSignCert
	privateKey := []byte("-----BEGIN PRIVATE KEY-----\nU0VDUkVUS0VZ\n-----END PRIVATE KEY-----\n")
	extraText := []byte("extra text\n")
	tlsCertFile := slices.Concat(tlsCert, privateKey, signCert, extraText)
	tlsCertPath := writeNodeTestFile(t, t.TempDir(), "tls-cert-and-key.pem", tlsCertFile)
	blockPath := writeNodeTestBlock(t, material.block)

	err := modify.New().PartyNode(change.Node{
		Party: nodeTestParty, Role: nodeTestConsenter, TLSCert: tlsCertPath, BlockPath: blockPath,
	})
	require.NoError(t, err)

	editedBlock, err := os.ReadFile(blockPath)
	require.NoError(t, err)
	require.NotContains(t, string(editedBlock), "PRIVATE KEY")
	require.NotContains(t, string(editedBlock), "extra text")
	orderer := nodeTestOrderer(t, editedBlock)
	consenterNode := nodeTestNode(t, nodeTestSharedConfig(t, orderer), nodeTestConsenter, 0)
	require.Equal(t, tlsCert, consenterNode.GetTlsCert())
	consenterMapping := nodeTestConsenterOf(t, orderer)
	require.Equal(t, tlsCert, consenterMapping.GetClientTlsCert())
	require.Equal(t, tlsCert, consenterMapping.GetServerTlsCert())
}

// TestPartyNodeChangesBothAddresses verifies that a router change sets the new
// host and port both in the router's shared config entry and in its broadcast
// endpoint, and that each sets only what it is given: --port alone keeps the
// host, and --host alone keeps the port. The base endpoint has another host and
// port than the shared config entry (router.org1:7050), so a test can tell
// which part of the endpoint was changed.
func TestPartyNodeChangesBothAddresses(t *testing.T) {
	t.Parallel()

	baseBlock := withNodeTestEndpoints(t, newNodeTestMaterial(t).block,
		"id=1,msp-id=orderer-org-1,broadcast,router.public:443", nodeTestDeliverEndpoint)

	for _, tc := range []struct {
		name          string
		newHost       string
		newPort       uint32
		wantRouter    nodeTestFields
		wantBroadcast string
	}{
		{
			name:          "port only",
			newPort:       8443,
			wantRouter:    nodeTestFields{host: nodeTestRouterHost, port: 8443},
			wantBroadcast: "id=1,msp-id=orderer-org-1,broadcast,router.public:8443",
		},
		{
			name:          "host only",
			newHost:       "router2.org1",
			wantRouter:    nodeTestFields{host: "router2.org1", port: nodeTestRouterPort},
			wantBroadcast: "id=1,msp-id=orderer-org-1,broadcast,router2.org1:443",
		},
		{
			name:          "host and port",
			newHost:       "router2.org1",
			newPort:       8443,
			wantRouter:    nodeTestFields{host: "router2.org1", port: 8443},
			wantBroadcast: "id=1,msp-id=orderer-org-1,broadcast,router2.org1:8443",
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			blockPath := writeNodeTestBlock(t, baseBlock)
			err := modify.New().PartyNode(change.Node{
				Party: nodeTestParty, Role: nodeTestRouter, Host: tc.newHost, Port: tc.newPort, BlockPath: blockPath,
			})
			require.NoError(t, err)

			editedBlock, err := os.ReadFile(blockPath)
			require.NoError(t, err)
			edited := nodeTestOrderer(t, editedBlock)
			router := nodeTestNode(t, nodeTestSharedConfig(t, edited), nodeTestRouter, 0)
			require.Equal(t, tc.wantRouter.host, router.GetHost())
			require.Equal(t, tc.wantRouter.port, router.GetPort())
			require.Equal(t, []string{tc.wantBroadcast, nodeTestDeliverEndpoint}, nodeTestEndpoints(t, edited))
		})
	}
}

// TestPartyNodeIsDeterministic verifies that the same change to the same block
// gives the same bytes on every run, so an edited block can be diffed and
// hashed.
func TestPartyNodeIsDeterministic(t *testing.T) {
	t.Parallel()

	baseBlock := newNodeTestMaterial(t).block
	var firstEditedBlock []byte
	for range 5 {
		blockPath := writeNodeTestBlock(t, baseBlock)
		err := modify.New().PartyNode(change.Node{
			Party: nodeTestParty, Role: nodeTestRouter, Port: 7150, BlockPath: blockPath,
		})
		require.NoError(t, err)
		editedBlock, err := os.ReadFile(blockPath)
		require.NoError(t, err)
		if firstEditedBlock == nil {
			firstEditedBlock = editedBlock
			continue
		}
		require.Equal(t, firstEditedBlock, editedBlock)
	}
}

// TestPartyNodeRejects covers changes that must fail, and asserts that a failed
// change leaves the block file unchanged.
func TestPartyNodeRejects(t *testing.T) {
	t.Parallel()

	baseBlock := newNodeTestMaterial(t).block
	certDir := t.TempDir()
	missingPath := filepath.Join(certDir, "none.pem")
	notPEMPath := writeNodeTestFile(t, certDir, "not-pem.pem", []byte("not a certificate"))
	keyPath := writeNodeTestFile(t, certDir, "key.pem",
		[]byte("-----BEGIN PRIVATE KEY-----\nAAAA\n-----END PRIVATE KEY-----\n"))
	// A block whose party's router and assembler share one endpoint serving both APIs.
	sharedEndpointBlock := withNodeTestEndpoints(t, baseBlock, "id=1,msp-id=orderer-org-1,router.org1:7050")
	// A block whose org lists no broadcast endpoint of the party.
	noBroadcastBlock := withNodeTestEndpoints(t, baseBlock, nodeTestDeliverEndpoint)
	// A block whose org lists no deliver endpoint of the party.
	noDeliverBlock := withNodeTestEndpoints(t, baseBlock, nodeTestBroadcastEndpoint)
	// A block whose org lists, next to the party's endpoints, an address with an
	// unknown key ("bogus=x") that cannot be parsed.
	malformedEndpoint := "id=1,bogus=x,router.org1:7050"
	malformedEndpointBlock := withNodeTestEndpoints(t, baseBlock,
		nodeTestBroadcastEndpoint, nodeTestDeliverEndpoint, malformedEndpoint)

	for _, tc := range []struct {
		name    string
		block   []byte
		request change.Node
		wantErr string
	}{
		{
			name:    "no field to change",
			block:   baseBlock,
			request: change.Node{Party: nodeTestParty, Role: nodeTestRouter},
			wantErr: "nothing to change",
		},
		{
			name:    "party not in the block",
			block:   baseBlock,
			request: change.Node{Party: 9, Role: nodeTestRouter, Port: 7150},
			wantErr: "the shared config has no party 9",
		},
		{
			name:    "batcher shard not in the party",
			block:   baseBlock,
			request: change.Node{Party: nodeTestParty, Role: nodeTestBatcher, Shard: 7, Port: 7150},
			wantErr: "party 1 has no batcher of shard 7",
		},
		{
			name:    "unknown role",
			block:   baseBlock,
			request: change.Node{Party: nodeTestParty, Role: "gateway", Port: 7150},
			wantErr: `unknown node role "gateway"`,
		},
		{
			name:    "certificate file that is not PEM",
			block:   baseBlock,
			request: change.Node{Party: nodeTestParty, Role: nodeTestRouter, TLSCert: notPEMPath},
			wantErr: "is not a PEM-encoded certificate",
		},
		{
			name:    "PEM file that is not a certificate",
			block:   baseBlock,
			request: change.Node{Party: nodeTestParty, Role: nodeTestRouter, SignCert: keyPath},
			wantErr: "is not a PEM-encoded certificate",
		},
		{
			name:    "missing certificate file",
			block:   baseBlock,
			request: change.Node{Party: nodeTestParty, Role: nodeTestRouter, TLSCert: missingPath},
			wantErr: "failed to read certificate",
		},
		{
			name:    "router endpoint that also serves deliver",
			block:   sharedEndpointBlock,
			request: change.Node{Party: nodeTestParty, Role: nodeTestRouter, Port: 7150},
			wantErr: "no orderer org has a broadcast endpoint of party 1",
		},
		{
			name:    "org with no broadcast endpoint of the party",
			block:   noBroadcastBlock,
			request: change.Node{Party: nodeTestParty, Role: nodeTestRouter, Port: 7150},
			wantErr: "no orderer org has a broadcast endpoint of party 1",
		},
		{
			name:    "org with no deliver endpoint of the party",
			block:   noDeliverBlock,
			request: change.Node{Party: nodeTestParty, Role: nodeTestAssembler, Host: "assembler2.org1"},
			wantErr: "no orderer org has a deliver endpoint of party 1",
		},
		{
			name:    "malformed endpoint in the org",
			block:   malformedEndpointBlock,
			request: change.Node{Party: nodeTestParty, Role: nodeTestRouter, Port: 7150},
			wantErr: fmt.Sprintf("failed to parse endpoint %q", malformedEndpoint),
		},
		{
			name:    "host with a port",
			block:   baseBlock,
			request: change.Node{Party: nodeTestParty, Role: nodeTestRouter, Host: "router2.org1:7050"},
			wantErr: `invalid host "router2.org1:7050"`,
		},
		{
			name:    "host with a comma",
			block:   baseBlock,
			request: change.Node{Party: nodeTestParty, Role: nodeTestRouter, Host: "router2.org1,deliver"},
			wantErr: `invalid host "router2.org1,deliver"`,
		},
		{
			name:    "host with a space",
			block:   baseBlock,
			request: change.Node{Party: nodeTestParty, Role: nodeTestRouter, Host: "router2 org1"},
			wantErr: `invalid host "router2 org1"`,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			blockPath := writeNodeTestBlock(t, tc.block)
			tc.request.BlockPath = blockPath

			err := modify.New().PartyNode(tc.request)
			require.ErrorContains(t, err, tc.wantErr)

			content, err := os.ReadFile(blockPath)
			require.NoError(t, err)
			require.Equal(t, tc.block, content, "a failed change must leave the block unchanged")
		})
	}
}

// nodeTestMaterial is what a test of a node change needs: the marshaled config
// block of nodeTestOrg's one party, and a TLS and a signing certificate for a
// node of that party that no node of the block has, issued by the party's CAs.
type nodeTestMaterial struct {
	block       []byte
	newTLSCert  []byte
	newSignCert []byte
}

// newNodeTestMaterial generates nodeTestOrg's crypto material and returns its
// config block and a node's new certificates.
//
// The block is a marshaled ARMA config block whose one party, nodeTestParty of
// nodeTestOrg, is consistent across the block: its shared config entry (its
// CAs, a router, batchers of shards 1 and 2, a consenter, and an assembler), its
// consenter mapping entry, its org's broadcast and deliver endpoints, and the
// BlockValidation policy over its consenter. The org has a spare node besides
// its router and assembler, whose certificates, issued by the org's CAs, serve
// as a node's new certificates.
func newNodeTestMaterial(t *testing.T) *nodeTestMaterial {
	t.Helper()
	cryptoDir := t.TempDir()
	block, err := cryptogen.CreateOrExtendConfigBlockWithCrypto(cryptogen.ConfigBlockParameters{
		TargetPath:  cryptoDir,
		BaseProfile: configtxgen.SampleFabricX,
		ChannelID:   "mychannel",
		Organizations: []cryptogen.OrganizationParameters{{
			Name:   nodeTestOrg,
			Domain: nodeTestOrg + ".com",
			OrdererEndpoints: []*types.OrdererEndpoint{
				{
					ID: nodeTestParty, Host: nodeTestRouterHost, Port: nodeTestRouterPort,
					API: []string{types.Broadcast},
				},
				{
					ID: nodeTestParty, Host: nodeTestAssemblerHost, Port: nodeTestAssemblerPort,
					API: []string{types.Deliver},
				},
			},
			ConsenterNodes: []cryptogen.Node{{CommonName: nodeTestConsenter, Hostname: "consenter.org1:7052"}},
			OrdererNodes: []cryptogen.Node{
				{CommonName: nodeTestRouter, Hostname: nodeTestRouterHost},
				{CommonName: nodeTestAssembler, Hostname: nodeTestAssemblerHost},
				{CommonName: nodeTestSpareNode, Hostname: nodeTestSpareNode + ".org1"},
			},
		}},
	})
	require.NoError(t, err)
	configBlock, err := helpers.DecodeConfigBlock(block)
	require.NoError(t, err)
	orderer := configBlock.Config().GetChannelGroup().GetGroups()[channelconfig.OrdererGroupKey]
	orgMSP := nodeTestFabricMSPConfig(t, orderer.GetGroups()[nodeTestOrg])

	// cryptogen leaves the shared config empty: write the party's entry, with
	// the org's CAs and the certificates of its consenter.
	consenter := nodeTestConsenterOf(t, orderer)
	signCert, tlsCert := consenter.GetIdentity(), consenter.GetServerTlsCert()
	shared, err := proto.Marshal(&ordererpb.SharedConfig{
		PartiesConfig: []*ordererpb.PartyConfig{{
			PartyID:    nodeTestParty,
			CACerts:    orgMSP.GetRootCerts(),
			TLSCACerts: orgMSP.GetTlsRootCerts(),
			RouterConfig: &ordererpb.RouterNodeConfig{
				Host: nodeTestRouterHost, Port: nodeTestRouterPort, TlsCert: tlsCert, SignCert: signCert,
			},
			BatchersConfig: []*ordererpb.BatcherNodeConfig{
				{ShardID: 1, Host: "batcher1.org1", Port: 7071, SignCert: signCert, TlsCert: tlsCert},
				{ShardID: 2, Host: "batcher2.org1", Port: 7072, SignCert: signCert, TlsCert: tlsCert},
			},
			ConsenterConfig: &ordererpb.ConsenterNodeConfig{
				Host: consenter.GetHost(), Port: consenter.GetPort(), SignCert: signCert, TlsCert: tlsCert,
			},
			AssemblerConfig: &ordererpb.AssemblerNodeConfig{
				Host: nodeTestAssemblerHost, Port: nodeTestAssemblerPort, TlsCert: tlsCert, SignCert: signCert,
			},
		}},
		MaxPartyID: nodeTestParty,
	})
	require.NoError(t, err)
	consensusType := &ab.ConsensusType{}
	value := orderer.GetValues()[channelconfig.ConsensusTypeKey]
	require.NoError(t, proto.Unmarshal(value.GetValue(), consensusType))
	consensusType.Metadata = shared
	value.Value = protoutil.MarshalOrPanic(consensusType)
	orderer.GetPolicies()[policies.BlockValidationPolicyKey].Version = nodeTestPolicyVersion

	require.NoError(t, configBlock.Encode())
	marshaled := protoutil.MarshalOrPanic(configBlock.Block)

	spareNodeDir := filepath.Join(cryptoDir, cryptogen.OrdererOrganizationsDir, nodeTestOrg+".com",
		cryptogen.OrdererNodesDir, nodeTestSpareNode)
	return &nodeTestMaterial{
		block:      marshaled,
		newTLSCert: readNodeTestFile(t, filepath.Join(spareNodeDir, cryptogen.TLSDir, "server.crt")),
		newSignCert: readNodeTestFile(t, filepath.Join(spareNodeDir, cryptogen.MSPDir, cryptogen.SignCertsDir,
			nodeTestSpareNode+cryptogen.CertSuffix)),
	}
}

// readNodeTestFile returns the content of the file at path.
func readNodeTestFile(t *testing.T, path string) []byte {
	t.Helper()
	content, err := os.ReadFile(path)
	require.NoError(t, err)
	return content
}

// nodeTestFabricMSPConfig returns the Fabric MSP config of an org group.
func nodeTestFabricMSPConfig(t *testing.T, org *cb.ConfigGroup) *msppb.FabricMSPConfig {
	t.Helper()
	mspConfig := &mspproto.MSPConfig{}
	require.NoError(t, proto.Unmarshal(org.GetValues()[channelconfig.MSPKey].GetValue(), mspConfig))
	fabricConfig := &msppb.FabricMSPConfig{}
	require.NoError(t, proto.Unmarshal(mspConfig.GetConfig(), fabricConfig))
	return fabricConfig
}

// withNodeTestEndpoints returns the marshaled block with the Endpoints value of
// nodeTestOrg replaced by the given addresses.
func withNodeTestEndpoints(t *testing.T, marshaledBlock []byte, addresses ...string) []byte {
	t.Helper()
	configBlock, err := helpers.UnmarshalAndDecodeConfigBlock(marshaledBlock)
	require.NoError(t, err)
	org := configBlock.Config().GetChannelGroup().GetGroups()[channelconfig.OrdererGroupKey].GetGroups()[nodeTestOrg]
	org.GetValues()[channelconfig.EndpointsKey].Value = protoutil.MarshalOrPanic(&cb.OrdererAddresses{
		Addresses: addresses,
	})
	require.NoError(t, configBlock.Encode())
	return protoutil.MarshalOrPanic(configBlock.Block)
}

// writeNodeTestBlock writes content as the block file of a fresh directory and
// returns its path.
func writeNodeTestBlock(t *testing.T, content []byte) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), nodeTestBlockFile)
	require.NoError(t, os.WriteFile(path, content, nodeTestBlockMode))
	return path
}

// writeNodeTestFile writes content as the named file in dir and returns its path.
func writeNodeTestFile(t *testing.T, dir, name string, content []byte) string {
	t.Helper()
	path := filepath.Join(dir, name)
	require.NoError(t, os.WriteFile(path, content, 0o600))
	return path
}

// nodeTestConfigOf returns the config of a marshaled config block.
func nodeTestConfigOf(t *testing.T, marshaledBlock []byte) *cb.Config {
	t.Helper()
	configBlock, err := helpers.UnmarshalAndDecodeConfigBlock(marshaledBlock)
	require.NoError(t, err)
	return configBlock.Config()
}

// nodeTestOrderer returns the Orderer group of a marshaled config block.
func nodeTestOrderer(t *testing.T, marshaledBlock []byte) *cb.ConfigGroup {
	t.Helper()
	orderer := nodeTestConfigOf(t, marshaledBlock).GetChannelGroup().GetGroups()[channelconfig.OrdererGroupKey]
	require.NotNil(t, orderer)
	return orderer
}

// nodeTestSharedConfig returns the ARMA shared config of an Orderer group.
func nodeTestSharedConfig(t *testing.T, orderer *cb.ConfigGroup) *ordererpb.SharedConfig {
	t.Helper()
	consensusType := &ab.ConsensusType{}
	require.NoError(t, proto.Unmarshal(orderer.GetValues()[channelconfig.ConsensusTypeKey].GetValue(), consensusType))
	shared := &ordererpb.SharedConfig{}
	require.NoError(t, proto.Unmarshal(consensusType.GetMetadata(), shared))
	return shared
}

// nodeTestNode returns the shared config entry of nodeTestParty's node of the
// given role (and shard, for a batcher).
func nodeTestNode(t *testing.T, shared *ordererpb.SharedConfig, role string, shard uint32) nodeTestConfig {
	t.Helper()
	parties := shared.GetPartiesConfig()
	require.Len(t, parties, 1)
	party := parties[0]
	switch role {
	case nodeTestRouter:
		return party.GetRouterConfig()
	case nodeTestConsenter:
		return party.GetConsenterConfig()
	case nodeTestAssembler:
		return party.GetAssemblerConfig()
	default:
		for _, batcher := range party.GetBatchersConfig() {
			if batcher.GetShardID() == shard {
				return batcher
			}
		}
		require.FailNow(t, "no such node", "role %s, shard %d", role, shard)
		return nil
	}
}

// nodeTestFieldsOf returns a node config's endpoint and certificates.
func nodeTestFieldsOf(node nodeTestConfig) nodeTestFields {
	return nodeTestFields{
		host: node.GetHost(), port: node.GetPort(), tlsCert: node.GetTlsCert(), signCert: node.GetSignCert(),
	}
}

// nodeTestConsenterOf returns the only consenter of an Orderer group's
// consenter mapping.
func nodeTestConsenterOf(t *testing.T, orderer *cb.ConfigGroup) *cb.Consenter {
	t.Helper()
	orderers := &cb.Orderers{}
	require.NoError(t, proto.Unmarshal(orderer.GetValues()[channelconfig.OrderersKey].GetValue(), orderers))
	require.Len(t, orderers.GetConsenterMapping(), 1)
	return orderers.GetConsenterMapping()[0]
}

// nodeTestEndpoints returns the addresses of nodeTestOrg's Endpoints value.
func nodeTestEndpoints(t *testing.T, orderer *cb.ConfigGroup) []string {
	t.Helper()
	addresses := &cb.OrdererAddresses{}
	value := orderer.GetGroups()[nodeTestOrg].GetValues()[channelconfig.EndpointsKey]
	require.NoError(t, proto.Unmarshal(value.GetValue(), addresses))
	return addresses.GetAddresses()
}

// requireNodeTestBlockValidation asserts that the Orderer group's
// BlockValidation policy is the BFT quorum over the one consenter, by its
// current identity, and keeps the block's mod_policy and version.
func requireNodeTestBlockValidation(t *testing.T, orderer *cb.ConfigGroup, consenter *cb.Consenter) {
	t.Helper()
	policy := orderer.GetPolicies()[policies.BlockValidationPolicyKey]
	require.Equal(t, uint64(nodeTestPolicyVersion), policy.GetVersion())
	require.Equal(t, channelconfig.AdminsPolicyKey, policy.GetModPolicy())
	require.Equal(t, int32(cb.Policy_SIGNATURE), policy.GetPolicy().GetType())
	envelope := &cb.SignaturePolicyEnvelope{}
	require.NoError(t, proto.Unmarshal(policy.GetPolicy().GetValue(), envelope))
	require.EqualValues(t, policies.ComputeBFTQuorum(1, 0), envelope.GetRule().GetNOutOf().GetN())
	require.Len(t, envelope.GetIdentities(), 1)
	wantIdentity := protoutil.MarshalOrPanic(msppb.NewIdentity(consenter.GetMspId(), consenter.GetIdentity()))
	require.Equal(t, wantIdentity, envelope.GetIdentities()[0].GetPrincipal())
}

// nodeTestRequestOf returns the request of a TestPartyNode case on the block at
// blockPath, and the endpoint and certificates the request should leave on the
// node, whose current ones are given.
func nodeTestRequestOf(
	tc *nodeTestCase, blockPath string, current nodeTestFields, certs *nodeTestCertFiles,
) (change.Node, nodeTestFields) {
	request := change.Node{
		Party:     nodeTestParty,
		Role:      tc.role,
		Shard:     tc.shard,
		Host:      tc.host,
		Port:      tc.port,
		BlockPath: blockPath,
	}
	want := current
	if tc.host != "" {
		want.host = tc.host
	}
	if tc.port != 0 {
		want.port = tc.port
	}
	if tc.tlsCert {
		request.TLSCert, want.tlsCert = certs.tlsCertPath, certs.tlsCert
	}
	if tc.signCert {
		request.SignCert, want.signCert = certs.signCertPath, certs.signCert
	}
	return request, want
}

// wantNodeTestConsenter returns the consenter mapping entry a change should
// leave when it leaves the node with want: the base entry, which follows the
// consenter only.
func wantNodeTestConsenter(t *testing.T, base *cb.ConfigGroup, role string, want nodeTestFields) *cb.Consenter {
	t.Helper()
	consenter := nodeTestConsenterOf(t, base)
	if role == nodeTestConsenter {
		consenter.Host, consenter.Port = want.host, want.port
		consenter.ClientTlsCert, consenter.ServerTlsCert = want.tlsCert, want.tlsCert
		consenter.Identity = want.signCert
	}
	return consenter
}

// requireNodeTestUnchangedElsewhere asserts that the edited block's config
// equals the base block's apart from the places a node change may touch: the
// Orderer group's ConsensusType and Orderers values and BlockValidation policy,
// and nodeTestOrg's Endpoints value.
func requireNodeTestUnchangedElsewhere(t *testing.T, baseBlock, editedBlock []byte) {
	t.Helper()
	base := nodeTestOrderer(t, baseBlock)
	restored := nodeTestConfigOf(t, editedBlock)
	orderer := restored.GetChannelGroup().GetGroups()[channelconfig.OrdererGroupKey]
	for _, key := range []string{channelconfig.ConsensusTypeKey, channelconfig.OrderersKey} {
		orderer.Values[key] = base.GetValues()[key]
	}
	blockValidation := policies.BlockValidationPolicyKey
	orderer.Policies[blockValidation] = base.GetPolicies()[blockValidation]
	endpoints := base.GetGroups()[nodeTestOrg].GetValues()[channelconfig.EndpointsKey]
	orderer.Groups[nodeTestOrg].Values[channelconfig.EndpointsKey] = endpoints
	test.RequireProtoEqual(t, nodeTestConfigOf(t, baseBlock), restored)
}
