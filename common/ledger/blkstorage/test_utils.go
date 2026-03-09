/*
Copyright IBM Corp. All Rights Reserved.

SPDX-License-Identifier: Apache-2.0
*/

package blkstorage

import (
	"testing"

	"github.com/stretchr/testify/require"
)

// requireNotFoundError is a test helper that verifies an error is ErrNotFound
// and contains the expected context string.
func requireNotFoundError(tb testing.TB, err error, expectedContext string) {
	tb.Helper()
	require.ErrorIs(tb, err, ErrNotFound)
	require.ErrorContains(tb, err, expectedContext)
}
