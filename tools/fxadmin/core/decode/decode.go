/*
Copyright IBM Corp. All Rights Reserved.

SPDX-License-Identifier: Apache-2.0
*/

// Package decode implements the `fxadmin decode` command, which converts a
// binary config block into human-readable JSON.
package decode

import (
	"fmt"

	"github.com/cockroachdb/errors"
	"github.com/hyperledger/fabric-lib-go/common/flogging"
)

var logger = flogging.MustGetLogger("fxadmin.decode")

var errNotImplemented = errors.New("not implemented")

// Handler executes the decode command.
type Handler struct{}

// New returns a decode command handler.
func New() *Handler {
	return &Handler{}
}

// Run implements `fxadmin decode`.
func (*Handler) Run(blockPath, outputPath string) error {
	logger.Debugf("decode: block=%s output=%s", blockPath, outputPath)
	return fmt.Errorf("decode: %w", errNotImplemented)
}
