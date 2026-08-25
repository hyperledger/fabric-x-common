/*
Copyright IBM Corp. All Rights Reserved.

SPDX-License-Identifier: Apache-2.0
*/

// Package follow implements the `fxadmin follow` command, which waits for a
// pending configuration update to commit across all assemblers.
package follow

import (
	"fmt"
	"strings"
	"sync"
	"text/tabwriter"
	"time"

	"github.com/hyperledger/fabric-lib-go/bccsp"
	"github.com/hyperledger/fabric-lib-go/bccsp/factory"
	"github.com/hyperledger/fabric-lib-go/common/flogging"

	"github.com/hyperledger/fabric-x-common/tools/fxadmin/core/client"
)

// pollInterval is how often each assembler's ledger is queried while following.
const pollInterval = time.Second

// status is the follow outcome for a single assembler.
type status string

const (
	statusCommitted   status = "committed"
	statusBehind      status = "behind"
	statusUnreachable status = "unreachable"
)

var logger = flogging.MustGetLogger("fxadmin.follow")

// Handler executes the follow command. It carries the BCCSP used to build the
// channel config bundle when reading connection info from the config block.
type Handler struct {
	csp bccsp.BCCSP
}

// New returns a follow command handler.
func New() *Handler {
	return &Handler{csp: factory.GetDefault()}
}

// Run implements `fxadmin follow`. It reads the assembler endpoints and current
// config sequence from the config block, then polls every assembler until each
// has committed the next config block (sequence current+1) or the timeout
// elapses, and prints a per-assembler summary. A config update changes at most
// one assembler, so the endpoints in the current block still reach the rest;
// assemblers that cannot be reached are reported as unreachable.
func (h *Handler) Run(configPath, currentBlockPath string, timeout time.Duration) error {
	logger.Debugf("follow: config=%s current-block=%s timeout=%s", configPath, currentBlockPath, timeout)

	block, err := client.ReadConfigBlock(currentBlockPath)
	if err != nil {
		return err
	}
	currentSequence, err := client.SequenceFromBlock(block)
	if err != nil {
		return err
	}
	expected := currentSequence + 1

	cl, err := client.LoadFromFiles(configPath, currentBlockPath, h.csp)
	if err != nil {
		return err
	}

	results := pollAll(cl, cl.AssemblerEndpoints(), expected, timeout)

	logger.Infof("%s", summaryLine(results, expected))
	if pending := notCommitted(results, expected); pending > 0 {
		logger.Warnf("timeout of %s elapsed with %d of %d assemblers not yet committed to last config sequence %d",
			timeout, pending, len(results), expected)
	}
	_, _ = fmt.Print(formatSummary(results, expected, timeout))
	return nil
}

// notCommitted returns how many assemblers had not committed a block at the
// expected config sequence.
func notCommitted(results []assemblerResult, expected uint64) int {
	pending := 0
	for _, result := range results {
		if classify(result, expected) != statusCommitted {
			pending++
		}
	}
	return pending
}

// assemblerResult is the ledger status observed at a single assembler: its last
// block number (ledger height indicator) and last config sequence. ok is false
// when the assembler never reported a status before the deadline.
type assemblerResult struct {
	endpoint           string
	lastBlockNumber    uint64
	lastConfigSequence uint64
	ok                 bool
}

// pollAll polls every assembler concurrently until each has committed a config
// block at expected sequence (or a later one) or the timeout elapses, returning one
// result per assembler in endpoint order.
func pollAll(cl *client.Client, endpoints []string, expectedSequence uint64, timeout time.Duration) []assemblerResult {
	deadline := time.Now().Add(timeout)
	results := make([]assemblerResult, len(endpoints))

	var wg sync.WaitGroup
	for i, endpoint := range endpoints {
		wg.Go(func() {
			results[i] = pollAssembler(cl, endpoint, expectedSequence, deadline)
		})
	}
	wg.Wait()
	return results
}

// pollAssembler polls one assembler's last config sequence until it reaches
// expected, or the deadline passes, returning the last sequence it observed.
func pollAssembler(cl *client.Client, endpoint string, expected uint64, deadline time.Time) assemblerResult {
	result := assemblerResult{endpoint: endpoint}
	for {
		ledger, err := cl.FetchLedgerStatus(endpoint)
		if err != nil {
			logger.Debugf("follow: assembler %s: %v", endpoint, err)
		} else {
			result.lastBlockNumber, result.lastConfigSequence, result.ok = ledger.LastBlockNumber, ledger.LastConfigSequence, true
			if ledger.LastConfigSequence >= expected {
				return result
			}
		}

		if time.Now().After(deadline) {
			return result
		}
		time.Sleep(pollInterval)
	}
}

// classify maps an assembler's result to its follow status against the expected
// config sequence.
func classify(result assemblerResult, expected uint64) status {
	switch {
	case !result.ok:
		return statusUnreachable
	case result.lastConfigSequence >= expected:
		return statusCommitted
	default:
		return statusBehind
	}
}

// summaryLine reports the expected sequence and how many assemblers committed a
// block at (or beyond) it.
func summaryLine(results []assemblerResult, expected uint64) string {
	committed := 0
	for _, result := range results {
		if classify(result, expected) == statusCommitted {
			committed++
		}
	}
	return fmt.Sprintf(
		"expected last config sequence: %d, %d out of %d assemblers committed a block with last config sequence %d",
		expected, committed, len(results), expected)
}

// formatSummary renders a table of every assembler, in order, with its last
// block number (ledger height indicator), the last config sequence it reached,
// and its status, preceded by the polling timeout. Assemblers never reached show
// a dash for the block and sequence.
func formatSummary(results []assemblerResult, expected uint64, timeout time.Duration) string {
	var b strings.Builder
	// Writes target the in-memory builder, so they cannot fail.
	_, _ = fmt.Fprintf(&b, "polling timeout: %s\n", timeout)
	w := tabwriter.NewWriter(&b, 0, 0, 2, ' ', 0)
	// Writes target the in-memory builder, so they cannot fail.
	_, _ = fmt.Fprintln(w, "ASSEMBLER\tLAST BLOCK\tLAST CONFIG SEQUENCE\tSTATUS")
	for _, result := range results {
		lastBlock, sequence := "-", "-"
		if result.ok {
			lastBlock = fmt.Sprintf("%d", result.lastBlockNumber)
			sequence = fmt.Sprintf("%d", result.lastConfigSequence)
		}
		_, _ = fmt.Fprintf(w, "%s\t%s\t%s\t%s\n", result.endpoint, lastBlock, sequence, classify(result, expected))
	}
	_ = w.Flush()
	return b.String()
}
