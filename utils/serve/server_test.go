/*
Copyright IBM Corp. All Rights Reserved.

SPDX-License-Identifier: Apache-2.0
*/

package serve

import (
	"context"
	"net"
	"sync/atomic"
	"testing"
	"time"

	"github.com/cockroachdb/errors"
	"github.com/stretchr/testify/require"
	healthgrpc "google.golang.org/grpc/health/grpc_health_v1"
)

func TestListenRetryExecuteSuccess(t *testing.T) {
	t.Parallel()

	for _, tc := range []struct {
		name     string
		op       func(calls *atomic.Int32) error
		minCalls int32
	}{
		{
			name: "binds a listener on the first attempt",
			op: func(calls *atomic.Int32) error {
				calls.Add(1)
				listener, err := net.Listen("tcp", "127.0.0.1:0")
				if err != nil {
					return err
				}
				return listener.Close()
			},
			minCalls: 1,
		},
		{
			name: "recovers after a transient port conflict",
			op: func(calls *atomic.Int32) error {
				if calls.Add(1) == 1 {
					return errors.New("address already in use")
				}
				return nil
			},
			minCalls: 2,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			var calls atomic.Int32
			err := ListenRetryExecute(t.Context(), func() error {
				return tc.op(&calls)
			})
			require.NoError(t, err)
			require.GreaterOrEqual(t, calls.Load(), tc.minCalls)
		})
	}
}

func TestListenRetryExecuteFailure(t *testing.T) {
	t.Parallel()

	for _, tc := range []struct {
		name          string
		op            func(calls *atomic.Int32) error
		ctxTimeout    time.Duration
		errorContains string
		errorIs       error
		minCalls      int32
	}{
		{
			name: "non-conflict error is permanent and not retried",
			op: func(calls *atomic.Int32) error {
				calls.Add(1)
				return errors.New("permission denied")
			},
			errorContains: "creating listener",
			minCalls:      1,
		},
		{
			name: "persistent port conflict stops when the context expires",
			op: func(calls *atomic.Int32) error {
				calls.Add(1)
				return errors.New("port is already allocated")
			},
			ctxTimeout: 300 * time.Millisecond,
			errorIs:    context.DeadlineExceeded,
			minCalls:   2,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			ctx := contextWithOptionalTimeout(t, tc.ctxTimeout)

			var calls atomic.Int32
			err := ListenRetryExecute(ctx, func() error {
				return tc.op(&calls)
			})
			requireRetryError(t, err, tc.errorContains, tc.errorIs)
			require.GreaterOrEqual(t, calls.Load(), tc.minCalls)
		})
	}
}

func TestDefaultHealthCheckService(t *testing.T) {
	t.Parallel()

	healthcheck := DefaultHealthCheckService()
	require.NotNil(t, healthcheck)

	resp, err := healthcheck.Check(t.Context(), &healthgrpc.HealthCheckRequest{})
	require.NoError(t, err)
	require.Equal(t, healthgrpc.HealthCheckResponse_SERVING, resp.GetStatus())
}

// contextWithOptionalTimeout returns the test context, wrapped with a timeout when one is given.
func contextWithOptionalTimeout(t *testing.T, timeout time.Duration) context.Context {
	t.Helper()
	if timeout <= 0 {
		return t.Context()
	}
	ctx, cancel := context.WithTimeout(t.Context(), timeout)
	t.Cleanup(cancel)
	return ctx
}

// requireRetryError asserts the error is non-nil and matches the optional substring and sentinel.
func requireRetryError(t *testing.T, err error, errorContains string, errorIs error) {
	t.Helper()
	require.Error(t, err)
	if errorContains != "" {
		require.ErrorContains(t, err, errorContains)
	}
	if errorIs != nil {
		require.ErrorIs(t, err, errorIs)
	}
}
