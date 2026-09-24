/*
Copyright IBM Corp. All Rights Reserved.

SPDX-License-Identifier: Apache-2.0
*/

// Package cli builds the fxadmin command tree. fxadmin administers the
// configuration of Fabric-X: it pulls the current configuration from the
// running network, decodes it, computes and endorses a configuration update,
// wraps it into a submittable transaction, submits it, and follows the
// assembler ledger until the change is committed.
package cli

import (
	"github.com/alecthomas/kingpin/v2"
	"github.com/cockroachdb/errors"
)

// Flag names shared across commands.
const (
	flagConfig       = "config"
	flagCurrentBlock = "current-block"
	flagOutput       = "output"
	flagTimeout      = "timeout"

	// Flags used by compute-update block mode and the modify commands.
	flagPB       = "pb"
	flagBlock    = "block"
	flagOrg      = "org"
	flagParty    = "party"
	flagCert     = "cert"
	flagRole     = "role"
	flagShard    = "shard"
	flagHost     = "host"
	flagPort     = "port"
	flagTLSCert  = "tls-cert"
	flagSignCert = "sign-cert"
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
	c.addModifyCommands()
	c.addComputeUpdateCommand()
	c.addTxCommands()
	c.addFollowCommand()

	return c
}

// parse resolves args to the selected command and binds flag/argument values.
func (c *CLI) parse(args []string) (string, error) {
	cmd, err := c.app.Parse(args)
	if err != nil {
		return "", errors.Wrap(err, "failed to parse arguments")
	}
	return cmd, nil
}

// Run parses args and executes the selected command via its handler.
func (c *CLI) Run(args []string) error {
	cmd, err := c.parse(args)
	if err != nil {
		return err
	}
	if err := c.handlers.validate(); err != nil {
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
	config := ledger.Flag(flagConfig, "Path to the admin configuration YAML file (identity, TLS).").
		Required().ExistingFile()
	currentBlock := ledger.
		Flag(
			flagCurrentBlock,
			"Path to the current config block containing the assembler endpoints.",
		).
		Required().
		ExistingFile()

	height := ledger.Command("height", "Print the height of the ledger.")
	c.register(height, func() error {
		return c.handlers.Ledger.Height(*config, *currentBlock)
	})

	block := ledger.Command("block", "Fetch a block (\"latest\" or a block number) and write it to a file.")
	blockRef := block.Arg("reference", "Block to fetch: \"latest\" or a block number.").Required().String()
	blockOut := block.Flag(flagOutput, "Path to the output block protobuf file.").Required().String()
	c.register(block, func() error {
		return c.handlers.Ledger.Block(*config, *currentBlock, *blockRef, *blockOut)
	})

	cfg := ledger.Command("config", "Fetch the last config block (\"latest\") and write it to a file.")
	cfgRef := cfg.Arg("reference", "Config block to fetch: \"latest\".").Required().String()
	cfgOut := cfg.Flag(flagOutput, "Path to the output config block protobuf file.").Required().String()
	c.register(cfg, func() error {
		return c.handlers.Ledger.Config(*config, *currentBlock, *cfgRef, *cfgOut)
	})
}

// addDecodeCommand wires `fxadmin decode`, which extracts the common.Config
// embedded in a binary config block and writes it as JSON for editing.
func (c *CLI) addDecodeCommand() {
	decode := c.app.Command("decode", "Extract the common.Config from a config block into editable JSON.")
	block := decode.Arg("config_block.pb", "Path to the protobuf config block file to decode.").
		Required().ExistingFile()
	output := decode.Flag(flagOutput, "Path to the output common.Config JSON file.").Required().String()
	c.register(decode, func() error {
		return c.handlers.Decode.Run(*block, *output)
	})
}

// addComputeUpdateCommand wires `fxadmin compute-update`, which computes the
// ConfigUpdate delta between an original and a modified configuration. In the
// default JSON mode the two positional arguments are decoded-config JSON files
// and the channel ID is read from --current-block. In block mode (--pb) they are
// two config block files (original, next).
func (c *CLI) addComputeUpdateCommand() {
	cmd := c.app.Command("compute-update",
		"Compute the ConfigUpdate delta between two configs (JSON files, or config blocks).")
	current := cmd.Arg("current", "Original config: JSON (default) or config block (--pb).").
		Required().ExistingFile()
	modified := cmd.Arg("modified", "Modified config: JSON (default) or the edited \"next\" config block (--pb).").
		Required().ExistingFile()
	pb := cmd.Flag(flagPB, "Block mode: treat the two arguments as config blocks (original, next) instead of JSON.").
		Bool()
	currentBlock := cmd.
		Flag(flagCurrentBlock, "JSON mode only: Path to the current config block whose channel ID the update targets.").
		ExistingFile()
	output := cmd.Flag(flagOutput, "Path to the output ConfigUpdate protobuf file.").Required().String()
	c.register(cmd, func() error {
		if *pb {
			return c.handlers.Update.RunFromBlocks(*current, *modified, *output)
		}
		if *currentBlock == "" {
			return errors.Newf("--%s is required in JSON mode (or pass --%s for block mode)", flagCurrentBlock, flagPB)
		}
		return c.handlers.Update.Run(*current, *modified, *currentBlock, *output)
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
	input := endorse.Arg("config_update.pb", "Path to the ConfigUpdate protobuf file to endorse.").
		Required().ExistingFile()
	config := endorse.Flag(flagConfig, "Path to the admin configuration YAML file (signing identity).").
		Required().ExistingFile()
	output := endorse.Flag(flagOutput, "Path to the generated endorsement protobuf file.").Required().String()
	c.register(endorse, func() error {
		return c.handlers.Tx.Endorse(*input, *config, *output)
	})
}

// addTxMergeCommand wires `fxadmin tx merge`.
func (c *CLI) addTxMergeCommand(tx *kingpin.CmdClause) {
	merge := tx.Command("merge", "Merge endorsements into a single endorsed ConfigUpdateEnvelope.")
	inputs := merge.Arg("endorsement.pb", "Paths to one or more endorsement protobuf files to merge.").
		Required().Strings()
	output := merge.Flag(flagOutput, "Path to the merged configuration update envelope.").Required().String()
	c.register(merge, func() error {
		return c.handlers.Tx.Merge(*inputs, *output)
	})
}

// addTxPrepareCommand wires `fxadmin tx prepare`.
func (c *CLI) addTxPrepareCommand(tx *kingpin.CmdClause) {
	prepare := tx.Command("prepare", "Wrap an endorsed config update into a signed configuration transaction.")
	input := prepare.Arg("endorsed_config_update.pb", "Path to the endorsed config update protobuf file.").
		Required().ExistingFile()
	config := prepare.Flag(flagConfig, "Path to the admin configuration YAML file (submitting client identity).").
		Required().ExistingFile()
	output := prepare.Flag(flagOutput, "Path to the generated configuration transaction protobuf file.").
		Required().String()
	c.register(prepare, func() error {
		return c.handlers.Tx.Prepare(*input, *config, *output)
	})
}

// addTxSubmitCommand wires `fxadmin tx submit`.
func (c *CLI) addTxSubmitCommand(tx *kingpin.CmdClause) {
	submit := tx.Command("submit", "Submit a prepared configuration transaction to all routers.")
	input := submit.Arg("config_tx.pb", "Path to the prepared configuration transaction protobuf file.").
		Required().ExistingFile()
	config := submit.Flag(flagConfig, "Path to the admin configuration YAML file.").Required().ExistingFile()
	currentBlock := submit.Flag(flagCurrentBlock, "Path to the current config block containing the router endpoints.").
		Required().ExistingFile()
	c.register(submit, func() error {
		return c.handlers.Tx.Submit(*input, *config, *currentBlock)
	})
}

// addTxSendCommand wires `fxadmin tx send`, equivalent to prepare + submit.
func (c *CLI) addTxSendCommand(tx *kingpin.CmdClause) {
	send := tx.Command("send", "Prepare and submit an endorsed config update in one step.")
	input := send.Arg("endorsed_config_update.pb", "Path to the endorsed config update protobuf file.").
		Required().ExistingFile()
	config := send.Flag(flagConfig, "Path to the admin configuration YAML file.").Required().ExistingFile()
	currentBlock := send.Flag(flagCurrentBlock, "Path to the current config block containing the router endpoints.").
		Required().ExistingFile()
	output := send.
		Flag(flagOutput, "Path to write the prepared configuration transaction to, for record keeping.").
		Required().
		String()
	c.register(send, func() error {
		return c.handlers.Tx.Send(*input, *config, *currentBlock, *output)
	})
}

// addFollowCommand wires `fxadmin follow`, which waits for the next config block
// to commit across the assemblers, or until the timeout expires.
func (c *CLI) addFollowCommand() {
	follow := c.app.Command("follow",
		"Wait for the next config block to commit across the assemblers, or until the timeout expires.")
	config := follow.Flag(flagConfig, "Path to the admin configuration YAML file.").Required().ExistingFile()
	currentBlock := follow.
		Flag(
			flagCurrentBlock,
			"Path to the current config block containing the assembler endpoints.",
		).
		Required().
		ExistingFile()
	timeout := follow.Flag(flagTimeout, "How long to pull blocks from the assemblers before reporting results.").
		Required().Duration()
	output := follow.
		Flag(flagOutput, "Path to write the agreed next config block to, for use as the next --current-block.").
		Required().String()
	c.register(follow, func() error {
		return c.handlers.Follow.Run(*config, *currentBlock, *output, *timeout)
	})
}

// addModifyCommands wires `fxadmin modify` and its app/party subcommands, which
// apply a structured change directly to a config block file,
// automating the manual decode/hand-edit step.
func (c *CLI) addModifyCommands() {
	modify := c.app.Command("modify",
		"Apply a structured configuration change to a config block file.")
	c.addModifyAppCommands(modify)
	c.addModifyPartyCommands(modify)
}

// blockFlag registers the --block flag common to every modify command: the
// config block file the command edits in place.
func blockFlag(cmd *kingpin.CmdClause) *string {
	return cmd.Flag(flagBlock, "Config block file to edit in place (the \"next\" block).").
		Required().ExistingFile()
}

// addModifyAppCommands wires `fxadmin modify app` (add / remove / known-certs).
func (c *CLI) addModifyAppCommands(modify *kingpin.CmdClause) {
	app := modify.Command("app", "Add or remove application organizations and their known-certs.")

	add := app.Command("add", "Add an application organization.")
	addOrg := add.Flag(flagOrg, "Path to the organization definition YAML.").Required().ExistingFile()
	addBlockFlag := blockFlag(add)
	c.register(add, func() error {
		return c.handlers.Modify.AppAdd(*addOrg, *addBlockFlag)
	})

	remove := app.Command("remove", "Remove an application organization.")
	removeOrg := remove.Arg("org", "Application organization name to remove.").Required().String()
	removeBlockFlag := blockFlag(remove)
	c.register(remove, func() error {
		return c.handlers.Modify.AppRemove(*removeOrg, *removeBlockFlag)
	})

	c.addModifyKnownCertsCommands(app)
}

// addModifyKnownCertsCommands wires `fxadmin modify app known-certs add|remove`.
func (c *CLI) addModifyKnownCertsCommands(app *kingpin.CmdClause) {
	known := app.Command("known-certs", "Add or remove entries in an application org's MSP known-certs list.")

	add := known.Command("add", "Add known-certs to an application organization.")
	addOrg := add.Flag(flagOrg, "Application organization whose known-certs change.").Required().String()
	addCerts := add.Flag(flagCert, "PEM path to add (repeatable).").Required().ExistingFiles()
	addBlockFlag := blockFlag(add)
	c.register(add, func() error {
		return c.handlers.Modify.AppKnownCertsAdd(*addOrg, *addCerts, *addBlockFlag)
	})

	remove := known.Command("remove", "Remove known-certs from an application organization.")
	removeOrg := remove.Flag(flagOrg, "Application organization whose known-certs change.").Required().String()
	removeCerts := remove.Flag(flagCert, "PEM path to remove (repeatable).").Required().ExistingFiles()
	removeBlockFlag := blockFlag(remove)
	c.register(remove, func() error {
		return c.handlers.Modify.AppKnownCertsRemove(*removeOrg, *removeCerts, *removeBlockFlag)
	})
}

// addModifyPartyCommands wires `fxadmin modify party` (add / remove / node / ca).
func (c *CLI) addModifyPartyCommands(modify *kingpin.CmdClause) {
	party := modify.Command("party", "Add or remove ARMA parties, or change a party's nodes and CA lists.")

	add := party.Command("add", "Add a new ARMA party and its orderer organization, if it does not already exist.")
	partyDef := add.Flag(flagParty, "Path to the party definition YAML.").Required().ExistingFile()
	addBlockFlag := blockFlag(add)
	c.register(add, func() error {
		return c.handlers.Modify.PartyAdd(*partyDef, *addBlockFlag)
	})

	remove := party.Command("remove",
		"Remove an ARMA party (and its orderer org, unless another party is still associated with it).")
	partyID := remove.Arg("party-id", "Numeric PartyID to remove.").Required().Uint32()
	removeBlockFlag := blockFlag(remove)
	c.register(remove, func() error {
		return c.handlers.Modify.PartyRemove(*partyID, *removeBlockFlag)
	})

	c.addModifyPartyNodeCommand(party)
	c.addModifyPartyCACommands(party)
}

// addModifyPartyNodeCommand wires `fxadmin modify party node`, which changes any
// subset of one node's endpoint and certificate fields.
func (c *CLI) addModifyPartyNodeCommand(party *kingpin.CmdClause) {
	node := party.Command("node", "Change one party node's endpoint and/or certificates (any subset of fields).")
	partyID := node.Flag(flagParty, "Party ID.").Required().Uint32()
	role := node.Flag(flagRole, "Node role.").Required().Enum("router", "batcher", "consenter", "assembler")
	shard := node.Flag(flagShard, "Batcher shard ID (required for --role batcher).").Uint32()
	host := node.Flag(flagHost, "New host (unchanged if omitted).").String()
	port := node.Flag(flagPort, "New port (unchanged if omitted).").Uint32()
	tlsCert := node.Flag(flagTLSCert, "Path to new TLS certificate (unchanged if omitted).").ExistingFile()
	signCert := node.Flag(flagSignCert, "Path to new signing certificate (unchanged if omitted).").ExistingFile()
	block := blockFlag(node)
	c.register(node, func() error {
		return c.handlers.Modify.PartyNode(NodeChange{
			Party:     *partyID,
			Role:      *role,
			Shard:     *shard,
			Host:      *host,
			Port:      *port,
			TLSCert:   *tlsCert,
			SignCert:  *signCert,
			BlockPath: *block,
		})
	})
}

// addModifyPartyCACommands wires `fxadmin modify party ca add|remove|set` over a
// party's CA (--sign-cert) and TLS-CA (--tls-cert) certificate lists.
func (c *CLI) addModifyPartyCACommands(party *kingpin.CmdClause) {
	ca := party.Command("ca", "Change a party's CA and/or TLS-CA certificate lists.")
	for _, op := range []string{"add", "remove", "set"} {
		c.registerPartyCAOp(ca, op)
	}
}

// registerPartyCAOp wires one `modify party ca` sub-verb (add, remove, or set).
func (c *CLI) registerPartyCAOp(ca *kingpin.CmdClause, op string) {
	cmd := ca.Command(op, "Apply the "+op+" operation to a party's CA / TLS-CA certificate lists.")
	partyID := cmd.Flag(flagParty, "Party whose CA list(s) change.").Required().Uint32()
	signCerts := cmd.Flag(flagSignCert, "PEM path(s) for the signing-CA list (repeatable).").ExistingFiles()
	tlsCerts := cmd.Flag(flagTLSCert, "PEM path(s) for the TLS-CA list (repeatable).").ExistingFiles()
	block := blockFlag(cmd)
	c.register(cmd, func() error {
		return c.handlers.Modify.PartyCA(CAChange{
			Op:        op,
			Party:     *partyID,
			SignCerts: *signCerts,
			TLSCerts:  *tlsCerts,
			BlockPath: *block,
		})
	})
}
