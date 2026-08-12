/*
Copyright IBM Corp. All Rights Reserved.

SPDX-License-Identifier: Apache-2.0
*/

package client

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/hyperledger/fabric-lib-go/bccsp/factory"
	cb "github.com/hyperledger/fabric-protos-go-apiv2/common"
	ab "github.com/hyperledger/fabric-protos-go-apiv2/orderer"
	"github.com/stretchr/testify/require"
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
		client, err := Load(config, block, factory.GetDefault())
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
		_, err := Load(&user.Config{MSP: user.MSPConfig{LocalMspID: "org1", LocalMspDir: t.TempDir()}},
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

		_, err := Load(&user.Config{MSP: user.MSPConfig{LocalMspID: "org1", LocalMspDir: t.TempDir()}},
			block, factory.GetDefault())
		require.ErrorContains(t, err, "failed to load local MSP")
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
	err := c.BroadcastToAllRouters(&cb.Envelope{})
	require.ErrorContains(t, err, "no router endpoints configured")
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
