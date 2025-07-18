/*
Copyright IBM Corp. All Rights Reserved.

SPDX-License-Identifier: Apache-2.0
*/

package blocksprovider

import (
	"context"

	"github.com/hyperledger/fabric-protos-go-apiv2/orderer"
	"google.golang.org/grpc"

	"github.com/hyperledger/fabric-x-common/internaltools/pkg/comm"
)

type DialerAdapter struct {
	ClientConfig comm.ClientConfig
}

func (da DialerAdapter) Dial(address string, rootCerts [][]byte) (*grpc.ClientConn, error) {
	cc := da.ClientConfig
	cc.SecOpts.ServerRootCAs = rootCerts
	return cc.Dial(address)
}

type DeliverAdapter struct{}

func (DeliverAdapter) Deliver(ctx context.Context, clientConn *grpc.ClientConn) (orderer.AtomicBroadcast_DeliverClient, error) {
	return orderer.NewAtomicBroadcastClient(clientConn).Deliver(ctx)
}
