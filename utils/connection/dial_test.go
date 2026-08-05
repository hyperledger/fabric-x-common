/*
Copyright IBM Corp. All Rights Reserved.

SPDX-License-Identifier: Apache-2.0
*/

package connection_test

import (
	"context"
	"net"
	"testing"
	"time"

	"github.com/cockroachdb/errors"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	healthgrpc "google.golang.org/grpc/health/grpc_health_v1"

	"github.com/hyperledger/fabric-x-common/utils/connection"
	"github.com/hyperledger/fabric-x-common/utils/retry"
	"github.com/hyperledger/fabric-x-common/utils/test"
)

// missingCertFile is an absolute path that is guaranteed not to exist, used to trigger
// certificate-loading failures when building TLS credentials.
const missingCertFile = "/nonexistent/connection-test/missing.pem"

func TestNewDialInfo(t *testing.T) {
	t.Parallel()

	profile := &retry.Profile{MaxElapsedTime: new(5 * time.Second)}
	endpoints := []*connection.Endpoint{{Host: connection.DefaultHost, Port: 7050}}
	d, err := connection.NewDialInfo(&connection.MultiClientConfig{
		Endpoints: endpoints,
		TLS:       connection.TLSConfig{Mode: connection.NoneTLSMode},
		Retry:     profile,
	})
	require.NoError(t, err)
	require.NotNil(t, d)
	require.Equal(t, endpoints, d.Endpoints)
	require.Equal(t, profile, d.Retry)
	require.Equal(t, connection.NoneTLSMode, d.TLS.Mode)

	for _, tc := range []struct {
		name    string
		config  *connection.MultiClientConfig
		wantErr string
	}{
		{
			name:    "unknown mode",
			config:  &connection.MultiClientConfig{TLS: connection.TLSConfig{Mode: "bogus"}},
			wantErr: "unknown TLS mode",
		},
		{
			name: "missing CA cert",
			config: &connection.MultiClientConfig{
				TLS: connection.TLSConfig{Mode: connection.MutualTLSMode, CACertPaths: []string{missingCertFile}},
			},
			wantErr: "failed to load root CA cert",
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got, err := connection.NewDialInfo(tc.config)
			require.ErrorContains(t, err, tc.wantErr)
			require.Nil(t, got)
		})
	}
}

func TestNewConnectionSuccess(t *testing.T) {
	t.Parallel()
	conn, err := connection.NewConnection(connection.ClientParameters{
		Address: net.JoinHostPort(connection.DefaultHost, "12345"),
		Creds:   insecure.NewCredentials(),
	})
	require.NoError(t, err)
	require.NotNil(t, conn)
	require.NoError(t, conn.Close())
}

func TestNewConnectionErrors(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name    string
		params  connection.ClientParameters
		wantErr string
	}{
		{
			name:    "invalid target",
			params:  connection.ClientParameters{Address: "\x00bad", Creds: insecure.NewCredentials()},
			wantErr: "error connecting to grpc",
		},
		{
			name:    "no transport security",
			params:  connection.ClientParameters{Address: net.JoinHostPort(connection.DefaultHost, "12345")},
			wantErr: "error connecting to grpc",
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			conn, err := connection.NewConnection(tc.params)
			require.ErrorContains(t, err, tc.wantErr)
			require.Nil(t, conn)
		})
	}
}

func TestNewSingleConnection(t *testing.T) {
	t.Parallel()
	ctx, cancel := context.WithTimeout(t.Context(), 30*time.Second)
	t.Cleanup(cancel)

	serverConfig := test.NewLocalHostServiceConfig(test.InsecureTLSConfig)
	test.ServeForTest(ctx, t, serverConfig, nil)

	conn, err := connection.NewSingleConnection(&connection.ClientConfig{
		Endpoint: &serverConfig.GRPC.Endpoint,
		TLS:      serverConfig.GRPC.TLS,
	})
	require.NoError(t, err)
	require.NotNil(t, conn)
	t.Cleanup(func() { connection.CloseConnectionsLog(conn) })
	requireServing(t, conn)

	for _, tc := range []struct {
		name    string
		cfg     *connection.ClientConfig
		wantErr string
	}{
		{
			name: "unknown mode",
			cfg: &connection.ClientConfig{
				Endpoint: &serverConfig.GRPC.Endpoint,
				TLS:      connection.TLSConfig{Mode: "bogus"},
			},
			wantErr: "unknown TLS mode",
		},
		{
			name: "missing CA cert",
			cfg: &connection.ClientConfig{
				Endpoint: &serverConfig.GRPC.Endpoint,
				TLS:      connection.TLSConfig{Mode: connection.MutualTLSMode, CACertPaths: []string{missingCertFile}},
			},
			wantErr: "failed to load root CA cert",
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			conn, err := connection.NewSingleConnection(tc.cfg)
			require.ErrorContains(t, err, tc.wantErr)
			require.Nil(t, conn)
		})
	}
}

func TestNewConnectionPerEndpoint(t *testing.T) {
	t.Parallel()
	ctx, cancel := context.WithTimeout(t.Context(), 30*time.Second)
	t.Cleanup(cancel)

	s1 := test.NewLocalHostServiceConfig(test.InsecureTLSConfig)
	s2 := test.NewLocalHostServiceConfig(test.InsecureTLSConfig)
	test.ServeForTest(ctx, t, s1, nil)
	test.ServeForTest(ctx, t, s2, nil)

	conns, err := connection.NewConnectionPerEndpoint(&connection.MultiClientConfig{
		Endpoints: []*connection.Endpoint{&s1.GRPC.Endpoint, &s2.GRPC.Endpoint},
	})
	require.NoError(t, err)
	require.Len(t, conns, 2)
	t.Cleanup(func() { connection.CloseConnectionsLog(conns...) })
	for _, conn := range conns {
		requireServing(t, conn)
	}

	for _, tc := range []struct {
		name    string
		config  *connection.MultiClientConfig
		wantErr string
	}{
		{
			name: "missing CA cert",
			config: &connection.MultiClientConfig{
				Endpoints: []*connection.Endpoint{&s1.GRPC.Endpoint},
				TLS:       connection.TLSConfig{Mode: connection.MutualTLSMode, CACertPaths: []string{missingCertFile}},
			},
			wantErr: "failed to load root CA cert",
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got, err := connection.NewConnectionPerEndpoint(tc.config)
			require.ErrorContains(t, err, tc.wantErr)
			require.Nil(t, got)
		})
	}
}

func TestDialInfoNewConnectionPerEndpoint(t *testing.T) {
	t.Parallel()
	ctx, cancel := context.WithTimeout(t.Context(), 30*time.Second)
	t.Cleanup(cancel)

	s1 := test.NewLocalHostServiceConfig(test.InsecureTLSConfig)
	test.ServeForTest(ctx, t, s1, nil)

	d := &connection.DialInfo{
		Endpoints: []*connection.Endpoint{&s1.GRPC.Endpoint},
		TLS:       connection.TLSCredentials{Mode: connection.NoneTLSMode},
	}
	conns, err := d.NewConnectionPerEndpoint()
	require.NoError(t, err)
	require.Len(t, conns, 1)
	t.Cleanup(func() { connection.CloseConnectionsLog(conns...) })
	requireServing(t, conns[0])

	for _, tc := range []struct {
		name    string
		dial    *connection.DialInfo
		wantErr string
	}{
		{
			name: "invalid transport credentials",
			dial: &connection.DialInfo{
				Endpoints: []*connection.Endpoint{&s1.GRPC.Endpoint},
				TLS: connection.TLSCredentials{
					Mode: connection.MutualTLSMode,
					Cert: []byte("bad"), Key: []byte("bad"), CACerts: [][]byte{[]byte("bad")},
				},
			},
			wantErr: "failed to load client certificates",
		},
		{
			name: "connection failure closes partial connections",
			dial: &connection.DialInfo{
				Endpoints: []*connection.Endpoint{&s1.GRPC.Endpoint, {Host: "\x00bad"}},
				TLS:       connection.TLSCredentials{Mode: connection.NoneTLSMode},
			},
			wantErr: "error connecting to grpc",
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got, err := tc.dial.NewConnectionPerEndpoint()
			require.ErrorContains(t, err, tc.wantErr)
			require.Nil(t, got)
		})
	}
}

func TestDialInfoNewLoadBalancedConnectionSingleEndpoint(t *testing.T) {
	t.Parallel()
	ctx, cancel := context.WithTimeout(t.Context(), 30*time.Second)
	t.Cleanup(cancel)

	s1 := test.NewLocalHostServiceConfig(test.InsecureTLSConfig)
	test.ServeForTest(ctx, t, s1, nil)

	d := &connection.DialInfo{
		Endpoints: []*connection.Endpoint{&s1.GRPC.Endpoint},
		TLS:       connection.TLSCredentials{Mode: connection.NoneTLSMode},
	}
	conn, err := d.NewLoadBalancedConnection()
	require.NoError(t, err)
	require.NotNil(t, conn)
	t.Cleanup(func() { connection.CloseConnectionsLog(conn) })
	requireServing(t, conn)
}

func TestNewLoadBalancedConnectionErrors(t *testing.T) {
	t.Parallel()
	endpoint := &connection.Endpoint{Host: connection.DefaultHost, Port: 12345}
	for _, tc := range []struct {
		name    string
		config  *connection.MultiClientConfig
		wantErr string
	}{
		{
			name: "unknown mode",
			config: &connection.MultiClientConfig{
				Endpoints: []*connection.Endpoint{endpoint},
				TLS:       connection.TLSConfig{Mode: "bogus"},
			},
			wantErr: "unknown TLS mode",
		},
		{
			name: "missing CA cert",
			config: &connection.MultiClientConfig{
				Endpoints: []*connection.Endpoint{endpoint},
				TLS:       connection.TLSConfig{Mode: connection.MutualTLSMode, CACertPaths: []string{missingCertFile}},
			},
			wantErr: "failed to load root CA cert",
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			conn, err := connection.NewLoadBalancedConnection(tc.config)
			require.ErrorContains(t, err, tc.wantErr)
			require.Nil(t, conn)
		})
	}
}

func TestDialInfoNewLoadBalancedConnectionError(t *testing.T) {
	t.Parallel()
	d := &connection.DialInfo{
		Endpoints: []*connection.Endpoint{{Host: connection.DefaultHost, Port: 12345}},
		TLS: connection.TLSCredentials{
			Mode: connection.MutualTLSMode,
			Cert: []byte("bad"), Key: []byte("bad"), CACerts: [][]byte{[]byte("bad")},
		},
	}
	conn, err := d.NewLoadBalancedConnection()
	require.ErrorContains(t, err, "failed to load client certificates")
	require.Nil(t, conn)
}

func TestCloseConnectionsLogError(t *testing.T) {
	t.Parallel()
	// A non-acceptable close error is returned by CloseConnections.
	require.ErrorContains(t, connection.CloseConnections(&closer{err: errors.New("boom")}), "boom")
	// CloseConnectionsLog logs the error instead of returning it; exercise that branch.
	connection.CloseConnectionsLog(&closer{err: errors.New("boom")})
}

func TestCalcMaxAttemptsClosedForm(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name                                                     string
		initialInterval, maxInterval, multiplier, maxElapsedTime float64
		want                                                     int
	}{
		{
			name:            "production default 15m",
			initialInterval: 0.5, maxInterval: 10, multiplier: 1.5, maxElapsedTime: 900,
			want: 96,
		},
		{
			name:            "production finite 15s",
			initialInterval: 0.5, maxInterval: 10, multiplier: 1.5, maxElapsedTime: 15,
			want: 7,
		},
		{
			name:            "zero budget",
			initialInterval: 0.5, maxInterval: 10, multiplier: 1.5, maxElapsedTime: 0,
			want: 1,
		},
		{
			name:            "flat interval capped phase",
			initialInterval: 1, maxInterval: 1, multiplier: 2, maxElapsedTime: 5,
			want: 6,
		},
		{
			name:            "capped phase after growth",
			initialInterval: 1, maxInterval: 4, multiplier: 2, maxElapsedTime: 100,
			want: 27,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			require.Equal(t, tc.want,
				connection.CalcMaxAttempts(tc.initialInterval, tc.maxInterval, tc.multiplier, tc.maxElapsedTime))
		})
	}
}

func requireServing(t *testing.T, conn *grpc.ClientConn) {
	t.Helper()
	resp, err := healthgrpc.NewHealthClient(conn).Check(t.Context(), &healthgrpc.HealthCheckRequest{})
	require.NoError(t, err)
	require.Equal(t, healthgrpc.HealthCheckResponse_SERVING, resp.Status)
}
