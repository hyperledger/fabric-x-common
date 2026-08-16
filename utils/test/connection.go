/*
Copyright IBM Corp. All Rights Reserved.

SPDX-License-Identifier: Apache-2.0
*/

package test

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"

	"github.com/hyperledger/fabric-x-common/utils/connection"
	"github.com/hyperledger/fabric-x-common/utils/retry"
	"github.com/hyperledger/fabric-x-common/utils/serve"
)

var (
	// InsecureTLSConfig defines an empty tls config.
	InsecureTLSConfig connection.TLSConfig
	// DefaultGrpcRetryProfile defines the retry policy for a gRPC client connection.
	DefaultGrpcRetryProfile retry.Profile
)

// CheckServerStopped returns true if the grpc server listening on a
// given address has been stopped.
func CheckServerStopped(t *testing.T, addr string) bool {
	t.Helper()
	ctx, cancel := context.WithTimeout(t.Context(), 2*time.Second)
	defer cancel()

	conn, err := grpc.DialContext( //nolint:staticcheck
		ctx,
		addr,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithBlock(), //nolint:staticcheck
	)
	if err != nil {
		return true
	}
	_ = conn.Close()
	return false
}

// NewInsecureConnection creates the default connection with insecure credentials.
func NewInsecureConnection(tb testing.TB, endpoint connection.WithAddress) *grpc.ClientConn {
	tb.Helper()
	return NewInsecureConnectionWithRetry(tb, endpoint, DefaultGrpcRetryProfile)
}

// NewInsecureConnectionWithRetry creates the default dial config with insecure credentials.
func NewInsecureConnectionWithRetry(
	tb testing.TB, endpoint connection.WithAddress, retryProfile retry.Profile,
) *grpc.ClientConn {
	tb.Helper()
	conn, err := connection.NewConnection(connection.ClientParameters{
		Address: endpoint.Address(),
		Creds:   insecure.NewCredentials(),
		Retry:   &retryProfile,
	})
	require.NoError(tb, err)
	tb.Cleanup(func() {
		_ = conn.Close()
	})
	return conn
}

// NewInsecureLoadBalancedConnection creates the default connection with insecure credentials.
func NewInsecureLoadBalancedConnection(t *testing.T, endpoints []*connection.Endpoint) *grpc.ClientConn {
	t.Helper()
	conn, err := connection.NewLoadBalancedConnection(&connection.MultiClientConfig{
		Endpoints: endpoints,
		Retry:     &DefaultGrpcRetryProfile,
	})
	require.NoError(t, err)
	t.Cleanup(func() {
		_ = conn.Close()
	})
	return conn
}

// NewLocalHostServiceConfig returns a grpcservice.ServerConfig with both gRPC and monitoring endpoints.
// Both endpoints use "localhost:0" (auto-assigned ports) with the given TLS credentials.
func NewLocalHostServiceConfig(creds connection.TLSConfig) *serve.Config {
	return &serve.Config{
		GRPC:                  *NewLocalHostServer(creds),
		HTTP:                  *NewLocalHostServer(creds),
		ServiceStartupTimeout: serve.DefaultServiceStartupTimeout,
	}
}

// NewLocalHostServer returns a default server config with endpoint "localhost:0" given server credentials.
func NewLocalHostServer(creds connection.TLSConfig) *serve.ServerConfig {
	return &serve.ServerConfig{
		Endpoint: connection.Endpoint{Host: "127.0.0.1"},
		TLS:      creds,
	}
}
