/*
Copyright IBM Corp. All Rights Reserved.

SPDX-License-Identifier: Apache-2.0
*/

package cli_test

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/hyperledger/fabric-x-common/tools/fxadmin/core/cli"
)

// Repeated command names, flags, and argument tokens used across the tables.
const (
	cmdLedger = "ledger"
	cmdFollow = "follow"
	subHeight = "height"

	flagConfig       = "--config"
	flagCurrentBlock = "--current-block"
	flagOutput       = "--output"

	refLatest = "latest"

	fileConfigUpdate = "config_update.pb"
	fileEndorsement1 = "endorsed_config_update1.pb"
	fileEndorsement2 = "endorsed_config_update2.pb"
	fileEndorsed     = "endorsed_config_update.pb"
	fileConfigTx     = "config_tx.pb"
)

// call records the handler that was invoked and the values it received, so a
// test can assert that parsing routed the right command with the right inputs
// without performing any file or network I/O.
type call struct {
	handler string
	args    []string
	timeout time.Duration
}

// newHandlers builds a cli.Handlers whose commands all record
// the invocation into invoked.
func newHandlers(invoked *call) cli.Handlers {
	return cli.Handlers{
		Ledger: fakeLedger{invoked},
		Decode: fakeDecode{invoked},
		Update: fakeUpdate{invoked},
		Tx:     fakeTx{invoked},
		Follow: fakeFollow{invoked},
	}
}

type fakeLedger struct{ invoked *call }

func (f fakeLedger) Height(configPath, currentBlockPath string) error {
	*f.invoked = call{handler: "LedgerHeight", args: []string{configPath, currentBlockPath}}
	return nil
}

func (f fakeLedger) Block(configPath, currentBlockPath, reference, outputPath string) error {
	*f.invoked = call{handler: "LedgerBlock", args: []string{configPath, currentBlockPath, reference, outputPath}}
	return nil
}

func (f fakeLedger) Config(configPath, currentBlockPath, reference, outputPath string) error {
	*f.invoked = call{handler: "LedgerConfig", args: []string{configPath, currentBlockPath, reference, outputPath}}
	return nil
}

type fakeDecode struct{ invoked *call }

func (f fakeDecode) Run(blockPath, outputPath string) error {
	*f.invoked = call{handler: "Decode", args: []string{blockPath, outputPath}}
	return nil
}

type fakeUpdate struct{ invoked *call }

func (f fakeUpdate) Run(currentPath, modifiedPath, outputPath string) error {
	*f.invoked = call{handler: "ComputeUpdate", args: []string{currentPath, modifiedPath, outputPath}}
	return nil
}

type fakeTx struct{ invoked *call }

func (f fakeTx) Endorse(inputPath, configPath, outputPath string) error {
	*f.invoked = call{handler: "TxEndorse", args: []string{inputPath, configPath, outputPath}}
	return nil
}

func (f fakeTx) Merge(inputPaths []string, outputPath string) error {
	*f.invoked = call{handler: "TxMerge", args: append(append([]string{}, inputPaths...), outputPath)}
	return nil
}

func (f fakeTx) Prepare(inputPath, configPath, outputPath string) error {
	*f.invoked = call{handler: "TxPrepare", args: []string{inputPath, configPath, outputPath}}
	return nil
}

func (f fakeTx) Submit(inputPath, configPath, currentBlockPath string) error {
	*f.invoked = call{handler: "TxSubmit", args: []string{inputPath, configPath, currentBlockPath}}
	return nil
}

func (f fakeTx) Send(inputPath, configPath, currentBlockPath string) error {
	*f.invoked = call{handler: "TxSend", args: []string{inputPath, configPath, currentBlockPath}}
	return nil
}

type fakeFollow struct{ invoked *call }

func (f fakeFollow) Run(configPath, currentBlockPath string, timeout time.Duration) error {
	*f.invoked = call{handler: "Follow", args: []string{configPath, currentBlockPath}, timeout: timeout}
	return nil
}

// TestRunRoutesToHandler feeds argument vectors through the real command tree
// and asserts each one selects the expected handler with the expected values.
// The placeholder tokens are replaced with real temp-file paths so the ExistingFile flag validation passes.
func TestRunRoutesToHandler(t *testing.T) {
	t.Parallel()

	admin := writeTempFile(t, "admin.yaml")
	currBlock := writeTempFile(t, "current_block.pb")
	currentJSON := writeTempFile(t, "current.json")
	modifiedJSON := writeTempFile(t, "modified.json")
	configUpdate := writeTempFile(t, fileConfigUpdate)
	endorsement1 := writeTempFile(t, fileEndorsement1)
	endorsement2 := writeTempFile(t, fileEndorsement2)
	endorsed := writeTempFile(t, fileEndorsed)
	configTx := writeTempFile(t, fileConfigTx)

	for _, tc := range []struct {
		name        string
		args        []string
		wantHandler string
		wantArgs    []string
		wantTimeout time.Duration
	}{
		{
			name:        "ledger height",
			args:        []string{cmdLedger, flagConfig, admin, flagCurrentBlock, currBlock, subHeight},
			wantHandler: "LedgerHeight",
			wantArgs:    []string{admin, currBlock},
		},
		{
			name: "ledger block latest",
			args: []string{
				cmdLedger,
				flagConfig,
				admin,
				flagCurrentBlock,
				currBlock,
				"block",
				refLatest,
				flagOutput,
				"last_block.pb",
			},
			wantHandler: "LedgerBlock",
			wantArgs:    []string{admin, currBlock, refLatest, "last_block.pb"},
		},
		{
			name: "ledger config latest",
			args: []string{
				cmdLedger,
				flagConfig,
				admin,
				flagCurrentBlock,
				currBlock,
				"config",
				refLatest,
				flagOutput,
				"last_config.pb",
			},
			wantHandler: "LedgerConfig",
			wantArgs:    []string{admin, currBlock, refLatest, "last_config.pb"},
		},
		{
			name:        "decode",
			args:        []string{"decode", "--block", currBlock, flagOutput, "current_config.json"},
			wantHandler: "Decode",
			wantArgs:    []string{currBlock, "current_config.json"},
		},
		{
			name:        "compute-update",
			args:        []string{"compute-update", currentJSON, modifiedJSON, flagOutput, fileConfigUpdate},
			wantHandler: "ComputeUpdate",
			wantArgs:    []string{currentJSON, modifiedJSON, fileConfigUpdate},
		},
		{
			name:        "tx endorse",
			args:        []string{"tx", "endorse", configUpdate, flagConfig, admin, flagOutput, fileEndorsement1},
			wantHandler: "TxEndorse",
			wantArgs:    []string{configUpdate, admin, fileEndorsement1},
		},
		{
			name:        "tx merge multiple",
			args:        []string{"tx", "merge", endorsement1, endorsement2, flagOutput, fileEndorsed},
			wantHandler: "TxMerge",
			wantArgs:    []string{endorsement1, endorsement2, fileEndorsed},
		},
		{
			name:        "tx prepare",
			args:        []string{"tx", "prepare", endorsed, flagConfig, admin, flagOutput, fileConfigTx},
			wantHandler: "TxPrepare",
			wantArgs:    []string{endorsed, admin, fileConfigTx},
		},
		{
			name:        "tx submit",
			args:        []string{"tx", "submit", configTx, flagConfig, admin, flagCurrentBlock, currBlock},
			wantHandler: "TxSubmit",
			wantArgs:    []string{configTx, admin, currBlock},
		},
		{
			name:        "tx send",
			args:        []string{"tx", "send", endorsed, flagConfig, admin, flagCurrentBlock, currBlock},
			wantHandler: "TxSend",
			wantArgs:    []string{endorsed, admin, currBlock},
		},
		{
			name:        "follow",
			args:        []string{cmdFollow, flagConfig, admin, flagCurrentBlock, currBlock, "--timeout", "30s"},
			wantHandler: "Follow",
			wantArgs:    []string{admin, currBlock},
			wantTimeout: 30 * time.Second,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			var invoked call
			c := cli.New(newHandlers(&invoked), "test")

			require.NoError(t, c.Run(tc.args))
			require.NotEmpty(t, invoked.handler, "no handler was invoked")
			require.Equal(t, tc.wantHandler, invoked.handler)
			require.Equal(t, tc.wantArgs, invoked.args)
			require.Equal(t, tc.wantTimeout, invoked.timeout)
		})
	}
}

// TestParseErrors covers argument vectors that must fail before any handler
// runs: missing required flags/args, unknown commands, and bad flag values.
func TestParseErrors(t *testing.T) {
	t.Parallel()

	admin := writeTempFile(t, "admin.yaml")
	currBlock := writeTempFile(t, "current_block.pb")

	for _, tc := range []struct {
		name    string
		args    []string
		wantErr string
	}{
		{
			name:    "missing required current-block",
			args:    []string{cmdLedger, flagConfig, admin, subHeight},
			wantErr: "current-block",
		},
		{
			name:    "missing required subcommand",
			args:    []string{cmdLedger, flagConfig, admin, flagCurrentBlock, currBlock},
			wantErr: "command",
		},
		{
			name:    "unknown command",
			args:    []string{"bogus"},
			wantErr: "bogus",
		},
		{
			name:    "nonexistent admin config file",
			args:    []string{cmdLedger, flagConfig, "/no/such/file.yaml", flagCurrentBlock, currBlock, subHeight},
			wantErr: "file",
		},
		{
			name:    "bad duration for follow",
			args:    []string{cmdFollow, flagConfig, admin, flagCurrentBlock, currBlock, "--timeout", "notaduration"},
			wantErr: "invalid duration",
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			// Handlers are left nil to prove parse failures never reach them
			c := cli.New(cli.Handlers{}, "test")

			err := c.Run(tc.args)
			require.ErrorContains(t, err, tc.wantErr)
		})
	}
}

// writeTempFile creates a file with the given base name in a per-test temp dir
// and returns its path, for use where the CLI validates that a flag points at
// an existing file.
func writeTempFile(t *testing.T, name string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), name)
	require.NoError(t, os.WriteFile(path, []byte("x"), 0o600))
	return path
}
