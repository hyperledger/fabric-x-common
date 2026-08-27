/*
Copyright IBM Corp. All Rights Reserved.

SPDX-License-Identifier: Apache-2.0
*/

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

	"github.com/hyperledger/fabric-x-common/protoutil"
)

// configDeliverStub is an in-process AtomicBroadcast server that emulates an
// assembler ledger for a single config commit: a seek for the newest block
// returns newest (whose last-config metadata points at configIndex), and a seek
// for a specific block returns the config block at configIndex carrying
// configSequence.
type configDeliverStub struct {
	ab.UnimplementedAtomicBroadcastServer
	newest      *cb.Block
	configBlock *cb.Block
}

// Deliver replies to each seek with the block at the sought position: the
// newest block for a newest-position seek, and the block at the requested
// number for a specified-position seek. This stub's specified block is a config
// block, so that seek yields the config; a specified seek for a data block
// would yield a data block. It returns when the client closes the stream.
func (s *configDeliverStub) Deliver(stream ab.AtomicBroadcast_DeliverServer) error {
	for {
		envelope, err := stream.Recv()
		if errors.Is(err, io.EOF) {
			return nil // client closed the stream
		}
		if err != nil {
			return err
		}

		seekInfo := &ab.SeekInfo{}
		_, err = protoutil.UnmarshalEnvelopeOfType(envelope, cb.HeaderType_DELIVER_SEEK_INFO, seekInfo)
		if err != nil {
			return err
		}

		block := s.configBlock
		if _, ok := seekInfo.GetStart().GetType().(*ab.SeekPosition_Newest); ok {
			block = s.newest
		}
		if err := stream.Send(&ab.DeliverResponse{Type: &ab.DeliverResponse_Block{Block: block}}); err != nil {
			return err
		}
	}
}

// ConfigLedger describes the ledger a config-deliver stub emulates: its newest
// block number, the number of the config block that newest points at, and the
// config sequence that config block carries.
type ConfigLedger struct {
	NewestNumber   uint64
	ConfigIndex    uint64
	ConfigSequence uint64
}

// StartConfigDeliverServer starts an in-process AtomicBroadcast server emulating
// the given config ledger, and returns its "host:port" address. The server is
// stopped when the test ends.
func StartConfigDeliverServer(t *testing.T, ledger ConfigLedger) string {
	t.Helper()
	lis, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	ServeConfigDeliver(t, lis, ledger)
	return lis.Addr().String()
}

// ServeConfigDeliver serves the config-deliver stub for the given config ledger
// on lis. It lets callers bind the listener first (learning its address before
// building a config block that references it). The server is stopped when the
// test ends.
func ServeConfigDeliver(t *testing.T, lis net.Listener, ledger ConfigLedger) {
	t.Helper()

	newest := protoutil.NewBlock(ledger.NewestNumber, nil)
	newest.Data = &cb.BlockData{Data: [][]byte{{0x01}}}
	lastConfig := &cb.OrdererBlockMetadata{LastConfig: &cb.LastConfig{Index: ledger.ConfigIndex}}
	newest.Metadata.Metadata[cb.BlockMetadataIndex_SIGNATURES] = protoutil.MarshalOrPanic(&cb.Metadata{
		Value: protoutil.MarshalOrPanic(lastConfig),
	})

	configBlock := configBlockAtSequence(ledger.ConfigIndex, ledger.ConfigSequence)

	srv := grpc.NewServer()
	ab.RegisterAtomicBroadcastServer(srv, &configDeliverStub{newest: newest, configBlock: configBlock})
	go func() { _ = srv.Serve(lis) }()
	t.Cleanup(srv.Stop)
}

// configBlockAtSequence builds a config block numbered number whose config
// envelope carries the given sequence.
func configBlockAtSequence(number, sequence uint64) *cb.Block {
	payload := &cb.Payload{
		Header: &cb.Header{ChannelHeader: protoutil.MarshalOrPanic(&cb.ChannelHeader{
			Type: int32(cb.HeaderType_CONFIG),
		})},
		Data: protoutil.MarshalOrPanic(&cb.ConfigEnvelope{Config: &cb.Config{Sequence: sequence}}),
	}
	envelope := &cb.Envelope{Payload: protoutil.MarshalOrPanic(payload)}
	block := protoutil.NewBlock(number, nil)
	block.Data = &cb.BlockData{Data: [][]byte{protoutil.MarshalOrPanic(envelope)}}
	return block
}
