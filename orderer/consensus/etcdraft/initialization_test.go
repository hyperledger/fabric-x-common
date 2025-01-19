/*
Copyright IBM Corp. All Rights Reserved.

SPDX-License-Identifier: Apache-2.0
*/

package etcdraft_test

import (
	"testing"

	"github.com/hyperledger/fabric-lib-go/bccsp/sw"
	"github.com/hyperledger/fabric-lib-go/common/metrics/disabled"
	"github.com/stretchr/testify/require"
	"github.ibm.com/decentralized-trust-research/fabricx-config/internaltools/pkg/comm"
	"github.ibm.com/decentralized-trust-research/fabricx-config/orderer/common/cluster"
	"github.ibm.com/decentralized-trust-research/fabricx-config/orderer/common/localconfig"
	"github.ibm.com/decentralized-trust-research/fabricx-config/orderer/consensus/etcdraft"
	"github.ibm.com/decentralized-trust-research/fabricx-config/orderer/consensus/etcdraft/mocks"
)

func TestNewEtcdRaftConsenter(t *testing.T) {
	srv, err := comm.NewGRPCServer("127.0.0.1:0", comm.ServerConfig{})
	require.NoError(t, err)
	defer srv.Stop()
	dialer := &cluster.PredicateDialer{}
	cryptoProvider, err := sw.NewDefaultSecurityLevelWithKeystore(sw.NewDummyKeyStore())
	require.NoError(t, err)
	consenter, metrics := etcdraft.New(
		dialer,
		&localconfig.TopLevel{},
		comm.ServerConfig{
			SecOpts: comm.SecureOptions{
				Certificate: []byte{1, 2, 3},
			},
		},
		srv,
		&mocks.ChainManager{},
		&disabled.Provider{},
		cryptoProvider,
	)

	// Assert that the certificate from the gRPC server was passed to the consenter
	require.Equal(t, []byte{1, 2, 3}, consenter.Cert)
	// Assert that all dependencies for the consenter were populated
	require.NotNil(t, consenter.Communication)
	require.NotNil(t, consenter.ChainManager)
	require.NotNil(t, consenter.ChainSelector)
	require.NotNil(t, consenter.Dispatcher)
	require.NotNil(t, consenter.Logger)
	require.NotNil(t, metrics)
}

func TestNewEtcdRaftConsenterNoSystemChannel(t *testing.T) {
	srv, err := comm.NewGRPCServer("127.0.0.1:0", comm.ServerConfig{})
	require.NoError(t, err)
	defer srv.Stop()
	dialer := &cluster.PredicateDialer{}
	cryptoProvider, err := sw.NewDefaultSecurityLevelWithKeystore(sw.NewDummyKeyStore())
	require.NoError(t, err)
	consenter, metrics := etcdraft.New(
		dialer,
		&localconfig.TopLevel{},
		comm.ServerConfig{
			SecOpts: comm.SecureOptions{
				Certificate: []byte{1, 2, 3},
			},
		},
		srv,
		&mocks.ChainManager{},
		&disabled.Provider{},
		cryptoProvider,
	)

	// Assert that the certificate from the gRPC server was passed to the consenter
	require.Equal(t, []byte{1, 2, 3}, consenter.Cert)
	// Assert that all dependencies for the consenter were populated
	require.NotNil(t, consenter.Communication)
	require.NotNil(t, consenter.ChainManager)
	require.NotNil(t, consenter.ChainSelector)
	require.NotNil(t, consenter.Dispatcher)
	require.NotNil(t, consenter.Logger)
	require.NotNil(t, metrics)
}
