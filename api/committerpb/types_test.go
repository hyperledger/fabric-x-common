/*
Copyright IBM Corp. All Rights Reserved.

SPDX-License-Identifier: Apache-2.0
*/

package committerpb

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestHeightComparison(t *testing.T) {
	t.Parallel()
	require.True(t, AreSameHeight(NewTxRef("tx1", 10, 100), NewTxRef("tx2", 10, 100)))
	require.False(t, AreSameHeight(NewTxRef("tx1", 10, 100), NewTxRef("tx1", 11, 100)))
	require.False(t, AreSameHeight(NewTxRef("tx1", 10, 100), NewTxRef("tx1", 10, 101)))

	require.True(t, AreSameHeight(nil, nil))
	require.False(t, AreSameHeight(NewTxRef("tx1", 10, 100), nil))
	require.False(t, AreSameHeight(nil, NewTxRef("tx1", 10, 100)))

	require.True(t, NewTxRef("tx1", 10, 100).IsHeight(10, 100))
	require.False(t, NewTxRef("tx1", 10, 100).IsHeight(11, 100))
	require.False(t, NewTxRef("tx1", 10, 100).IsHeight(10, 101))
}

func TestSystemNamespaces(t *testing.T) {
	t.Parallel()

	namespaces := SystemNamespaces()
	require.Equal(t, []string{
		MetaNamespaceID,
		ConfigNamespaceID,
		SnapshotNamespaceID,
		CheckpointNamespaceID,
	}, namespaces)

	namespaces[0] = "mutated"
	require.Equal(t, MetaNamespaceID, SystemNamespaces()[0])
}

func TestIsSystemNamespace(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		description string
		nsID        string
		expected    bool
	}{
		{
			description: "meta namespace",
			nsID:        MetaNamespaceID,
			expected:    true,
		},
		{
			description: "config namespace",
			nsID:        ConfigNamespaceID,
			expected:    true,
		},
		{
			description: "snapshot namespace",
			nsID:        SnapshotNamespaceID,
			expected:    true,
		},
		{
			description: "checkpoint namespace",
			nsID:        CheckpointNamespaceID,
			expected:    true,
		},
		{
			description: "application namespace",
			nsID:        "asset",
			expected:    false,
		},
		{
			description: "empty namespace",
			nsID:        "",
			expected:    false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.description, func(t *testing.T) {
			t.Parallel()
			require.Equal(t, tc.expected, IsSystemNamespace(tc.nsID))
		})
	}
}
