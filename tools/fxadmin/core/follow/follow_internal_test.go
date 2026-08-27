/*
Copyright IBM Corp. All Rights Reserved.

SPDX-License-Identifier: Apache-2.0
*/

package follow

import (
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

// Endpoint addresses reused across the classification and summary tests.
const (
	endpointA = "a:1"
	endpointB = "b:2"
	endpointC = "c:3"
	endpointD = "d:4"
)

// TestClassify asserts an assembler is committed once its last config sequence
// reaches the expected sequence (>=), behind when it is lower, and unreachable
// when it never reported a sequence.
func TestClassify(t *testing.T) {
	t.Parallel()

	require.Equal(t, statusCommitted, classify(assemblerResult{lastConfigSequence: 5, ok: true}, 5))
	// A later config sequence also counts as committed.
	require.Equal(t, statusCommitted, classify(assemblerResult{lastConfigSequence: 6, ok: true}, 5))
	require.Equal(t, statusBehind, classify(assemblerResult{lastConfigSequence: 4, ok: true}, 5))
	require.Equal(t, statusUnreachable, classify(assemblerResult{ok: false}, 5))
}

// TestFormatSummary asserts the summary reports the polling timeout, then a
// header and one row per assembler, in order, reporting each assembler's last
// block number, last config sequence, and status, with dashes for assemblers
// that were never reached.
func TestFormatSummary(t *testing.T) {
	t.Parallel()

	results := []assemblerResult{
		{endpoint: endpointA, lastBlockNumber: 104, lastConfigSequence: 5, ok: true},
		{endpoint: endpointB, lastBlockNumber: 103, lastConfigSequence: 4, ok: true},
		{endpoint: endpointC, ok: false},
	}

	lines := strings.Split(strings.TrimSpace(formatSummary(results, 5, 90*time.Second)), "\n")
	require.Len(t, lines, 5) // timeout + header + three assemblers

	require.Contains(t, lines[0], "polling timeout: 1m30s")
	require.Contains(t, lines[1], "ASSEMBLER")
	require.Contains(t, lines[1], "LAST BLOCK")
	require.Contains(t, lines[1], "LAST CONFIG SEQUENCE")
	require.Contains(t, lines[1], "STATUS")

	require.Regexp(t, `^a:1\s+104\s+5\s+committed$`, lines[2])
	require.Regexp(t, `^b:2\s+103\s+4\s+behind$`, lines[3])
	require.Regexp(t, `^c:3\s+-\s+-\s+unreachable$`, lines[4])
}

// TestNotCommitted asserts notCommitted counts every assembler that has not
// committed a block at the expected sequence, including behind and unreachable.
func TestNotCommitted(t *testing.T) {
	t.Parallel()

	results := []assemblerResult{
		{endpoint: endpointA, lastConfigSequence: 5, ok: true}, // committed
		{endpoint: endpointB, lastConfigSequence: 4, ok: true}, // behind
		{endpoint: endpointC, ok: false},                       // unreachable
	}
	require.Equal(t, 2, notCommitted(results, 5)) // behind + unreachable
	require.Equal(t, 1, notCommitted(results, 4)) // a,b committed; c still unreachable
}

// TestSummaryLine asserts the one-line summary reports the expected sequence and
// how many assemblers committed a block at that sequence.
func TestSummaryLine(t *testing.T) {
	t.Parallel()

	results := []assemblerResult{
		{endpoint: endpointA, lastConfigSequence: 5, ok: true},
		{endpoint: endpointB, lastConfigSequence: 5, ok: true},
		{endpoint: endpointC, lastConfigSequence: 4, ok: true},
		{endpoint: endpointD, ok: false},
	}

	line := summaryLine(results, 5)
	require.Equal(t,
		"expected last config sequence: 5, 2 out of 4 assemblers committed a block with last config sequence 5",
		line)
}
