/*
Copyright IBM Corp. All Rights Reserved.

SPDX-License-Identifier: Apache-2.0
*/

package connection

import (
	"crypto/tls"
	"crypto/x509"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/hyperledger/fabric-x-common/common/crypto/tlsgen"
)

// missingFile is an absolute path that is guaranteed not to exist, used to trigger
// os.ReadFile failures when loading certificate material.
const missingFile = "/nonexistent/connection-test/missing.pem"

func TestNewServerTLSCredentialsFailures(t *testing.T) {
	t.Parallel()
	p := setupTestFiles(t)
	for _, tc := range []struct {
		name    string
		cfg     TLSConfig
		wantErr string
	}{
		{
			name:    "unknown mode",
			cfg:     TLSConfig{Mode: "bogus"},
			wantErr: "unknown TLS mode",
		},
		{
			name:    "missing certificate",
			cfg:     TLSConfig{Mode: OneSideTLSMode, CertPath: missingFile, KeyPath: p.key},
			wantErr: "failed to load certificate",
		},
		{
			name:    "missing private key",
			cfg:     TLSConfig{Mode: OneSideTLSMode, CertPath: p.cert, KeyPath: missingFile},
			wantErr: "failed to load private key",
		},
		{
			name: "missing CA cert",
			cfg: TLSConfig{
				Mode: MutualTLSMode, CertPath: p.cert, KeyPath: p.key, CACertPaths: []string{missingFile},
			},
			wantErr: "failed to load root CA cert",
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			creds, err := NewServerTLSCredentials(tc.cfg)
			require.ErrorContains(t, err, tc.wantErr)
			require.Nil(t, creds)
		})
	}
}

func TestNewClientTLSCredentialsFailures(t *testing.T) {
	t.Parallel()
	p := setupTestFiles(t)
	for _, tc := range []struct {
		name    string
		cfg     TLSConfig
		wantErr string
	}{
		{
			name:    "unknown mode",
			cfg:     TLSConfig{Mode: "bogus"},
			wantErr: "unknown TLS mode",
		},
		{
			name:    "missing CA cert",
			cfg:     TLSConfig{Mode: OneSideTLSMode, CACertPaths: []string{missingFile}},
			wantErr: "failed to load root CA cert",
		},
		{
			name: "missing client certificate",
			cfg: TLSConfig{
				Mode: MutualTLSMode, CACertPaths: []string{p.ca}, CertPath: missingFile, KeyPath: p.key,
			},
			wantErr: "failed to load client certificate",
		},
		{
			name: "missing client private key",
			cfg: TLSConfig{
				Mode: MutualTLSMode, CACertPaths: []string{p.ca}, CertPath: p.cert, KeyPath: missingFile,
			},
			wantErr: "failed to load client private key",
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			creds, err := NewClientTLSCredentials(tc.cfg)
			require.ErrorContains(t, err, tc.wantErr)
			require.Nil(t, creds)
		})
	}
}

func TestCreateServerTLSConfigSuccess(t *testing.T) {
	t.Parallel()
	cert, key, ca := newCertMaterial(t)
	for _, tc := range []struct {
		name       string
		creds      *TLSCredentials
		wantNil    bool
		wantAuth   tls.ClientAuthType
		wantCerts  int
		wantCAPool bool
	}{
		{name: "none", creds: &TLSCredentials{Mode: NoneTLSMode}, wantNil: true},
		{name: "unmentioned", creds: &TLSCredentials{Mode: UnmentionedTLSMode}, wantNil: true},
		{
			name:      "server-side TLS",
			creds:     &TLSCredentials{Mode: OneSideTLSMode, Cert: cert, Key: key},
			wantAuth:  tls.NoClientCert,
			wantCerts: 1,
		},
		{
			name:       "mutual TLS",
			creds:      &TLSCredentials{Mode: MutualTLSMode, Cert: cert, Key: key, CACerts: [][]byte{ca}},
			wantAuth:   tls.RequireAndVerifyClientCert,
			wantCerts:  1,
			wantCAPool: true,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			cfg, err := tc.creds.CreateServerTLSConfig()
			require.NoError(t, err)
			if tc.wantNil {
				require.Nil(t, cfg)
				return
			}
			require.NotNil(t, cfg)
			require.Equal(t, uint16(DefaultTLSMinVersion), cfg.MinVersion)
			require.Len(t, cfg.Certificates, tc.wantCerts)
			require.Equal(t, tc.wantAuth, cfg.ClientAuth)
			require.Equal(t, tc.wantCAPool, cfg.ClientCAs != nil)
		})
	}
}

func TestCreateServerTLSConfigFailures(t *testing.T) {
	t.Parallel()
	cert, key, _ := newCertMaterial(t)
	for _, tc := range []struct {
		name    string
		creds   *TLSCredentials
		wantErr string
	}{
		{name: "unknown mode", creds: &TLSCredentials{Mode: "bogus"}, wantErr: "unknown TLS mode"},
		{
			name:    "invalid server certificates",
			creds:   &TLSCredentials{Mode: OneSideTLSMode, Cert: []byte("bad"), Key: []byte("bad")},
			wantErr: "failed to load server certificates",
		},
		{
			name:    "mutual TLS without CA",
			creds:   &TLSCredentials{Mode: MutualTLSMode, Cert: cert, Key: key},
			wantErr: "no CA certificates provided",
		},
		{
			name:    "mutual TLS invalid CA",
			creds:   &TLSCredentials{Mode: MutualTLSMode, Cert: cert, Key: key, CACerts: [][]byte{[]byte("bad")}},
			wantErr: "unable to parse CA cert",
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			cfg, err := tc.creds.CreateServerTLSConfig()
			require.ErrorContains(t, err, tc.wantErr)
			require.Nil(t, cfg)
		})
	}
}

func TestCreateClientTLSConfigSuccess(t *testing.T) {
	t.Parallel()
	cert, key, ca := newCertMaterial(t)
	for _, tc := range []struct {
		name        string
		creds       *TLSCredentials
		wantNil     bool
		wantCerts   int
		wantRootCAs bool
	}{
		{name: "none", creds: &TLSCredentials{Mode: NoneTLSMode}, wantNil: true},
		{name: "unmentioned", creds: &TLSCredentials{Mode: UnmentionedTLSMode}, wantNil: true},
		{
			name:        "server-side TLS",
			creds:       &TLSCredentials{Mode: OneSideTLSMode, CACerts: [][]byte{ca}},
			wantCerts:   0,
			wantRootCAs: true,
		},
		{
			name:        "mutual TLS",
			creds:       &TLSCredentials{Mode: MutualTLSMode, Cert: cert, Key: key, CACerts: [][]byte{ca}},
			wantCerts:   1,
			wantRootCAs: true,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			cfg, err := tc.creds.CreateClientTLSConfig()
			require.NoError(t, err)
			if tc.wantNil {
				require.Nil(t, cfg)
				return
			}
			require.NotNil(t, cfg)
			require.Equal(t, uint16(DefaultTLSMinVersion), cfg.MinVersion)
			require.Len(t, cfg.Certificates, tc.wantCerts)
			require.Equal(t, tc.wantRootCAs, cfg.RootCAs != nil)
		})
	}
}

func TestCreateClientTLSConfigFailures(t *testing.T) {
	t.Parallel()
	_, _, ca := newCertMaterial(t)
	for _, tc := range []struct {
		name    string
		creds   *TLSCredentials
		wantErr string
	}{
		{name: "unknown mode", creds: &TLSCredentials{Mode: "bogus"}, wantErr: "unknown TLS mode"},
		{
			name: "invalid client certificates",
			creds: &TLSCredentials{
				Mode: MutualTLSMode, Cert: []byte("bad"), Key: []byte("bad"), CACerts: [][]byte{ca},
			},
			wantErr: "failed to load client certificates",
		},
		{
			name:    "no CA",
			creds:   &TLSCredentials{Mode: OneSideTLSMode},
			wantErr: "no CA certificates provided",
		},
		{
			name:    "invalid CA",
			creds:   &TLSCredentials{Mode: OneSideTLSMode, CACerts: [][]byte{[]byte("bad")}},
			wantErr: "unable to parse CA cert",
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			cfg, err := tc.creds.CreateClientTLSConfig()
			require.ErrorContains(t, err, tc.wantErr)
			require.Nil(t, cfg)
		})
	}
}

func TestBuildCertPoolSuccess(t *testing.T) {
	t.Parallel()
	_, _, ca := newCertMaterial(t)
	for _, tc := range []struct {
		name string
		cas  [][]byte
	}{
		{name: "single CA", cas: [][]byte{ca}},
		{name: "multiple CAs", cas: [][]byte{ca, ca}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			pool, err := BuildCertPool(tc.cas...)
			require.NoError(t, err)
			require.NotNil(t, pool)
		})
	}
}

func TestBuildCertPoolFailures(t *testing.T) {
	t.Parallel()
	_, _, ca := newCertMaterial(t)
	for _, tc := range []struct {
		name    string
		cas     [][]byte
		wantErr string
	}{
		{name: "no CA certificates", cas: nil, wantErr: "no CA certificates provided"},
		{name: "invalid CA", cas: [][]byte{[]byte("bad")}, wantErr: "unable to parse CA cert"},
		{name: "valid and invalid CA", cas: [][]byte{ca, []byte("bad")}, wantErr: "unable to parse CA cert"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			pool, err := BuildCertPool(tc.cas...)
			require.ErrorContains(t, err, tc.wantErr)
			require.Nil(t, pool)
		})
	}
}

func TestExtendCertPool(t *testing.T) {
	t.Parallel()
	_, _, ca := newCertMaterial(t)
	for _, tc := range []struct {
		name   string
		cas    [][]byte
		wantOK bool
	}{
		{name: "valid CA", cas: [][]byte{ca}, wantOK: true},
		{name: "no CAs", cas: nil, wantOK: true},
		{name: "invalid CA", cas: [][]byte{[]byte("bad")}, wantOK: false},
		{name: "valid and invalid CA", cas: [][]byte{ca, []byte("bad")}, wantOK: false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			require.Equal(t, tc.wantOK, ExtendCertPool(x509.NewCertPool(), tc.cas...))
		})
	}
}

func TestNewServerGRPCTransportCredentialsSuccess(t *testing.T) {
	t.Parallel()
	cert, key, _ := newCertMaterial(t)
	for _, tc := range []struct {
		name      string
		creds     *TLSCredentials
		wantProto string
	}{
		{name: "none", creds: &TLSCredentials{Mode: NoneTLSMode}, wantProto: "insecure"},
		{
			name:      "server TLS",
			creds:     &TLSCredentials{Mode: OneSideTLSMode, Cert: cert, Key: key},
			wantProto: "tls",
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got, err := NewServerGRPCTransportCredentials(tc.creds)
			require.NoError(t, err)
			require.NotNil(t, got)
			require.Equal(t, tc.wantProto, got.Info().SecurityProtocol)
		})
	}
}

func TestNewClientGRPCTransportCredentialsSuccess(t *testing.T) {
	t.Parallel()
	_, _, ca := newCertMaterial(t)
	for _, tc := range []struct {
		name      string
		creds     *TLSCredentials
		wantProto string
	}{
		{name: "none", creds: &TLSCredentials{Mode: NoneTLSMode}, wantProto: "insecure"},
		{
			name:      "client TLS",
			creds:     &TLSCredentials{Mode: OneSideTLSMode, CACerts: [][]byte{ca}},
			wantProto: "tls",
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got, err := NewClientGRPCTransportCredentials(tc.creds)
			require.NoError(t, err)
			require.NotNil(t, got)
			require.Equal(t, tc.wantProto, got.Info().SecurityProtocol)
		})
	}
}

func TestNewServerGRPCTransportCredentialsError(t *testing.T) {
	t.Parallel()
	creds := &TLSCredentials{
		Mode: MutualTLSMode, Cert: []byte("bad"), Key: []byte("bad"), CACerts: [][]byte{[]byte("bad")},
	}
	got, err := NewServerGRPCTransportCredentials(creds)
	require.ErrorContains(t, err, "failed to load server certificates")
	require.Nil(t, got)
}

func TestNewClientGRPCTransportCredentialsError(t *testing.T) {
	t.Parallel()
	_, _, ca := newCertMaterial(t)
	creds := &TLSCredentials{
		Mode: MutualTLSMode, Cert: []byte("bad"), Key: []byte("bad"), CACerts: [][]byte{ca},
	}
	got, err := NewClientGRPCTransportCredentials(creds)
	require.ErrorContains(t, err, "failed to load client certificates")
	require.Nil(t, got)
}

func TestServerCredentials(t *testing.T) {
	t.Parallel()
	p := setupTestFiles(t)

	for _, tc := range []struct {
		name      string
		cfg       TLSConfig
		wantProto string
	}{
		{name: "none", cfg: TLSConfig{Mode: NoneTLSMode}, wantProto: "insecure"},
		{
			name:      "server TLS",
			cfg:       TLSConfig{Mode: OneSideTLSMode, CertPath: p.cert, KeyPath: p.key},
			wantProto: "tls",
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got, err := tc.cfg.ServerCredentials()
			require.NoError(t, err)
			require.NotNil(t, got)
			require.Equal(t, tc.wantProto, got.Info().SecurityProtocol)
		})
	}

	for _, tc := range []struct {
		name    string
		cfg     TLSConfig
		wantErr string
	}{
		{name: "unknown mode", cfg: TLSConfig{Mode: "bogus"}, wantErr: "unknown TLS mode"},
		{
			name:    "missing certificate file",
			cfg:     TLSConfig{Mode: OneSideTLSMode, CertPath: missingFile, KeyPath: missingFile},
			wantErr: "failed to load certificate",
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got, err := tc.cfg.ServerCredentials()
			require.ErrorContains(t, err, tc.wantErr)
			require.Nil(t, got)
		})
	}
}

func TestClientCredentials(t *testing.T) {
	t.Parallel()
	p := setupTestFiles(t)

	for _, tc := range []struct {
		name      string
		cfg       TLSConfig
		wantProto string
	}{
		{name: "none", cfg: TLSConfig{Mode: NoneTLSMode}, wantProto: "insecure"},
		{
			name:      "client TLS",
			cfg:       TLSConfig{Mode: OneSideTLSMode, CACertPaths: []string{p.ca}},
			wantProto: "tls",
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got, err := tc.cfg.ClientCredentials()
			require.NoError(t, err)
			require.NotNil(t, got)
			require.Equal(t, tc.wantProto, got.Info().SecurityProtocol)
		})
	}

	for _, tc := range []struct {
		name    string
		cfg     TLSConfig
		wantErr string
	}{
		{name: "unknown mode", cfg: TLSConfig{Mode: "bogus"}, wantErr: "unknown TLS mode"},
		{
			name:    "missing CA file",
			cfg:     TLSConfig{Mode: MutualTLSMode, CACertPaths: []string{missingFile}},
			wantErr: "failed to load root CA cert",
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got, err := tc.cfg.ClientCredentials()
			require.ErrorContains(t, err, tc.wantErr)
			require.Nil(t, got)
		})
	}
}

func newCertMaterial(t *testing.T) (cert, key, ca []byte) {
	t.Helper()
	c, err := tlsgen.NewCA()
	require.NoError(t, err)
	keyPair, err := c.NewServerCertKeyPair(localHost)
	require.NoError(t, err)
	return keyPair.Cert, keyPair.Key, c.CertBytes()
}
