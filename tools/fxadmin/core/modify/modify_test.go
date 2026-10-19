/*
Copyright IBM Corp. All Rights Reserved.

SPDX-License-Identifier: Apache-2.0
*/

package modify_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/hyperledger/fabric-x-common/tools/fxadmin/core/modify"
	"github.com/hyperledger/fabric-x-common/tools/fxadmin/core/modify/change"
)

// TestHandlerNotImplemented asserts every modify command currently returns
// ErrNotImplemented, since the change logic is not implemented yet.
func TestHandlerNotImplemented(t *testing.T) {
	t.Parallel()

	h := modify.New()

	require.ErrorIs(t, h.AppAdd("org.yaml", "next.pb"), modify.ErrNotImplemented)
	require.ErrorIs(t, h.AppRemove("peer2", "next.pb"), modify.ErrNotImplemented)
	require.ErrorIs(t, h.AppKnownCertsAdd("peer1", []string{"c.pem"}, "next.pb"), modify.ErrNotImplemented)
	require.ErrorIs(t, h.AppKnownCertsRemove("peer1", []string{"c.pem"}, "next.pb"), modify.ErrNotImplemented)
	require.ErrorIs(t, h.PartyAdd("party.yaml", "next.pb"), modify.ErrNotImplemented)
	require.ErrorIs(t, h.PartyRemove(5, "next.pb"), modify.ErrNotImplemented)
	require.ErrorIs(t, h.PartyNode(change.Node{Party: 1, Role: "batcher"}), modify.ErrNotImplemented)
	require.ErrorIs(t, h.PartyCA(change.CA{Op: "add", Party: 1}), modify.ErrNotImplemented)
}
