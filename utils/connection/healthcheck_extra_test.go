/*
Copyright IBM Corp. All Rights Reserved.

SPDX-License-Identifier: Apache-2.0
*/

package connection_test

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/health"
	healthgrpc "google.golang.org/grpc/health/grpc_health_v1"

	"github.com/hyperledger/fabric-x-common/utils/connection"
	"github.com/hyperledger/fabric-x-common/utils/serve"
	"github.com/hyperledger/fabric-x-common/utils/test"
)

// notServingHealthService registers a health service that reports NOT_SERVING.
type notServingHealthService struct{}

func (notServingHealthService) RegisterService(s serve.Servers) {
	hs := health.NewServer()
	hs.SetServingStatus("", healthgrpc.HealthCheckResponse_NOT_SERVING)
	healthgrpc.RegisterHealthServer(s.GRPC, hs)
}

func TestRunHealthCheckNotServing(t *testing.T) {
	t.Parallel()
	ctx, cancel := context.WithTimeout(t.Context(), 30*time.Second)
	t.Cleanup(cancel)

	serverConfig := test.NewLocalHostServiceConfig(test.InsecureTLSConfig)
	test.ServeForTest(ctx, t, serverConfig, notServingHealthService{})

	err := connection.RunHealthCheck(ctx, serverConfig.GRPC.Endpoint, serverConfig.GRPC.TLS)
	require.ErrorContains(t, err, "NOT_SERVING")
}

func TestRunHealthCheckClientCreationError(t *testing.T) {
	t.Parallel()
	ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
	t.Cleanup(cancel)

	err := connection.RunHealthCheck(ctx,
		connection.Endpoint{Host: connection.DefaultHost, Port: 1},
		connection.TLSConfig{Mode: connection.MutualTLSMode, CACertPaths: []string{missingCertFile}},
	)
	require.ErrorContains(t, err, "failed to create gRPC client")
}
