/*
Copyright IBM Corp. All Rights Reserved.

SPDX-License-Identifier: Apache-2.0
*/

package cryptogen

import (
	"crypto/tls"
	"crypto/x509"
	"encoding/pem"
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
	target, block, armaData := defaultConfigBlock(t)

	var expectedDirs []string //nolint:prealloc // Hard to estimate size.

	org1Dir := filepath.Join(GenericOrganizationsDir, "org-1")
	org2Dir := filepath.Join(OrdererOrganizationsDir, "org-2")
	org3Dir := filepath.Join(PeerOrganizationsDir, "org-3")
	// Add all users.
	for _, orgDir := range []string{org1Dir, org2Dir, org3Dir} {
		for _, n := range []string{"client", "Admin"} {
			expectedDirs = append(expectedDirs, filepath.Join(orgDir, "users", n+"@"+path.Base(orgDir)+".com", "msp"))
		}
	}
	// Add all committer nodes.
	for _, orgDir := range []string{org1Dir, org3Dir} {
		for _, n := range []string{"committer", "coordinator", "vc", "query", "endorser"} {
			expectedDirs = append(expectedDirs, filepath.Join(orgDir, "peers", n, "msp"))
		}
	}
	// Add all org-1 orderers.
	for _, n := range []string{
		"party-1/router-1", "party-2/router-2",
		"party-1/assembler-1", "party-2/assembler-2",
		"party-1/batcher-1", "party-2/batcher-2",
		"party-1/consenter-1", "party-2/consenter-2",
	} {
		expectedDirs = append(expectedDirs, filepath.Join(org1Dir, "orderers", n, "msp"))
	}
	// Add all org-2 orderers.
	for _, n := range []string{"router", "assembler", "batcher", "consenter"} {
		expectedDirs = append(expectedDirs, filepath.Join(org2Dir, "orderers", n, "msp"))
	}

	test.RequireTree(t, target, []string{"config-block.pb.bin"}, expectedDirs)

	bundle := readBundle(t, block)
	oc, ok := bundle.OrdererConfig()
	require.True(t, ok)
	orgMap := oc.Organizations()
	require.Len(t, orgMap, 2)

	var endpoints []*types.OrdererEndpoint
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
	require.Len(t, endpoints, 4)
	require.ElementsMatch(t, endpoints, []*types.OrdererEndpoint{
		{
			Host:  "localhost",
			Port:  6001,
			ID:    0,
			MspID: "org-1",
			API:   []string{types.Broadcast},
		},
		{
			Host:  "localhost",
			Port:  7001,
			ID:    0,
			MspID: "org-1",
			API:   []string{types.Deliver},
		},
		{
			Host:  "localhost",
			Port:  6002,
			ID:    1,
			MspID: "org-2",
			API:   []string{types.Broadcast},
		},
		{
			Host:  "localhost",
			Port:  7002,
			ID:    1,
			MspID: "org-2",
			API:   []string{types.Deliver},
		},
	})

	require.Equal(t, armaData, oc.ConsensusMetadata())

	requireSign(t, bundle, "Admins", msp.DirLoadParameters{
		MspName: "org-1",
		MspDir:  path.Join(target, org1Dir, UsersDir, "Admin@org-1.com", MSPDir),
	}, msp.DirLoadParameters{
		MspName: "org-2",
		MspDir:  path.Join(target, org2Dir, UsersDir, "Admin@org-2.com", MSPDir),
	}, msp.DirLoadParameters{
		MspName: "org-3",
		MspDir:  path.Join(target, org3Dir, UsersDir, "Admin@org-3.com", MSPDir),
	})
	requireSign(t, bundle, "Writers", msp.DirLoadParameters{
		MspName: "org-1",
		MspDir:  path.Join(target, org1Dir, UsersDir, "client@org-1.com", MSPDir),
	}, msp.DirLoadParameters{
		MspName: "org-2",
		MspDir:  path.Join(target, org2Dir, UsersDir, "client@org-2.com", MSPDir),
	}, msp.DirLoadParameters{
		MspName: "org-3",
		MspDir:  path.Join(target, org3Dir, UsersDir, "client@org-3.com", MSPDir),
	})
	requireSign(t, bundle, "Application/Endorsement", msp.DirLoadParameters{
		MspName: "org-1",
		MspDir:  path.Join(target, org1Dir, PeerNodesDir, "endorser", MSPDir),
	}, msp.DirLoadParameters{
		MspName: "org-3",
		MspDir:  path.Join(target, org3Dir, PeerNodesDir, "endorser", MSPDir),
	})
	requireSign(t, bundle, "Orderer/BlockValidation", msp.DirLoadParameters{
		MspName: "org-1",
		MspDir:  path.Join(target, org1Dir, OrdererNodesDir, "party-1", "consenter-1", MSPDir),
	}, msp.DirLoadParameters{
		MspName: "org-2",
		MspDir:  path.Join(target, org2Dir, OrdererNodesDir, "consenter", MSPDir),
	})
	requireSign(t, bundle, "Orderer/BlockValidation", msp.DirLoadParameters{
		MspName: "org-1",
		MspDir:  path.Join(target, org1Dir, OrdererNodesDir, "party-2", "consenter-2", MSPDir),
	}, msp.DirLoadParameters{
		MspName: "org-2",
		MspDir:  path.Join(target, org2Dir, OrdererNodesDir, "consenter", MSPDir),
	})
}

func TestCryptoGenTLS(t *testing.T) {
	t.Parallel()
	testDir, _, _ := defaultConfigBlock(t)

	org2Node := path.Join(testDir, OrdererOrganizationsDir, "org-2", OrdererNodesDir, "assembler")
	org3Node := path.Join(testDir, PeerOrganizationsDir, "org-3", PeerNodesDir, "committer")

	org2Ca := buildCertPool(t, path.Join(testDir, OrdererOrganizationsDir, "org-2", "tlsca", "tlsorg-2-CA-cert.pem"))
	org3Ca := buildCertPool(t, path.Join(testDir, PeerOrganizationsDir, "org-3", "tlsca", "tlsorg-3-CA-cert.pem"))

	address := grpcServer(t, org2Node, org3Ca)
	healthClient := grpcClient(t, org3Node, org2Ca, address)
	ret, err := healthClient.Check(t.Context(), nil)
	require.NoError(t, err)
	require.NotNil(t, ret)
	t.Log(ret)
}

func TestConfigBlockTLS(t *testing.T) {
	t.Parallel()
	testDir, block, _ := defaultConfigBlock(t)
	org2Node := path.Join(testDir, OrdererOrganizationsDir, "org-2", OrdererNodesDir, "assembler")
	org3Node := path.Join(testDir, PeerOrganizationsDir, "org-3", PeerNodesDir, "committer")

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
	org2, ok := orgMap["org-2"]
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

func requireSign(t *testing.T, bundle *channelconfig.Bundle, policyName string, users ...msp.DirLoadParameters) {
	t.Helper()
	policy, ok := bundle.PolicyManager().GetPolicy(policyName)
	require.Truef(t, ok, "policy %s not found", policyName)
	require.NotNil(t, policy)

	data := []byte("data")
	signedData := make([]*protoutil.SignedData, len(users))
	for i, u := range users {
		mspUser, err := msp.LoadLocalMspDir(u)
		require.NoError(t, err)
		require.NotNil(t, mspUser)

		si, err := mspUser.GetDefaultSigningIdentity()
		require.NoError(t, err)
		siID, err := si.Serialize()
		require.NoError(t, err)
		sig, err := si.Sign(data)
		require.NoError(t, err)
		signedData[i] = &protoutil.SignedData{
			Data:      data,
			Identity:  siID,
			Signature: sig,
		}
	}

	err := policy.EvaluateSignedData(signedData)
	require.NoError(t, err)
}

func defaultConfigBlock(t *testing.T) (
	target string, block *common.Block, armaData []byte,
) {
	t.Helper()
	target = t.TempDir()
	armaData = []byte("fake-arma-data")

	key, err := generatePrivateKey(target, ECDSA)
	require.NoError(t, err)
	certBytes, err := x509.MarshalPKIXPublicKey(getPublicKey(key))
	require.NoError(t, err)
	metaKeyBytes := pem.EncodeToMemory(&pem.Block{Type: CertType, Bytes: certBytes})
	p := ConfigBlockParameters{
		TargetPath: target,
		ChannelID:  "my-chan",
		Organizations: []OrganizationParameters{
			{ // Joint org with two ordering parties.
				Name:   "org-1",
				Domain: "org-1.com",
				OrdererEndpoints: []OrdererEndpoint{
					{Address: "localhost:6001", API: []string{types.Broadcast}},
					{Address: "localhost:7001", API: []string{types.Deliver}},
				},
				ConsenterNodes: []Node{
					{Party: "party-1", CommonName: "consenter-1", Hostname: "localhost", SANS: []string{"127.0.0.1"}},
					{Party: "party-2", CommonName: "consenter-2", Hostname: "localhost", SANS: []string{"127.0.0.1"}},
				},
				OrdererNodes: []Node{
					{Party: "party-1", CommonName: "router-1", Hostname: "localhost", SANS: []string{"127.0.0.1"}},
					{Party: "party-1", CommonName: "assembler-1", Hostname: "localhost", SANS: []string{"127.0.0.1"}},
					{Party: "party-1", CommonName: "batcher-1", Hostname: "localhost", SANS: []string{"127.0.0.1"}},
					{Party: "party-2", CommonName: "router-2", Hostname: "localhost", SANS: []string{"127.0.0.1"}},
					{Party: "party-2", CommonName: "assembler-2", Hostname: "localhost", SANS: []string{"127.0.0.1"}},
					{Party: "party-2", CommonName: "batcher-2", Hostname: "localhost", SANS: []string{"127.0.0.1"}},
				},
				PeerNodes: []Node{
					{CommonName: "committer", Hostname: "localhost", SANS: []string{"127.0.0.1"}},
					{CommonName: "coordinator", Hostname: "localhost", SANS: []string{"127.0.0.1"}},
					{CommonName: "verifier", Hostname: "localhost", SANS: []string{"127.0.0.1"}},
					{CommonName: "vc", Hostname: "localhost", SANS: []string{"127.0.0.1"}},
					{CommonName: "query", Hostname: "localhost", SANS: []string{"127.0.0.1"}},
					{CommonName: "endorser", Hostname: "localhost", SANS: []string{"127.0.0.1"}},
				},
			},
			{ // Ordering org with a single party.
				Name:   "org-2",
				Domain: "org-2.com",
				OrdererEndpoints: []OrdererEndpoint{
					{Address: "localhost:6002", API: []string{types.Broadcast}},
					{Address: "localhost:7002", API: []string{types.Deliver}},
				},
				ConsenterNodes: []Node{
					{CommonName: "consenter", Hostname: "localhost", SANS: []string{"127.0.0.1"}},
				},
				OrdererNodes: []Node{
					{CommonName: "router", Hostname: "localhost", SANS: []string{"127.0.0.1"}},
					{CommonName: "assembler", Hostname: "localhost", SANS: []string{"127.0.0.1"}},
					{CommonName: "batcher", Hostname: "localhost", SANS: []string{"127.0.0.1"}},
				},
			},
			{ // Peer org.
				Name:   "org-3",
				Domain: "org-3.com",
				PeerNodes: []Node{
					{CommonName: "committer", Hostname: "localhost", SANS: []string{"127.0.0.1"}},
					{CommonName: "coordinator", Hostname: "localhost", SANS: []string{"127.0.0.1"}},
					{CommonName: "verifier", Hostname: "localhost", SANS: []string{"127.0.0.1"}},
					{CommonName: "vc", Hostname: "localhost", SANS: []string{"127.0.0.1"}},
					{CommonName: "query", Hostname: "localhost", SANS: []string{"127.0.0.1"}},
					{CommonName: "endorser", Hostname: "localhost", SANS: []string{"127.0.0.1"}},
				},
			},
		},
		ArmaMetaBytes:                armaData,
		MetaNamespaceVerificationKey: metaKeyBytes,
	}

	block, err = CreateDefaultConfigBlockWithCrypto(p)
	require.NoError(t, err)
	require.NotNil(t, block)
	require.NotNil(t, block.Data)
	require.NotEmpty(t, block.Data.Data)

	t.Logf("Actual tree: %s", test.GetTree(t, target))
	return target, block, armaData
}
