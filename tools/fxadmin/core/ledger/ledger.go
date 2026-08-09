/*
Copyright IBM Corp. All Rights Reserved.

SPDX-License-Identifier: Apache-2.0
*/

// Package ledger implements the `fxadmin ledger` command family: querying the
// assembler ledger for its height, an arbitrary block, or the last config
// block.
package ledger

import (
	"fmt"

	"github.com/cockroachdb/errors"
	"github.com/hyperledger/fabric-lib-go/common/flogging"
)

var logger = flogging.MustGetLogger("fxadmin.ledger")

var errNotImplemented = errors.New("not implemented")

// Handler executes the ledger subcommands. Its dependencies (config loading,
// the assembler Deliver client) will be added as constructor arguments and
// struct fields when the commands are implemented.
type Handler struct{}

// New returns a ledger command handler.
func New() *Handler {
	return &Handler{}
}

// Height implements `fxadmin ledger height`.
func (*Handler) Height(configPath, currentBlockPath string) error {
	logger.Debugf("ledger height: config=%s current-block=%s", configPath, currentBlockPath)
	return fmt.Errorf("ledger height: %w", errNotImplemented)
}

// Block implements `fxadmin ledger block <block>`.
func (*Handler) Block(configPath, currentBlockPath, reference, outputPath string) error {
	logger.Debugf("ledger block %s: config=%s current-block=%s output=%s",
		reference, configPath, currentBlockPath, outputPath)
	return fmt.Errorf("ledger block: %w", errNotImplemented)
}

// Config implements `fxadmin ledger config <config>`.
func (*Handler) Config(configPath, currentBlockPath, reference, outputPath string) error {
	logger.Debugf("ledger config %s: config=%s current-block=%s output=%s",
		reference, configPath, currentBlockPath, outputPath)
	return fmt.Errorf("ledger config: %w", errNotImplemented)
}
