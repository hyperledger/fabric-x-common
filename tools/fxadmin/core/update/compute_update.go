/*
Copyright IBM Corp. All Rights Reserved.

SPDX-License-Identifier: Apache-2.0
*/

// Package update implements the `fxadmin compute-update` command, which
// computes the ConfigUpdate delta between the original and modified
// configuration JSON.
package update

import (
	"fmt"

	"github.com/cockroachdb/errors"
	"github.com/hyperledger/fabric-lib-go/common/flogging"
)

var logger = flogging.MustGetLogger("fxadmin.compute-update")

var errNotImplemented = errors.New("not implemented")

// Handler executes the compute-update command.
type Handler struct{}

// New returns a compute-update command handler.
func New() *Handler {
	return &Handler{}
}

// Run implements `fxadmin compute-update`.
func (*Handler) Run(currentPath, modifiedPath, outputPath string) error {
	logger.Debugf("compute-update: current=%s modified=%s output=%s", currentPath, modifiedPath, outputPath)
	return fmt.Errorf("compute-update: %w", errNotImplemented)
}
