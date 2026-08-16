/*
Copyright IBM Corp. All Rights Reserved.

SPDX-License-Identifier: Apache-2.0
*/

package serve

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/hyperledger/fabric-x-common/common/crypto/tlsgen"

	"github.com/hyperledger/fabric-x-common/utils/connection"
)

func TestNewTLSProvider(t *testing.T) {
	t.Parallel()

	ca, err := tlsgen.NewCA()
	require.NoError(t, err)
	certPath, keyPath, caPath := writeServerCerts(t, ca)

	for _, tc := range []struct {
		name          string
		config        connection.TLSConfig
		wantNilConfig bool
		wantMutual    bool
	}{
		{
			name:          "none mode has no server config",
			config:        connection.TLSConfig{Mode: connection.NoneTLSMode},
			wantNilConfig: true,
		},
		{
			name: "one-side mode returns a static server config",
			config: connection.TLSConfig{
				Mode:     connection.OneSideTLSMode,
				CertPath: certPath,
				KeyPath:  keyPath,
			},
		},
		{
			name: "mutual mode returns a dynamic server config",
			config: connection.TLSConfig{
				Mode:        connection.MutualTLSMode,
				CertPath:    certPath,
				KeyPath:     keyPath,
				CACertPaths: []string{caPath},
			},
			wantMutual: true,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			provider, err := NewTLSProvider(tc.config)
			require.NoError(t, err)
			require.NotNil(t, provider)

			serverConfig := provider.GetServerTLSCredentials()
			if tc.wantNilConfig {
				require.Nil(t, serverConfig)
				return
			}
			require.NotNil(t, serverConfig)

			if tc.wantMutual {
				// Mutual mode installs a per-handshake callback and hides the static config.
				require.NotNil(t, serverConfig.GetConfigForClient)
				require.Empty(t, serverConfig.Certificates)
			} else {
				// Non-mutual mode exposes the static config directly with the server cert.
				require.Nil(t, serverConfig.GetConfigForClient)
				require.NotEmpty(t, serverConfig.Certificates)
			}
		})
	}
}

func TestNewTLSProviderError(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	garbageCertPath := filepath.Join(dir, "bad-cert.pem")
	garbageKeyPath := filepath.Join(dir, "bad-key.pem")
	require.NoError(t, os.WriteFile(garbageCertPath, []byte("not a certificate"), 0o600))
	require.NoError(t, os.WriteFile(garbageKeyPath, []byte("not a key"), 0o600))

	for _, tc := range []struct {
		name          string
		config        connection.TLSConfig
		errorContains string
	}{
		{
			name:          "unknown TLS mode",
			config:        connection.TLSConfig{Mode: "bogus"},
			errorContains: "unknown TLS mode",
		},
		{
			name: "missing certificate file",
			config: connection.TLSConfig{
				Mode:     connection.OneSideTLSMode,
				CertPath: filepath.Join(dir, "does-not-exist.pem"),
				KeyPath:  garbageKeyPath,
			},
			errorContains: "failed to load certificate",
		},
		{
			name: "invalid certificate contents",
			config: connection.TLSConfig{
				Mode:     connection.OneSideTLSMode,
				CertPath: garbageCertPath,
				KeyPath:  garbageKeyPath,
			},
			errorContains: "failed to load server certificates",
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			provider, err := NewTLSProvider(tc.config)
			require.ErrorContains(t, err, tc.errorContains)
			require.Nil(t, provider)
		})
	}
}

func TestDynamicTLSUpdaterLoad(t *testing.T) {
	t.Parallel()

	var updater DynamicTLSUpdater
	require.Nil(t, updater.Load())

	certs := [][]byte{[]byte("cert-a"), []byte("cert-b")}
	updater.UpdateClientRootCAs(certs)
	require.Equal(t, certs, updater.Load())
}

func TestUpdateNoLockGuards(t *testing.T) {
	t.Parallel()

	ca, err := tlsgen.NewCA()
	require.NoError(t, err)
	certPath, keyPath, caPath := writeServerCerts(t, ca)

	for _, tc := range []struct {
		name            string
		config          connection.TLSConfig
		registerUpdater bool
	}{
		{
			name: "mutual provider without a registered updater does nothing",
			config: connection.TLSConfig{
				Mode:        connection.MutualTLSMode,
				CertPath:    certPath,
				KeyPath:     keyPath,
				CACertPaths: []string{caPath},
			},
		},
		{
			name: "one-side provider has no static cert pool to extend",
			config: connection.TLSConfig{
				Mode:     connection.OneSideTLSMode,
				CertPath: certPath,
				KeyPath:  keyPath,
			},
			registerUpdater: true,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			provider, err := NewTLSProvider(tc.config)
			require.NoError(t, err)

			if tc.registerUpdater {
				var updater DynamicTLSUpdater
				RegisterDynamicTLSUpdater(provider, &updater)
				updater.UpdateClientRootCAs([][]byte{ca.CertBytes()})
			}
			require.False(t, provider.updateNoLock())
		})
	}
}

// writeServerCerts writes a server cert/key pair and the signing CA cert to a
// temp dir, returning their file paths.
func writeServerCerts(t *testing.T, ca tlsgen.CA) (certPath, keyPath, caPath string) {
	t.Helper()
	keyPair, err := ca.NewServerCertKeyPair(localHost)
	require.NoError(t, err)

	dir := t.TempDir()
	certPath = filepath.Join(dir, "cert.pem")
	keyPath = filepath.Join(dir, "key.pem")
	caPath = filepath.Join(dir, "ca.pem")
	require.NoError(t, os.WriteFile(certPath, keyPair.Cert, 0o600))
	require.NoError(t, os.WriteFile(keyPath, keyPair.Key, 0o600))
	require.NoError(t, os.WriteFile(caPath, ca.CertBytes(), 0o600))
	return certPath, keyPath, caPath
}
