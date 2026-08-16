/*
Copyright IBM Corp. All Rights Reserved.

SPDX-License-Identifier: Apache-2.0
*/

package test

import (
	"context"
	"sync"
	"testing"

	"github.com/cockroachdb/errors"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	healthgrpc "google.golang.org/grpc/health/grpc_health_v1"

	"github.com/hyperledger/fabric-x-common/utils/serve"
)

// HealthService is a test helper that implements grpcservice.Registerer
// and provides a default health check service for gRPC servers in tests.
type HealthService struct {
	healthgrpc.HealthServer
}

// ServeForTest starts a GRPC server and optionally a monitoring server using a register method.
// It handles the cleanup of the servers at the end of a test, and ensure the test is ended
// only when the servers are down.
// It also updates the server config endpoint port to the actual port if the configuration
// did not specify a port.
// The method asserts that the servers did not end with failure.
func ServeForTest(
	ctx context.Context, tb testing.TB, sc *serve.Config, registerer serve.Registerer,
) (stop context.CancelFunc) {
	tb.Helper()

	servers, err := serve.NewServers(ctx, sc)
	tb.Cleanup(servers.Stop)
	require.NoError(tb, err)

	if registerer == nil {
		registerer = &HealthService{HealthServer: serve.DefaultHealthCheckService()}
	}

	var wg sync.WaitGroup
	tb.Cleanup(wg.Wait)

	// The parent error capture the caller stack trace,
	// which helps track the server origin when debugging test failures.
	parentErr := errors.New("parent stack context")
	wg.Go(func() {
		serveErr := servers.Serve(ctx, registerer)
		// We use assert to prevent panicking for cleanup errors.
		if serveErr != nil {
			assert.NoError(tb, errors.WithSecondaryError(serveErr, parentErr))
		}
	})

	_ = context.AfterFunc(ctx, func() {
		servers.Stop()
	})
	return servers.Stop
}

// RegisterService registers the health check service with the gRPC server.
// This implements the grpcservice.Registerer interface.
func (h *HealthService) RegisterService(s serve.Servers) {
	healthgrpc.RegisterHealthServer(s.GRPC, h)
}
