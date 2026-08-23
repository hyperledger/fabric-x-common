/*
Copyright IBM Corp. All Rights Reserved.

SPDX-License-Identifier: Apache-2.0
*/

package client

import (
	"errors"
	"io"
	"net"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/hyperledger/fabric-lib-go/bccsp/factory"
	cb "github.com/hyperledger/fabric-protos-go-apiv2/common"
	ab "github.com/hyperledger/fabric-protos-go-apiv2/orderer"
	"github.com/stretchr/testify/require"
	"go.yaml.in/yaml/v3"
	"google.golang.org/grpc"
	"google.golang.org/protobuf/proto"

	"github.com/hyperledger/fabric-x-common/api/ordererpb"
	"github.com/hyperledger/fabric-x-common/api/types"
	"github.com/hyperledger/fabric-x-common/common/crypto/tlsgen"
	"github.com/hyperledger/fabric-x-common/tools/configtxgen"
	"github.com/hyperledger/fabric-x-common/tools/cryptogen"
	"github.com/hyperledger/fabric-x-common/tools/fxadmin/core/ordererconn"
	"github.com/hyperledger/fabric-x-common/tools/fxadmin/core/user"
	"github.com/hyperledger/fabric-x-common/tools/pkg/comm"
	"github.com/hyperledger/fabric-x-common/utils/testcrypto"
)

func enabled() *bool { b := true; return &b }

func TestLoad(t *testing.T) {
	t.Parallel()

	t.Run("builds a client from a config block and a loadable MSP", func(t *testing.T) {
		t.Parallel()
		sharedConfig := &ordererpb.SharedConfig{
			PartiesConfig: []*ordererpb.PartyConfig{{
				PartyID:         1,
				TLSCACerts:      [][]byte{[]byte("ca-1")},
				RouterConfig:    &ordererpb.RouterNodeConfig{Host: "router1.example.com", Port: 8013},
				AssemblerConfig: &ordererpb.AssemblerNodeConfig{Host: "assembler1.example.com", Port: 8011},
			}},
		}
		block, mspDir, mspID := newConfigBlockWithMSP(t, "arma", sharedConfig)

		config := &user.Config{MSP: user.MSPConfig{LocalMspID: mspID, LocalMspDir: mspDir}}
		client, err := load(config, block, factory.GetDefault())
		require.NoError(t, err)
		require.Equal(t, "arma", client.ChannelID())
		require.Equal(t, []string{"router1.example.com:8013"}, client.ordererConnInfo.RouterEndpoints)
		require.Equal(t, []string{"assembler1.example.com:8011"}, client.ordererConnInfo.AssemblerEndpoints)
		require.Equal(t, [][]byte{[]byte("ca-1")}, client.ordererConnInfo.TLSCACerts)
		require.NotNil(t, client.signer)
		require.False(t, client.clientConfig.SecOpts.UseTLS)
		require.Nil(t, client.tlsCertHash)
	})

	// failure cases
	t.Run("errors on a block that is not a config block", func(t *testing.T) {
		t.Parallel()
		_, err := load(&user.Config{MSP: user.MSPConfig{LocalMspID: "org1", LocalMspDir: t.TempDir()}},
			&cb.Block{}, factory.GetDefault())
		require.Error(t, err)
	})

	t.Run("errors when the MSP directory cannot be loaded", func(t *testing.T) {
		t.Parallel()
		sharedConfig := &ordererpb.SharedConfig{
			PartiesConfig: []*ordererpb.PartyConfig{{
				PartyID:         1,
				TLSCACerts:      [][]byte{[]byte("ca-1")},
				RouterConfig:    &ordererpb.RouterNodeConfig{Host: "router1.example.com", Port: 8013},
				AssemblerConfig: &ordererpb.AssemblerNodeConfig{Host: "assembler1.example.com", Port: 8011},
			}},
		}
		block, _, _ := newConfigBlockWithMSP(t, "arma", sharedConfig)

		_, err := load(&user.Config{MSP: user.MSPConfig{LocalMspID: "org1", LocalMspDir: t.TempDir()}},
			block, factory.GetDefault())
		require.ErrorContains(t, err, "failed to load local MSP")
	})
}

// TestLoadFromFiles asserts the file-based loader reads the user configuration
// YAML and the config block from disk and builds a client for the network the
// block describes.
func TestLoadFromFiles(t *testing.T) {
	t.Parallel()

	sharedConfig := &ordererpb.SharedConfig{
		PartiesConfig: []*ordererpb.PartyConfig{{
			PartyID:         1,
			TLSCACerts:      [][]byte{[]byte("ca-1")},
			RouterConfig:    &ordererpb.RouterNodeConfig{Host: "router1.example.com", Port: 8013},
			AssemblerConfig: &ordererpb.AssemblerNodeConfig{Host: "assembler1.example.com", Port: 8011},
		}},
	}
	block, mspDir, mspID := newConfigBlockWithMSP(t, "arma", sharedConfig)
	blockPath := filepath.Join(t.TempDir(), "current.pb")
	raw, err := proto.Marshal(block)
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(blockPath, raw, 0o600))

	configPath := filepath.Join(t.TempDir(), "admin.yaml")
	writeUserConfig(t, configPath, mspID, mspDir)

	c, err := LoadFromFiles(configPath, blockPath, factory.GetDefault())
	require.NoError(t, err)
	require.Equal(t, "arma", c.ChannelID())
	require.Equal(t, []string{"router1.example.com:8013"}, c.ordererConnInfo.RouterEndpoints)
}

// TestLoadFromFilesErrors asserts the loader reports readable errors for a
// missing configuration file and an unreadable config block.
func TestLoadFromFilesErrors(t *testing.T) {
	t.Parallel()

	configPath := filepath.Join(t.TempDir(), "admin.yaml")
	writeUserConfig(t, configPath, "org1", t.TempDir())
	notABlock := filepath.Join(t.TempDir(), "current.pb")
	require.NoError(t, os.WriteFile(notABlock, []byte("not a block"), 0o600))

	t.Run("missing config file", func(t *testing.T) {
		t.Parallel()
		_, err := LoadFromFiles(filepath.Join(t.TempDir(), "absent.yaml"), notABlock, factory.GetDefault())
		require.ErrorContains(t, err, "failed to read user configuration")
	})

	t.Run("unreadable config block", func(t *testing.T) {
		t.Parallel()
		_, err := LoadFromFiles(configPath, notABlock, factory.GetDefault())
		require.ErrorContains(t, err, "failed to unmarshal config block")
	})
}

func TestFetchBlockNoAssemblerEndpoints(t *testing.T) {
	t.Parallel()
	c := &Client{ordererConnInfo: &ordererconn.Info{}}
	block, err := c.FetchBlock(&ab.SeekInfo{})
	require.Nil(t, block)
	require.ErrorContains(t, err, "no assembler endpoints configured")
}

func TestBroadcastToAllRoutersNoRouterEndpoints(t *testing.T) {
	t.Parallel()
	c := &Client{ordererConnInfo: &ordererconn.Info{}}
	statuses, err := c.BroadcastToAllRouters(&cb.Envelope{})
	require.Nil(t, statuses)
	require.ErrorContains(t, err, "no router endpoints configured")
}

// TestBroadcastToAllRoutersCollectsStatuses asserts the client sends the
// envelope to every configured router and returns one RouterStatus per router,
// in endpoint order, with Err nil for routers that acknowledge SUCCESS and a
// non-nil Err for routers that reject or are unreachable.
func TestBroadcastToAllRoutersCollectsStatuses(t *testing.T) {
	t.Parallel()

	acking := startBroadcastServer(t, cb.Status_SUCCESS)
	rejecting := startBroadcastServer(t, cb.Status_BAD_REQUEST)
	unreachable := freeAddress(t)

	c := &Client{
		ordererConnInfo: &ordererconn.Info{
			RouterEndpoints: []string{acking, rejecting, unreachable},
		},
		clientConfig: comm.ClientConfig{DialTimeout: 5 * time.Second},
	}

	statuses, err := c.BroadcastToAllRouters(&cb.Envelope{})
	require.NoError(t, err)
	require.Len(t, statuses, 3)

	require.Equal(t, acking, statuses[0].Endpoint)
	require.NoError(t, statuses[0].Err)

	require.Equal(t, rejecting, statuses[1].Endpoint)
	require.ErrorContains(t, statuses[1].Err, cb.Status_BAD_REQUEST.String())

	require.Equal(t, unreachable, statuses[2].Endpoint)
	require.Error(t, statuses[2].Err)
	require.ErrorContains(t, statuses[2].Err, "connection refused")
}

func TestNewClientConfig(t *testing.T) {
	t.Parallel()

	caCerts := [][]byte{[]byte("ca-1"), []byte("ca-2")}
	info := &ordererconn.Info{TLSCACerts: caCerts}

	t.Run("TLS disabled", func(t *testing.T) {
		t.Parallel()
		cc, err := newClientConfig(&user.Config{}, info)
		require.NoError(t, err)
		require.False(t, cc.SecOpts.UseTLS)
		require.False(t, cc.SecOpts.RequireClientCert)
	})

	t.Run("server-only TLS sets root CAs from the block", func(t *testing.T) {
		t.Parallel()
		config := &user.Config{TLS: user.TLSConfig{Enabled: enabled()}}
		cc, err := newClientConfig(config, info)
		require.NoError(t, err)
		require.True(t, cc.SecOpts.UseTLS)
		require.False(t, cc.SecOpts.RequireClientCert)
		require.Equal(t, caCerts, cc.SecOpts.ServerRootCAs)
	})

	t.Run("mutual TLS loads client cert and key", func(t *testing.T) {
		t.Parallel()
		ckp := writeClientCertKeyPair(t)
		config := &user.Config{TLS: user.TLSConfig{
			Enabled: enabled(), ClientCert: ckp.certPath, ClientKey: ckp.keyPath,
		}}
		cc, err := newClientConfig(config, info)
		require.NoError(t, err)
		require.True(t, cc.SecOpts.UseTLS)
		require.True(t, cc.SecOpts.RequireClientCert)
		require.Equal(t, ckp.certPEM, cc.SecOpts.Certificate)
		require.Equal(t, ckp.keyPEM, cc.SecOpts.Key)
	})

	// failure cases
	t.Run("TLS enabled without CA certs errors", func(t *testing.T) {
		t.Parallel()
		config := &user.Config{TLS: user.TLSConfig{Enabled: enabled()}}
		_, err := newClientConfig(config, &ordererconn.Info{})
		require.ErrorContains(t, err, "no TLS CA certificates")
	})

	t.Run("missing client cert file errors", func(t *testing.T) {
		t.Parallel()
		config := &user.Config{TLS: user.TLSConfig{
			Enabled:    enabled(),
			ClientCert: filepath.Join(t.TempDir(), "absent.crt"),
			ClientKey:  filepath.Join(t.TempDir(), "absent.key"),
		}}
		_, err := newClientConfig(config, info)
		require.ErrorContains(t, err, "failed to read file")
	})

	t.Run("missing client key file errors when the cert is present", func(t *testing.T) {
		t.Parallel()
		ckp := writeClientCertKeyPair(t)
		config := &user.Config{TLS: user.TLSConfig{
			Enabled:    enabled(),
			ClientCert: ckp.certPath,
			ClientKey:  filepath.Join(t.TempDir(), "absent.key"),
		}}
		_, err := newClientConfig(config, info)
		require.ErrorContains(t, err, "failed to read file")
	})
}

func TestTLSCertHash(t *testing.T) {
	t.Parallel()

	t.Run("nil when not mutual TLS", func(t *testing.T) {
		t.Parallel()
		config := &user.Config{TLS: user.TLSConfig{Enabled: enabled()}}
		cc, err := newClientConfig(config, &ordererconn.Info{TLSCACerts: [][]byte{[]byte("ca")}})
		require.NoError(t, err)
		hash, err := tlsCertHash(cc)
		require.NoError(t, err)
		require.Nil(t, hash)
	})

	t.Run("hashes the client certificate under mutual TLS", func(t *testing.T) {
		t.Parallel()
		ckp := writeClientCertKeyPair(t)
		config := &user.Config{TLS: user.TLSConfig{
			Enabled: enabled(), ClientCert: ckp.certPath, ClientKey: ckp.keyPath,
		}}
		cc, err := newClientConfig(config, &ordererconn.Info{TLSCACerts: [][]byte{[]byte("ca")}})
		require.NoError(t, err)
		hash, err := tlsCertHash(cc)
		require.NoError(t, err)
		require.NotNil(t, hash)
	})

	t.Run("errors when the client certificate is not valid PEM", func(t *testing.T) {
		t.Parallel()
		cc := comm.ClientConfig{SecOpts: comm.SecureOptions{
			UseTLS:            true,
			RequireClientCert: true,
			Certificate:       []byte("not a pem certificate"),
		}}
		_, err := tlsCertHash(cc)
		require.ErrorContains(t, err, "failed to decode client TLS certificate PEM")
	})
}

// newConfigBlockWithMSP builds a real ARMA config block whose consensus
// metadata is the marshaled shared config, and returns it together with a
// loadable consenter MSP directory and its MSP ID from the same crypto tree.
func newConfigBlockWithMSP(t *testing.T, channelID string, shared *ordererpb.SharedConfig) (
	block *cb.Block, mspDir, mspID string,
) {
	t.Helper()
	meta, err := proto.Marshal(shared)
	require.NoError(t, err)

	targetPath := t.TempDir()
	block, err = cryptogen.CreateOrExtendConfigBlockWithCrypto(cryptogen.ConfigBlockParameters{
		TargetPath:  targetPath,
		BaseProfile: configtxgen.SampleFabricX,
		ChannelID:   channelID,
		Organizations: []cryptogen.OrganizationParameters{{
			Name:             "orderer-org-1",
			Domain:           "orderer-org-1.com",
			OrdererEndpoints: []*types.OrdererEndpoint{{ID: 1, Host: "localhost", Port: 7050}},
			ConsenterNodes:   []cryptogen.Node{{CommonName: "consenter", Hostname: "consenter"}},
			OrdererNodes:     []cryptogen.Node{{CommonName: "orderer-node", Hostname: "orderer-node"}},
		}},
		ArmaMetaBytes: meta,
	})
	require.NoError(t, err)

	mspDirs := testcrypto.GetConsenterMspDirs(targetPath)
	require.NotEmpty(t, mspDirs)
	return block, mspDirs[0].MspDir, mspDirs[0].MspName
}

// broadcastStub is an in-process AtomicBroadcast server that replies to every
// received envelope with a fixed status.
type broadcastStub struct {
	ab.UnimplementedAtomicBroadcastServer
	status cb.Status
}

// Broadcast replies with the stub's fixed status for each envelope received,
// returning when the client closes the stream.
func (s *broadcastStub) Broadcast(stream ab.AtomicBroadcast_BroadcastServer) error {
	for {
		_, err := stream.Recv()
		if errors.Is(err, io.EOF) {
			return nil // client closed the stream
		}
		if err != nil {
			return err
		}
		if err := stream.Send(&ab.BroadcastResponse{Status: s.status}); err != nil {
			return err
		}
	}
}

// startBroadcastServer starts an in-process AtomicBroadcast server that answers
// every broadcast with status, and returns its "host:port" address. The server
// is stopped when the test ends.
func startBroadcastServer(t *testing.T, status cb.Status) string {
	t.Helper()
	lis, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)

	srv := grpc.NewServer()
	ab.RegisterAtomicBroadcastServer(srv, &broadcastStub{status: status})
	go func() { _ = srv.Serve(lis) }()
	t.Cleanup(srv.Stop)

	return lis.Addr().String()
}

// writeUserConfig writes a minimal user configuration YAML naming the given MSP
// to path.
func writeUserConfig(t *testing.T, path, mspID, mspDir string) {
	t.Helper()
	content, err := yaml.Marshal(user.Config{
		MSP: user.MSPConfig{LocalMspID: mspID, LocalMspDir: mspDir},
	})
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(path, content, 0o600))
}

// freeAddress returns a "host:port" that no server is listening on, so a dial
// to it fails. The listener is closed before returning.
func freeAddress(t *testing.T) string {
	t.Helper()
	lis, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	addr := lis.Addr().String()
	require.NoError(t, lis.Close())
	return addr
}

// clientKeyPair is a client TLS cert/key pair written to disk for a test.
type clientCertKeyPair struct {
	certPath, keyPath string
	certPEM, keyPEM   []byte
}

// writeClientKeyPair generates a client cert/key pair and writes it to temp
// files, returning the paths and PEM bytes.
func writeClientCertKeyPair(t *testing.T) clientCertKeyPair {
	t.Helper()
	ca, err := tlsgen.NewCA()
	require.NoError(t, err)
	pair, err := ca.NewClientCertKeyPair()
	require.NoError(t, err)

	dir := t.TempDir()
	kp := clientCertKeyPair{
		certPath: filepath.Join(dir, "client.crt"),
		keyPath:  filepath.Join(dir, "client.key"),
		certPEM:  pair.Cert,
		keyPEM:   pair.Key,
	}
	require.NoError(t, os.WriteFile(kp.certPath, pair.Cert, 0o600))
	require.NoError(t, os.WriteFile(kp.keyPath, pair.Key, 0o600))
	return kp
}
