/*
Copyright IBM Corp. All Rights Reserved.

SPDX-License-Identifier: Apache-2.0
*/

package cryptogen

import (
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"maps"
	"net"
	"os"
	"path"
	"path/filepath"
	"sync"
	"testing"

	"github.com/hyperledger/fabric-lib-go/bccsp/factory"
	"github.com/hyperledger/fabric-protos-go-apiv2/common"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/health"
	healthgrpc "google.golang.org/grpc/health/grpc_health_v1"

	"github.com/hyperledger/fabric-x-common/api/types"
	"github.com/hyperledger/fabric-x-common/common/channelconfig"
	"github.com/hyperledger/fabric-x-common/msp"
	"github.com/hyperledger/fabric-x-common/protoutil"
	"github.com/hyperledger/fabric-x-common/tools/test"
)

func TestMakeConfig(t *testing.T) {
	t.Parallel()
	target := t.TempDir()
	p, block, armaData := defaultConfigBlock(t, target)

	var expectedDirs []string //nolint:prealloc // Hard to estimate size.

	orgNames := map[int]string{
		1: "ordering-and-peer-org-1",
		2: "ordering-org-2",
		3: "peer-org-3",
		4: "peer-org-4",
	}
	orgDirs := map[int]string{
		1: filepath.Join(GenericOrganizationsDir, orgNames[1]),
		2: filepath.Join(OrdererOrganizationsDir, orgNames[2]),
		3: filepath.Join(PeerOrganizationsDir, orgNames[3]),
		4: filepath.Join(PeerOrganizationsDir, orgNames[4]),
	}
	// Add all users.
	for orgDir := range maps.Values(orgDirs) {
		for _, n := range []string{"client", "Admin"} {
			expectedDirs = append(expectedDirs, filepath.Join(orgDir, "users", n+"@"+path.Base(orgDir)+".com", "msp"))
		}
	}
	// Add all committer nodes to the peer organizations.
	for _, orgDir := range []string{orgDirs[1], orgDirs[3], orgDirs[4]} {
		for _, n := range []string{"committer", "coordinator", "vc", "query", "endorser"} {
			expectedDirs = append(expectedDirs, filepath.Join(orgDir, "peers", n, "msp"))
		}
	}
	// Add all org-1 parties' orderers.
	for _, n := range []string{
		"party-1/router-1", "party-2/router-2",
		"party-1/assembler-1", "party-2/assembler-2",
		"party-1/batcher-1", "party-2/batcher-2",
		"party-1/consenter-1", "party-2/consenter-2",
	} {
		expectedDirs = append(expectedDirs, filepath.Join(orgDirs[1], "orderers", n, "msp"))
	}
	// Add all org-2 orderers.
	for _, n := range []string{"router", "assembler", "batcher", "consenter"} {
		expectedDirs = append(expectedDirs, filepath.Join(orgDirs[2], "orderers", n, "msp"))
	}

	test.RequireTree(t, target, []string{ConfigBlockFileName}, expectedDirs)

	bundle := readBundle(t, block)
	oc, ok := bundle.OrdererConfig()
	require.True(t, ok)
	orgMap := oc.Organizations()
	require.Len(t, orgMap, 2)

	var endpoints []*types.OrdererEndpoint //nolint:prealloc // Hard to estimate size.
	for orgID, org := range orgMap {
		require.Equal(t, orgID, org.MSPID())
		require.Equal(t, orgID, org.Name())
		endpointsStr := org.Endpoints()
		for _, eStr := range endpointsStr {
			e, parseErr := types.ParseOrdererEndpoint(eStr)
			require.NoError(t, parseErr)
			e.MspID = orgID
			endpoints = append(endpoints, e)
		}
	}
	require.Len(t, endpoints, 6)
	require.ElementsMatch(t, endpoints, []*types.OrdererEndpoint{
		{
			Host:  "localhost",
			Port:  6001,
			ID:    1,
			MspID: orgNames[1],
			API:   []string{types.Broadcast},
		},
		{
			Host:  "localhost",
			Port:  7001,
			ID:    1,
			MspID: orgNames[1],
			API:   []string{types.Deliver},
		},
		{
			Host:  "localhost",
			Port:  6002,
			ID:    2,
			MspID: orgNames[1],
			API:   []string{types.Broadcast},
		},
		{
			Host:  "localhost",
			Port:  7002,
			ID:    2,
			MspID: orgNames[1],
			API:   []string{types.Deliver},
		},
		{
			Host:  "localhost",
			Port:  6003,
			ID:    3,
			MspID: orgNames[2],
			API:   []string{types.Broadcast},
		},
		{
			Host:  "localhost",
			Port:  7003,
			ID:    3,
			MspID: orgNames[2],
			API:   []string{types.Deliver},
		},
	})

	require.Equal(t, armaData, oc.ConsensusMetadata())

	t.Log("Checking channel level policies with majority")
	mspDir := func(orgIdx int, subPath ...string) msp.DirLoadParameters {
		return msp.DirLoadParameters{
			MspName: orgNames[orgIdx],
			MspDir:  path.Join(target, orgDirs[orgIdx], path.Join(subPath...), MSPDir),
		}
	}
	userDir := func(orgIdx int, username string) msp.DirLoadParameters {
		return mspDir(orgIdx, UsersDir, fmt.Sprintf("%s@%s.com", username, orgNames[orgIdx]))
	}
	adminDir := func(orgIdx int) msp.DirLoadParameters {
		return userDir(orgIdx, "Admin")
	}
	clientDir := func(orgIdx int) msp.DirLoadParameters {
		return userDir(orgIdx, "client")
	}
	endorserDir := func(orgIdx int) msp.DirLoadParameters {
		return mspDir(orgIdx, PeerNodesDir, "endorser")
	}

	admins := loadMSPs(t, adminDir(1), adminDir(2), adminDir(3), adminDir(4))
	writers := loadMSPs(t, clientDir(1), clientDir(2), clientDir(3), clientDir(4))
	endorsers := loadMSPs(t, endorserDir(1), endorserDir(3), endorserDir(4))
	consenters := loadMSPs(t,
		mspDir(1, OrdererNodesDir, "party-1", "consenter-1"),
		mspDir(1, OrdererNodesDir, "party-2", "consenter-2"),
		mspDir(2, OrdererNodesDir, "consenter"),
	)

	requireSign(t, bundle, "Orderer/Admins", admins[:2]...)
	requireSign(t, bundle, "Application/Admins", admins[0], admins[2], admins[3])
	requireSign(t, bundle, "Application/Admins", admins[0], admins[2])
	requireSign(t, bundle, "Orderer/Writers", writers[:2]...)
	requireSign(t, bundle, "Application/Writers", writers[0], writers[2], writers[3])
	requireSign(t, bundle, "Application/Writers", writers[0], writers[2])
	requireSign(t, bundle, "Application/Endorsement", endorsers...)
	requireSign(t, bundle, "Application/Endorsement", endorsers[:2]...)
	requireSign(t, bundle, "Application/Endorsement", endorsers[1:]...)
	requireSign(t, bundle, "Orderer/BlockValidation", consenters...)
	requireSign(t, bundle, "Orderer/BlockValidation", consenters[0], consenters[2])
	requireSign(t, bundle, "Orderer/BlockValidation", consenters[1:]...)

	t.Log("Add 3 peer organizations")
	// We add 3 peer organizations (total 6). So we need 4 for majority.
	// This means that we always need at least one organization for each group (old and new).
	// By testing both cases, we ensure the previous credentials haven't changed, and the new ones were added.
	p.Organizations = append(p.Organizations, []OrganizationParameters{
		{
			Name:      "peer-org-5",
			Domain:    "peer-org-5.com",
			PeerNodes: peerNodes,
		},
		{
			Name:      "peer-org-6",
			Domain:    "peer-org-6.com",
			PeerNodes: peerNodes,
		},
		{
			Name:      "peer-org-7",
			Domain:    "peer-org-7.com",
			PeerNodes: peerNodes,
		},
	}...)
	block2 := createBlock(t, p)
	bundle2 := readBundle(t, block2)

	orgNames[5] = "peer-org-5"
	orgNames[6] = "peer-org-6"
	orgNames[7] = "peer-org-7"
	orgDirs[5] = filepath.Join(PeerOrganizationsDir, orgNames[5])
	orgDirs[6] = filepath.Join(PeerOrganizationsDir, orgNames[6])
	orgDirs[7] = filepath.Join(PeerOrganizationsDir, orgNames[7])
	endorsers = append(endorsers, loadMSPs(t, endorserDir(5), endorserDir(6), endorserDir(7))...)

	requireSign(t, bundle2, "Application/Endorsement", endorsers...)
	requireSign(t, bundle2, "Application/Endorsement", endorsers[2:]...)
	requireSign(t, bundle2, "Application/Endorsement", endorsers[:4]...)

	t.Log("Remove 2 peer organizations")
	// We remove 2 peer organnizations, so now 3 peers are sufficient for majority.
	p.Organizations = p.Organizations[:len(p.Organizations)-2]
	block3 := createBlock(t, p)
	bundle3 := readBundle(t, block3)
	requireSign(t, bundle3, "Application/Endorsement", endorsers...)
	requireSign(t, bundle3, "Application/Endorsement", endorsers[:3]...)
	requireSign(t, bundle3, "Application/Endorsement", endorsers[1:4]...)
}

func TestCryptoGenTLS(t *testing.T) {
	t.Parallel()
	testDir := t.TempDir()
	defaultConfigBlock(t, testDir)

	org2Node := path.Join(testDir, OrdererOrganizationsDir, "ordering-org-2", OrdererNodesDir, "assembler")
	org3Node := path.Join(testDir, PeerOrganizationsDir, "peer-org-3", PeerNodesDir, "committer")

	org2Ca := buildCertPool(t, path.Join(testDir, OrdererOrganizationsDir, "ordering-org-2",
		"tlsca", "tlsordering-org-2-CA-cert.pem"))
	org3Ca := buildCertPool(t, path.Join(testDir, PeerOrganizationsDir, "peer-org-3",
		"tlsca", "tlspeer-org-3-CA-cert.pem"))

	address := grpcServer(t, org2Node, org3Ca)
	healthClient := grpcClient(t, org3Node, org2Ca, address)
	ret, err := healthClient.Check(t.Context(), nil)
	require.NoError(t, err)
	require.NotNil(t, ret)
	t.Log(ret)
}

func TestConfigBlockTLS(t *testing.T) {
	t.Parallel()
	testDir := t.TempDir()
	_, block, _ := defaultConfigBlock(t, testDir)
	org2Node := path.Join(testDir, OrdererOrganizationsDir, "ordering-org-2", OrdererNodesDir, "assembler")
	org3Node := path.Join(testDir, PeerOrganizationsDir, "peer-org-3", PeerNodesDir, "committer")

	bundle := readBundle(t, block)

	// We use all the application's CAs for the server to mimic a real server that support's all peers.
	ac, ok := bundle.ApplicationConfig()
	require.True(t, ok)
	appOrgMap := ac.Organizations()
	appCaCerts := make([][]byte, 0, len(appOrgMap))
	for _, o := range appOrgMap {
		appCaCerts = append(appCaCerts, o.MSP().GetTLSRootCerts()...)
	}
	appCa := buildCertPoolFromBytes(t, appCaCerts...)

	// We only use the target org's CA to mimic a client that connects to a specific server.
	oc, ok := bundle.OrdererConfig()
	require.True(t, ok)
	orgMap := oc.Organizations()
	org2, ok := orgMap["ordering-org-2"]
	require.True(t, ok)
	org2CaCerts := org2.MSP().GetTLSRootCerts()
	org2Ca := buildCertPoolFromBytes(t, org2CaCerts...)

	address := grpcServer(t, org2Node, appCa)
	healthClient := grpcClient(t, org3Node, org2Ca, address)
	ret, err := healthClient.Check(t.Context(), nil)
	require.NoError(t, err)
	require.NotNil(t, ret)
	t.Log(ret)
}

func readBundle(t *testing.T, block *common.Block) *channelconfig.Bundle {
	t.Helper()
	require.NotNil(t, block.Data)
	require.NotEmpty(t, block.Data.Data)
	envelope, err := protoutil.ExtractEnvelope(block, 0)
	require.NoError(t, err)

	bundle, err := channelconfig.NewBundleFromEnvelope(envelope, factory.GetDefault())
	require.NoError(t, err)
	return bundle
}

func grpcServer(t *testing.T, nodePath string, caCert *x509.CertPool) string {
	t.Helper()
	server := grpc.NewServer(grpc.Creds(credentials.NewTLS(&tls.Config{
		MinVersion:   tls.VersionTLS12,
		ClientAuth:   tls.RequireAndVerifyClientCert,
		ClientCAs:    caCert,
		Certificates: loadServerKeyPair(t, nodePath),
	})))

	healthcheck := health.NewServer()
	healthcheck.SetServingStatus("", healthgrpc.HealthCheckResponse_SERVING)
	healthgrpc.RegisterHealthServer(server, healthcheck)

	address := "127.0.0.1:0"

	listener, err := net.Listen("tcp", address)
	require.NoError(t, err)
	require.NotNil(t, listener)

	addr := listener.Addr()
	tcpAddress, ok := addr.(*net.TCPAddr)
	require.True(t, ok)
	address = tcpAddress.String()

	wg := sync.WaitGroup{}
	t.Cleanup(wg.Wait)
	t.Cleanup(server.Stop)
	wg.Go(func() {
		assert.NoError(t, server.Serve(listener))
	})
	return address
}

//nolint:ireturn // forced to return interface.
func grpcClient(t *testing.T, nodePath string, caCert *x509.CertPool, endpoint string) healthgrpc.HealthClient {
	t.Helper()
	tlsCreds := credentials.NewTLS(&tls.Config{
		MinVersion:   tls.VersionTLS12,
		RootCAs:      caCert,
		Certificates: loadServerKeyPair(t, nodePath),
	})
	conn, err := grpc.NewClient(endpoint, grpc.WithTransportCredentials(tlsCreds))
	require.NoError(t, err)
	return healthgrpc.NewHealthClient(conn)
}

func loadServerKeyPair(t *testing.T, nodePath string) []tls.Certificate {
	t.Helper()
	certPath := path.Join(nodePath, TLSDir, "server.crt")
	keyPath := path.Join(nodePath, TLSDir, "server.key")
	cert, err := tls.LoadX509KeyPair(certPath, keyPath)
	require.NoError(t, err)
	return []tls.Certificate{cert}
}

func buildCertPool(t *testing.T, paths ...string) *x509.CertPool {
	t.Helper()
	pemBytesList := make([][]byte, len(paths))
	for i, p := range paths {
		pemBytes, err := os.ReadFile(p)
		require.NoError(t, err)
		require.NotEmpty(t, pemBytes)
		pemBytesList[i] = pemBytes
	}
	return buildCertPoolFromBytes(t, pemBytesList...)
}

func buildCertPoolFromBytes(t *testing.T, certs ...[]byte) *x509.CertPool {
	t.Helper()
	require.NotEmpty(t, certs)
	certPool := x509.NewCertPool()
	for _, pemBytes := range certs {
		ok := certPool.AppendCertsFromPEM(pemBytes)
		require.True(t, ok)
	}
	return certPool
}

func loadMSPs(t *testing.T, users ...msp.DirLoadParameters) []msp.MSP {
	t.Helper()
	ret := make([]msp.MSP, len(users))
	for i, u := range users {
		mspUser, err := msp.LoadLocalMspDir(u)
		require.NoError(t, err)
		require.NotNil(t, mspUser)
		ret[i] = mspUser
	}
	return ret
}

func requireSign(t *testing.T, bundle *channelconfig.Bundle, policyName string, users ...msp.MSP) {
	t.Helper()
	policy, ok := bundle.PolicyManager().GetPolicy(policyName)
	require.Truef(t, ok, "policy %s not found", policyName)
	require.NotNil(t, policy)

	data := []byte("data")
	signedData := make([]*protoutil.SignedData, len(users))
	for i, mspUser := range users {
		si, err := mspUser.GetDefaultSigningIdentity()
		require.NoError(t, err)
		siID, err := si.Serialize()
		require.NoError(t, err)
		id, err := protoutil.UnmarshalIdentity(siID)
		require.NoError(t, err)
		sig, err := si.Sign(data)
		require.NoError(t, err)
		signedData[i] = &protoutil.SignedData{
			Data:      data,
			Identity:  id,
			Signature: sig,
		}
	}

	err := policy.EvaluateSignedData(signedData)
	require.NoError(t, err)
}

var (
	sans      = []string{"127.0.0.1"}
	peerNodes = []Node{
		{CommonName: "committer", Hostname: "localhost", SANS: sans},
		{CommonName: "coordinator", Hostname: "localhost", SANS: sans},
		{CommonName: "verifier", Hostname: "localhost", SANS: sans},
		{CommonName: "vc", Hostname: "localhost", SANS: sans},
		{CommonName: "query", Hostname: "localhost", SANS: sans},
		{CommonName: "endorser", Hostname: "localhost", SANS: sans},
	}
)

func defaultConfigBlock(t *testing.T, target string) (
	p ConfigBlockParameters, block *common.Block, armaData []byte,
) {
	t.Helper()
	armaData = []byte("fake-arma-data")

	p = ConfigBlockParameters{
		TargetPath: target,
		ChannelID:  "my-chan",
		Organizations: []OrganizationParameters{
			{ // Joint org with two ordering parties.
				Name:   "ordering-and-peer-org-1",
				Domain: "ordering-and-peer-org-1.com",
				OrdererEndpoints: []*types.OrdererEndpoint{
					{ID: 1, Host: "localhost", Port: 6001, API: []string{types.Broadcast}},
					{ID: 1, Host: "localhost", Port: 7001, API: []string{types.Deliver}},
					{ID: 2, Host: "localhost", Port: 6002, API: []string{types.Broadcast}},
					{ID: 2, Host: "localhost", Port: 7002, API: []string{types.Deliver}},
				},
				ConsenterNodes: []Node{
					{PartyName: "party-1", CommonName: "consenter-1", Hostname: "localhost", SANS: sans},
					{PartyName: "party-2", CommonName: "consenter-2", Hostname: "localhost", SANS: sans},
				},
				OrdererNodes: []Node{
					{PartyName: "party-1", CommonName: "router-1", Hostname: "localhost", SANS: sans},
					{PartyName: "party-1", CommonName: "assembler-1", Hostname: "localhost", SANS: sans},
					{PartyName: "party-1", CommonName: "batcher-1", Hostname: "localhost", SANS: sans},
					{PartyName: "party-2", CommonName: "router-2", Hostname: "localhost", SANS: sans},
					{PartyName: "party-2", CommonName: "assembler-2", Hostname: "localhost", SANS: sans},
					{PartyName: "party-2", CommonName: "batcher-2", Hostname: "localhost", SANS: sans},
				},
				PeerNodes: peerNodes,
			},
			{ // Ordering org with a single party.
				Name:   "ordering-org-2",
				Domain: "ordering-org-2.com",
				OrdererEndpoints: []*types.OrdererEndpoint{
					{ID: 3, Host: "localhost", Port: 6003, API: []string{types.Broadcast}},
					{ID: 3, Host: "localhost", Port: 7003, API: []string{types.Deliver}},
				},
				ConsenterNodes: []Node{
					{CommonName: "consenter", Hostname: "localhost", SANS: sans},
				},
				OrdererNodes: []Node{
					{CommonName: "router", Hostname: "localhost", SANS: sans},
					{CommonName: "assembler", Hostname: "localhost", SANS: sans},
					{CommonName: "batcher", Hostname: "localhost", SANS: sans},
				},
			},
			{ // Peer org.
				Name:      "peer-org-3",
				Domain:    "peer-org-3.com",
				PeerNodes: peerNodes,
			},
			{ // Peer org.
				Name:      "peer-org-4",
				Domain:    "peer-org-4.com",
				PeerNodes: peerNodes,
			},
		},
		ArmaMetaBytes: armaData,
	}

	block = createBlock(t, p)
	return p, block, armaData
}

func createBlock(t *testing.T, p ConfigBlockParameters) *common.Block {
	t.Helper()
	block, err := CreateOrExtendConfigBlockWithCrypto(p)
	require.NoError(t, err)
	require.NotNil(t, block)
	require.NotNil(t, block.Data)
	require.NotEmpty(t, block.Data.Data)
	actualTree := test.GetTree(t, p.TargetPath)
	t.Cleanup(func() {
		t.Logf("Actual tree: %s", actualTree)
	})
	return block
}
