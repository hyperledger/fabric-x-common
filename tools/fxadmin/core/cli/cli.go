/*
Copyright IBM Corp. All Rights Reserved.

SPDX-License-Identifier: Apache-2.0
*/

// Package cli builds the fxadmin command tree. fxadmin administers the
// configuration of Fabric-X: it pulls the current configuration from the
// running network, decodes it, computes and endorses a configuration update,
// wraps it into a submittable transaction, submits it, and follows the
// assembler ledger until the change is committed. fxadmin operates on the
// single channel "arma".
package cli

import (
	"github.com/alecthomas/kingpin/v2"
	"github.com/cockroachdb/errors"
)

// Channel is the single, fixed channel fxadmin operates on.
const Channel = "arma"

// Flag names shared across commands.
const (
	flagConfig       = "config"
	flagCurrentBlock = "current-block"
	flagOutput       = "output"
	flagBlock        = "block"
	flagTimeout      = "timeout"
)

// CLI is the fxadmin command-line application. It owns the kingpin command
// tree and a dispatch table mapping each command to a closure
// that invokes the corresponding Handlers method with the bound values.
type CLI struct {
	app      *kingpin.Application
	handlers Handlers
	dispatch map[string]func() error
}

// New builds the fxadmin CLI around the given handlers. The version string is
// surfaced through the --version flag. No command is executed until Run is called.
func New(handlers Handlers, version string) *CLI {
	app := kingpin.New("fxadmin", "Fabric-X reconfiguration admin CLI.")
	app.Version(version)

	c := &CLI{
		app:      app,
		handlers: handlers,
		dispatch: make(map[string]func() error),
	}

	c.addLedgerCommands()
	c.addDecodeCommand()
	c.addComputeUpdateCommand()
	c.addTxCommands()
	c.addFollowCommand()

	return c
}

// Parse resolves args to the selected command and binds flag/argument values.
func (c *CLI) Parse(args []string) (string, error) {
	cmd, err := c.app.Parse(args)
	if err != nil {
		return "", errors.Wrap(err, "failed to parse arguments")
	}
	return cmd, nil
}

// Run parses args and executes the selected command via its handler.
func (c *CLI) Run(args []string) error {
	cmd, err := c.Parse(args)
	if err != nil {
		return err
	}
	run, ok := c.dispatch[cmd]
	if !ok {
		return errors.Newf("no handler registered for command %q", cmd)
	}
	return run()
}

// register records the closure that executes cmd after a successful parse.
func (c *CLI) register(cmd *kingpin.CmdClause, run func() error) {
	c.dispatch[cmd.FullCommand()] = run
}

// addLedgerCommands wires `fxadmin ledger` and its height/block/config
// subcommands. --config and --current-block are common to all of them.
func (c *CLI) addLedgerCommands() {
	ledger := c.app.Command("ledger", "Query the assembler ledger.")
	config := ledger.Flag(flagConfig, "Path to the admin configuration YAML file (identity, TLS).").Required().ExistingFile()
	currentBlock := ledger.Flag(flagCurrentBlock, "Path to the current config block containing the assembler endpoints.").Required().ExistingFile()

	height := ledger.Command("height", "Print the height of the ledger.")
	c.register(height, func() error {
		return c.handlers.Ledger.Height(*config, *currentBlock)
	})

	block := ledger.Command("block", "Fetch a block (\"latest\" or a block number) and write it to a file.")
	blockRef := block.Arg("block", "Block to fetch: \"latest\" or a block number.").Required().String()
	blockOut := block.Flag(flagOutput, "Path to the output block protobuf file.").Required().String()
	c.register(block, func() error {
		return c.handlers.Ledger.Block(*config, *currentBlock, *blockRef, *blockOut)
	})

	cfg := ledger.Command("config", "Fetch the last config block (\"latest\") and write it to a file.")
	cfgRef := cfg.Arg("config", "Config block to fetch: \"latest\".").Required().String()
	cfgOut := cfg.Flag(flagOutput, "Path to the output config block protobuf file.").Required().String()
	c.register(cfg, func() error {
		return c.handlers.Ledger.Config(*config, *currentBlock, *cfgRef, *cfgOut)
	})
}

// addDecodeCommand wires `fxadmin decode`, which converts a binary config
// block into human-readable JSON.
func (c *CLI) addDecodeCommand() {
	decode := c.app.Command("decode", "Decode a binary config block into JSON.")
	block := decode.Flag(flagBlock, "Path to the protobuf block file to decode.").Required().ExistingFile()
	output := decode.Flag(flagOutput, "Path to the output JSON file.").Required().String()
	c.register(decode, func() error {
		return c.handlers.Decode.Run(*block, *output)
	})
}

// addComputeUpdateCommand wires `fxadmin compute-update`, which computes the
// ConfigUpdate delta between the original and modified configuration JSON.
func (c *CLI) addComputeUpdateCommand() {
	cmd := c.app.Command("compute-update", "Compute the ConfigUpdate delta between two config JSON files.")
	current := cmd.Arg("current.json", "Original configuration JSON.").Required().ExistingFile()
	modified := cmd.Arg("modified.json", "Modified configuration JSON.").Required().ExistingFile()
	output := cmd.Flag(flagOutput, "Path to the output ConfigUpdate protobuf file.").Required().String()
	c.register(cmd, func() error {
		return c.handlers.Update.Run(*current, *modified, *output)
	})
}

// addTxCommands wires `fxadmin tx` and its endorse/merge/prepare/submit/send
// subcommands, which build and broadcast the configuration update transaction.
func (c *CLI) addTxCommands() {
	tx := c.app.Command("tx", "Endorse, merge, prepare, and submit configuration update transactions.")
	c.addTxEndorseCommand(tx)
	c.addTxMergeCommand(tx)
	c.addTxPrepareCommand(tx)
	c.addTxSubmitCommand(tx)
	c.addTxSendCommand(tx)
}

// addTxEndorseCommand wires `fxadmin tx endorse`.
func (c *CLI) addTxEndorseCommand(tx *kingpin.CmdClause) {
	endorse := tx.Command("endorse", "Sign a ConfigUpdate with the admin identity.")
	input := endorse.Arg("config_update.pb", "Path to the ConfigUpdate protobuf file to endorse.").Required().ExistingFile()
	config := endorse.Flag(flagConfig, "Path to the admin configuration YAML file (signing identity).").Required().ExistingFile()
	output := endorse.Flag(flagOutput, "Path to the generated endorsement protobuf file.").Required().String()
	c.register(endorse, func() error {
		return c.handlers.Tx.Endorse(*input, *config, *output)
	})
}

// addTxMergeCommand wires `fxadmin tx merge`.
func (c *CLI) addTxMergeCommand(tx *kingpin.CmdClause) {
	merge := tx.Command("merge", "Merge endorsements into a single endorsed ConfigUpdateEnvelope.")
	inputs := merge.Arg("endorsement.pb", "Paths to one or more endorsement protobuf files to merge.").Required().Strings()
	output := merge.Flag(flagOutput, "Path to the merged configuration update envelope.").Required().String()
	c.register(merge, func() error {
		return c.handlers.Tx.Merge(*inputs, *output)
	})
}

// addTxPrepareCommand wires `fxadmin tx prepare`.
func (c *CLI) addTxPrepareCommand(tx *kingpin.CmdClause) {
	prepare := tx.Command("prepare", "Wrap an endorsed config update into a signed configuration transaction.")
	input := prepare.Arg("endorsed_config_update.pb", "Path to the endorsed config update protobuf file.").Required().ExistingFile()
	config := prepare.Flag(flagConfig, "Path to the admin configuration YAML file (submitting client identity).").Required().ExistingFile()
	output := prepare.Flag(flagOutput, "Path to the generated configuration transaction protobuf file.").Required().String()
	c.register(prepare, func() error {
		return c.handlers.Tx.Prepare(*input, *config, *output)
	})
}

// addTxSubmitCommand wires `fxadmin tx submit`.
func (c *CLI) addTxSubmitCommand(tx *kingpin.CmdClause) {
	submit := tx.Command("submit", "Submit a prepared configuration transaction to all routers.")
	input := submit.Arg("config_tx.pb", "Path to the prepared configuration transaction protobuf file.").Required().ExistingFile()
	config := submit.Flag(flagConfig, "Path to the admin configuration YAML file.").Required().ExistingFile()
	currentBlock := submit.Flag(flagCurrentBlock, "Path to the current config block containing the router endpoints.").Required().ExistingFile()
	c.register(submit, func() error {
		return c.handlers.Tx.Submit(*input, *config, *currentBlock)
	})
}

// addTxSendCommand wires `fxadmin tx send`, equivalent to prepare + submit.
func (c *CLI) addTxSendCommand(tx *kingpin.CmdClause) {
	send := tx.Command("send", "Prepare and submit an endorsed config update in one step.")
	input := send.Arg("endorsed_config_update.pb", "Path to the endorsed config update protobuf file.").Required().ExistingFile()
	config := send.Flag(flagConfig, "Path to the admin configuration YAML file.").Required().ExistingFile()
	currentBlock := send.Flag(flagCurrentBlock, "Path to the current config block containing the router endpoints.").Required().ExistingFile()
	c.register(send, func() error {
		return c.handlers.Tx.Send(*input, *config, *currentBlock)
	})
}

// addFollowCommand wires `fxadmin follow`, which monitors the assembler ledgers
// until the configured timeout expires.
func (c *CLI) addFollowCommand() {
	follow := c.app.Command("follow", "Follow the assembler ledgers until the timeout expires.")
	config := follow.Flag(flagConfig, "Path to the admin configuration YAML file.").Required().ExistingFile()
	currentBlock := follow.Flag(flagCurrentBlock, "Path to the current config block containing the assembler endpoints.").Required().ExistingFile()
	timeout := follow.Flag(flagTimeout, "How long to pull blocks from the assemblers before reporting results.").Required().Duration()
	c.register(follow, func() error {
		return c.handlers.Follow.Run(*config, *currentBlock, *timeout)
	})
}
