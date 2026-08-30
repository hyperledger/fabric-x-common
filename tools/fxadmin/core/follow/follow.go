/*
Copyright IBM Corp. All Rights Reserved.

SPDX-License-Identifier: Apache-2.0
*/

// Package follow implements the `fxadmin follow` command, which waits for a
// pending configuration update to commit across all assemblers.
package follow

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"text/tabwriter"
	"time"

	"github.com/cockroachdb/errors"
	"github.com/hyperledger/fabric-lib-go/bccsp"
	"github.com/hyperledger/fabric-lib-go/bccsp/factory"
	"github.com/hyperledger/fabric-lib-go/common/flogging"
	cb "github.com/hyperledger/fabric-protos-go-apiv2/common"

	"github.com/hyperledger/fabric-x-common/protoutil"
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
//
// Once at least f+1 assemblers report the identical next config block (f being
// the number of faulty parties the network tolerates), that block is trusted —
// f+1 matching copies guarantee at least one came from an honest assembler — and
// written to outputPath, ready to be the --current-block of the next
// reconfiguration. Run returns an error if no such agreement is reached before
// the timeout.
func (h *Handler) Run(configPath, currentBlockPath, outputPath string, timeout time.Duration) error {
	logger.Debugf("follow: config=%s current-block=%s output=%s timeout=%s",
		configPath, currentBlockPath, outputPath, timeout)

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

	quorum := cl.FaultThreshold() + 1
	agreed, count := agreedConfigBlock(results, expected, quorum)
	if agreed == nil {
		return errors.Newf("no config block at last config sequence %d was agreed by a quorum of %d assemblers",
			expected, quorum)
	}
	if err := client.WriteBlock(agreed, outputPath); err != nil {
		return err
	}
	logger.Infof("config block at last config sequence %d agreed by %d assemblers (quorum %d), written to %s",
		expected, count, quorum, outputPath)
	return nil
}

// agreedConfigBlock returns the config block at the expected sequence that a
// quorum of assemblers reported identically, along with the number of
// assemblers that reported it. It returns nil when no block reaches quorum
// copies. Blocks are compared by their header hash, which commits to the block
// number, previous hash, and data hash.
func agreedConfigBlock(results []assemblerResult, expected uint64, quorum int) (*cb.Block, int) {
	counts := make(map[string]int)
	blocks := make(map[string]*cb.Block)
	for _, result := range results {
		if !result.ok || result.lastConfigSequence != expected || result.configBlock == nil {
			continue
		}
		key := string(protoutil.BlockHeaderHash(result.configBlock.GetHeader()))
		counts[key]++
		blocks[key] = result.configBlock
		if counts[key] >= quorum {
			return blocks[key], counts[key]
		}
	}
	return nil, 0
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
// block number (ledger height indicator), last config sequence, and last config
// block. ok is false when the assembler never reported a status before the
// deadline.
type assemblerResult struct {
	endpoint           string
	lastBlockNumber    uint64
	lastConfigSequence uint64
	configBlock        *cb.Block
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
// Each query is bounded by the remaining time until deadline, so a hung
// assembler cannot block past the command's timeout, and the sleep between polls
// is capped to the remaining time.
func pollAssembler(cl *client.Client, endpoint string, expected uint64, deadline time.Time) assemblerResult {
	result := assemblerResult{endpoint: endpoint}
	for {
		ctx, cancel := context.WithDeadline(context.Background(), deadline)
		ledger, err := cl.FetchLedgerStatus(ctx, endpoint)
		cancel()
		if err != nil {
			logger.Debugf("follow: assembler %s: %v", endpoint, err)
		} else {
			result.lastBlockNumber = ledger.LastBlockNumber
			result.lastConfigSequence = ledger.LastConfigSequence
			result.configBlock = ledger.ConfigBlock
			result.ok = true
			if ledger.LastConfigSequence >= expected {
				return result
			}
		}

		remaining := time.Until(deadline)
		if remaining <= 0 {
			return result
		}
		time.Sleep(min(pollInterval, remaining))
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
