/*
Copyright IBM Corp. All Rights Reserved.

SPDX-License-Identifier: Apache-2.0
*/

package cli_test

import (
	"os"
	"path/filepath"
	"strconv"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/hyperledger/fabric-x-common/tools/fxadmin/core/cli"
)

// Repeated command names, flags, and argument tokens used across the tables.
const (
	cmdLedger = "ledger"
	cmdFollow = "follow"
	cmdModify = "modify"
	subHeight = "height"
	subRemove = "remove"
	subAdd    = "add"
	subApp    = "app"
	subParty  = "party"

	flagConfig       = "--config"
	flagCurrentBlock = "--current-block"
	flagOutput       = "--output"
	flagBlock        = "--block"
	flagOrg          = "--org"
	flagParty        = "--party"
	flagCert         = "--cert"

	refLatest = "latest"
	orgPeer1  = "peer1"

	fileConfigUpdate = "config_update.pb"
	fileEndorsement1 = "endorsed_config_update1.pb"
	fileEndorsement2 = "endorsed_config_update2.pb"
	fileEndorsed     = "endorsed_config_update.pb"
	fileConfigTx     = "config_tx.pb"
	fileNextConfig   = "next_config.pb"
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
		Modify: fakeModify{invoked},
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

func (f fakeUpdate) Run(currentPath, modifiedPath, currentBlockPath, outputPath string) error {
	*f.invoked = call{
		handler: "ComputeUpdate",
		args:    []string{currentPath, modifiedPath, currentBlockPath, outputPath},
	}
	return nil
}

func (f fakeUpdate) RunFromBlocks(currentBlockPath, nextBlockPath, outputPath string) error {
	*f.invoked = call{
		handler: "ComputeUpdatePB",
		args:    []string{currentBlockPath, nextBlockPath, outputPath},
	}
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

func (f fakeTx) Send(inputPath, configPath, currentBlockPath, outputPath string) error {
	*f.invoked = call{handler: "TxSend", args: []string{inputPath, configPath, currentBlockPath, outputPath}}
	return nil
}

type fakeFollow struct{ invoked *call }

func (f fakeFollow) Run(configPath, currentBlockPath, outputPath string, timeout time.Duration) error {
	*f.invoked = call{handler: "Follow", args: []string{configPath, currentBlockPath, outputPath}, timeout: timeout}
	return nil
}

type fakeModify struct{ invoked *call }

func (f fakeModify) AppAdd(orgPath, blockPath string) error {
	*f.invoked = call{handler: "ModifyAppAdd", args: []string{orgPath, blockPath}}
	return nil
}

func (f fakeModify) AppRemove(org, blockPath string) error {
	*f.invoked = call{handler: "ModifyAppRemove", args: []string{org, blockPath}}
	return nil
}

func (f fakeModify) AppKnownCertsAdd(org string, certPaths []string, blockPath string) error {
	*f.invoked = certCall("ModifyAppKnownCertsAdd", org, certPaths, blockPath)
	return nil
}

func (f fakeModify) AppKnownCertsRemove(org string, certPaths []string, blockPath string) error {
	*f.invoked = certCall("ModifyAppKnownCertsRemove", org, certPaths, blockPath)
	return nil
}

// certCall records a known-certs invocation as {org, certPaths…, blockPath}.
func certCall(handler, org string, certPaths []string, blockPath string) call {
	args := make([]string, 0, len(certPaths)+2)
	args = append(args, org)
	args = append(args, certPaths...)
	args = append(args, blockPath)
	return call{handler: handler, args: args}
}

func (f fakeModify) PartyAdd(partyPath, blockPath string) error {
	*f.invoked = call{handler: "ModifyPartyAdd", args: []string{partyPath, blockPath}}
	return nil
}

func (f fakeModify) PartyRemove(partyID uint32, blockPath string) error {
	*f.invoked = call{handler: "ModifyPartyRemove", args: []string{strconv.FormatUint(uint64(partyID), 10), blockPath}}
	return nil
}

func (f fakeModify) PartyNode(ch cli.NodeChange) error {
	*f.invoked = call{handler: "ModifyPartyNode", args: []string{
		strconv.FormatUint(uint64(ch.Party), 10), ch.Role, strconv.FormatUint(uint64(ch.Shard), 10),
		ch.Host, strconv.FormatUint(uint64(ch.Port), 10), ch.TLSCert, ch.SignCert, ch.BlockPath,
	}}
	return nil
}

func (f fakeModify) PartyCA(ch cli.CAChange) error {
	args := make([]string, 0, len(ch.SignCerts)+len(ch.TLSCerts)+3)
	args = append(args, ch.Op, strconv.FormatUint(uint64(ch.Party), 10))
	args = append(args, ch.SignCerts...)
	args = append(args, ch.TLSCerts...)
	args = append(args, ch.BlockPath)
	*f.invoked = call{handler: "ModifyPartyCA", args: args}
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
	nextBlock := writeTempFile(t, "next.pb")
	orgYAML := writeTempFile(t, "org.yaml")
	partyYAML := writeTempFile(t, "party.yaml")
	cert1 := writeTempFile(t, "cert1.pem")
	cert2 := writeTempFile(t, "cert2.pem")

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
			args:        []string{"decode", currBlock, flagOutput, "current_config.json"},
			wantHandler: "Decode",
			wantArgs:    []string{currBlock, "current_config.json"},
		},
		{
			name: "compute-update",
			args: []string{
				"compute-update",
				currentJSON,
				modifiedJSON,
				flagCurrentBlock,
				currBlock,
				flagOutput,
				fileConfigUpdate,
			},
			wantHandler: "ComputeUpdate",
			wantArgs:    []string{currentJSON, modifiedJSON, currBlock, fileConfigUpdate},
		},
		{
			name: "compute-update block mode",
			args: []string{
				"compute-update", "--pb", currBlock, nextBlock, flagOutput, fileConfigUpdate,
			},
			wantHandler: "ComputeUpdatePB",
			wantArgs:    []string{currBlock, nextBlock, fileConfigUpdate},
		},
		{
			name:        "modify app add",
			args:        []string{cmdModify, subApp, subAdd, flagOrg, orgYAML, flagBlock, nextBlock},
			wantHandler: "ModifyAppAdd",
			wantArgs:    []string{orgYAML, nextBlock},
		},
		{
			name:        "modify app remove",
			args:        []string{cmdModify, subApp, subRemove, "peer2", flagBlock, nextBlock},
			wantHandler: "ModifyAppRemove",
			wantArgs:    []string{"peer2", nextBlock},
		},
		{
			name: "modify app known-certs add",
			args: []string{
				cmdModify, subApp, "known-certs", subAdd, flagOrg, orgPeer1,
				flagCert, cert1, flagCert, cert2, flagBlock, nextBlock,
			},
			wantHandler: "ModifyAppKnownCertsAdd",
			wantArgs:    []string{orgPeer1, cert1, cert2, nextBlock},
		},
		{
			name: "modify app known-certs remove",
			args: []string{
				cmdModify, subApp, "known-certs", subRemove, flagOrg, orgPeer1, flagCert, cert1, flagBlock, nextBlock,
			},
			wantHandler: "ModifyAppKnownCertsRemove",
			wantArgs:    []string{orgPeer1, cert1, nextBlock},
		},
		{
			name:        "modify party add",
			args:        []string{cmdModify, subParty, subAdd, flagParty, partyYAML, flagBlock, nextBlock},
			wantHandler: "ModifyPartyAdd",
			wantArgs:    []string{partyYAML, nextBlock},
		},
		{
			name:        "modify party remove",
			args:        []string{cmdModify, subParty, subRemove, "5", flagBlock, nextBlock},
			wantHandler: "ModifyPartyRemove",
			wantArgs:    []string{"5", nextBlock},
		},
		{
			name: "modify party node",
			args: []string{
				cmdModify, subParty, "node", flagParty, "1", "--role", "batcher", "--shard", "1",
				"--port", "9014", "--tls-cert", cert1, flagBlock, nextBlock,
			},
			wantHandler: "ModifyPartyNode",
			// party, role, shard, host, port, tls-cert, sign-cert, block
			wantArgs: []string{"1", "batcher", "1", "", "9014", cert1, "", nextBlock},
		},
		{
			name: "modify party ca add",
			args: []string{
				cmdModify, subParty, "ca", subAdd, flagParty, "1",
				"--sign-cert", cert1, "--tls-cert", cert2, flagBlock, nextBlock,
			},
			wantHandler: "ModifyPartyCA",
			// op, party, sign-certs…, tls-certs…, block
			wantArgs: []string{"add", "1", cert1, cert2, nextBlock},
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
			name: "tx send",
			args: []string{
				"tx", "send", endorsed, flagConfig, admin, flagCurrentBlock, currBlock, flagOutput, fileConfigTx,
			},
			wantHandler: "TxSend",
			wantArgs:    []string{endorsed, admin, currBlock, fileConfigTx},
		},
		{
			name: "follow",
			args: []string{
				cmdFollow, flagConfig, admin, flagCurrentBlock, currBlock,
				"--timeout", "30s", flagOutput, fileNextConfig,
			},
			wantHandler: "Follow",
			wantArgs:    []string{admin, currBlock, fileNextConfig},
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
			name: "bad duration for follow",
			args: []string{
				cmdFollow, flagConfig, admin, flagCurrentBlock, currBlock,
				"--timeout", "notaduration", flagOutput, fileNextConfig,
			},
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

// TestRunMissingHandler verifies that invoking a command whose
// handler was left nil returns a clear error.
func TestRunMissingHandler(t *testing.T) {
	t.Parallel()

	admin := writeTempFile(t, "admin.yaml")
	currBlock := writeTempFile(t, "current_block.pb")

	// Everything wired except the ledger handler, then a ledger command is run.
	var invoked call
	h := newHandlers(&invoked)
	h.Ledger = nil
	c := cli.New(h, "test")

	err := c.Run([]string{cmdLedger, flagConfig, admin, flagCurrentBlock, currBlock, subHeight})
	require.ErrorContains(t, err, "missing command handlers: ledger")
	require.Empty(t, invoked.handler, "handler should not have been invoked")
}

// TestRunTypedNilHandler verifies that a typed-nil interface value (a nil
// concrete pointer stored in a handler field) is treated as missing. A plain
// != nil check would let it pass and then panic when the command dispatched.
func TestRunTypedNilHandler(t *testing.T) {
	t.Parallel()

	admin := writeTempFile(t, "admin.yaml")
	currBlock := writeTempFile(t, "current_block.pb")

	// Ledger holds a non-nil interface wrapping a nil *fakeLedger
	var invoked call
	h := newHandlers(&invoked)
	h.Ledger = (*fakeLedger)(nil)
	c := cli.New(h, "test")

	err := c.Run([]string{cmdLedger, flagConfig, admin, flagCurrentBlock, currBlock, subHeight})
	require.ErrorContains(t, err, "missing command handlers: ledger")
	require.Empty(t, invoked.handler, "handler should not have been invoked")
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
