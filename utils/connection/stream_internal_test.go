/*
Copyright IBM Corp. All Rights Reserved.

SPDX-License-Identifier: Apache-2.0
*/

package connection

import (
	"context"
	"io"
	"testing"

	"github.com/cockroachdb/errors"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func TestIsStreamEnd(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name string
		err  error
		want bool
	}{
		{name: "nil", err: nil, want: false},
		{name: "raw EOF", err: io.EOF, want: true},
		{name: "raw context canceled", err: context.Canceled, want: true},
		{name: "raw deadline exceeded", err: context.DeadlineExceeded, want: true},
		{name: "status canceled", err: status.Error(codes.Canceled, "cancelled"), want: true},
		{name: "status deadline exceeded", err: status.Error(codes.DeadlineExceeded, "deadline"), want: true},
		{
			name: "unavailable connection refused",
			err:  status.Error(codes.Unavailable, "connection refused"), want: true,
		},
		{name: "unavailable eof", err: status.Error(codes.Unavailable, "received unexpected EOF"), want: true},
		{
			name: "unavailable connection reset",
			err:  status.Error(codes.Unavailable, "connection reset by peer"), want: true,
		},
		{
			name: "unavailable unrelated message",
			err:  status.Error(codes.Unavailable, "service is overloaded"), want: false,
		},
		{name: "status internal", err: status.Error(codes.Internal, "boom"), want: false},
		{name: "generic error", err: errors.New("boom"), want: false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			require.Equal(t, tc.want, isStreamEnd(tc.err))
		})
	}
}

func TestIsStreamContextEnd(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name string
		err  error
		want bool
	}{
		{name: "nil", err: nil, want: false},
		{name: "raw context canceled", err: context.Canceled, want: true},
		{name: "raw deadline exceeded", err: context.DeadlineExceeded, want: true},
		{name: "raw EOF", err: io.EOF, want: false},
		{name: "status canceled", err: status.Error(codes.Canceled, "cancelled"), want: true},
		{name: "status deadline exceeded", err: status.Error(codes.DeadlineExceeded, "deadline"), want: true},
		{name: "status unavailable", err: status.Error(codes.Unavailable, "connection refused"), want: false},
		{name: "generic error", err: errors.New("boom"), want: false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			require.Equal(t, tc.want, isStreamContextEnd(tc.err))
		})
	}
}
