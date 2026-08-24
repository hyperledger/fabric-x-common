/*
Copyright IBM Corp. All Rights Reserved.

SPDX-License-Identifier: Apache-2.0
*/

// Package test provides in-process AtomicBroadcast servers for testing the
// broadcast of envelopes to Fabric-X routers.
package test

import (
	"errors"
	"io"
	"net"
	"testing"

	cb "github.com/hyperledger/fabric-protos-go-apiv2/common"
	ab "github.com/hyperledger/fabric-protos-go-apiv2/orderer"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
)

// stub is an in-process AtomicBroadcast server that replies to every received
// envelope with a fixed status.
type stub struct {
	ab.UnimplementedAtomicBroadcastServer
	status cb.Status
}

// Broadcast replies with the stub's fixed status for each envelope received,
// returning when the client closes the stream.
func (s *stub) Broadcast(stream ab.AtomicBroadcast_BroadcastServer) error {
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

// StartBroadcastServer starts an in-process AtomicBroadcast server that answers
// every broadcast with status, and returns its "host:port" address. The server
// is stopped when the test ends.
func StartBroadcastServer(t *testing.T, status cb.Status) string {
	t.Helper()
	lis, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)

	srv := grpc.NewServer()
	ab.RegisterAtomicBroadcastServer(srv, &stub{status: status})
	go func() { _ = srv.Serve(lis) }()
	t.Cleanup(srv.Stop)

	return lis.Addr().String()
}

// FreeAddress returns a "host:port" that no server is listening on, so a dial
// to it fails. The listener is closed before returning.
func FreeAddress(t *testing.T) string {
	t.Helper()
	lis, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	addr := lis.Addr().String()
	require.NoError(t, lis.Close())
	return addr
}
