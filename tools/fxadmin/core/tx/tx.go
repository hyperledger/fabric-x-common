/*
Copyright IBM Corp. All Rights Reserved.

SPDX-License-Identifier: Apache-2.0
*/

// Package tx implements the `fxadmin tx` command family: endorsing, merging,
// preparing, submitting, and sending configuration update transactions.
package tx

import (
	"fmt"

	"github.com/cockroachdb/errors"
	"github.com/hyperledger/fabric-lib-go/common/flogging"
)

var logger = flogging.MustGetLogger("fxadmin.tx")

var errNotImplemented = errors.New("not implemented")

// Handler executes the tx subcommands. Its dependencies (the signer, the
// router Broadcast client) will be added as constructor arguments and struct
// fields when the commands are implemented.
type Handler struct{}

// New returns a tx command handler.
func New() *Handler {
	return &Handler{}
}

// Endorse implements `fxadmin tx endorse`.
func (*Handler) Endorse(inputPath, configPath, outputPath string) error {
	logger.Debugf("tx endorse: input=%s config=%s output=%s", inputPath, configPath, outputPath)
	return fmt.Errorf("tx endorse: %w", errNotImplemented)
}

// Merge implements `fxadmin tx merge`.
func (*Handler) Merge(inputPaths []string, outputPath string) error {
	logger.Debugf("tx merge: inputs=%v output=%s", inputPaths, outputPath)
	return fmt.Errorf("tx merge: %w", errNotImplemented)
}

// Prepare implements `fxadmin tx prepare`.
func (*Handler) Prepare(inputPath, configPath, outputPath string) error {
	logger.Debugf("tx prepare: input=%s config=%s output=%s", inputPath, configPath, outputPath)
	return fmt.Errorf("tx prepare: %w", errNotImplemented)
}

// Submit implements `fxadmin tx submit`.
func (*Handler) Submit(inputPath, configPath, currentBlockPath string) error {
	logger.Debugf("tx submit: input=%s config=%s current-block=%s", inputPath, configPath, currentBlockPath)
	return fmt.Errorf("tx submit: %w", errNotImplemented)
}

// Send implements `fxadmin tx send`.
func (*Handler) Send(inputPath, configPath, currentBlockPath string) error {
	logger.Debugf("tx send: input=%s config=%s current-block=%s", inputPath, configPath, currentBlockPath)
	return fmt.Errorf("tx send: %w", errNotImplemented)
}
