/*
Copyright IBM Corp. All Rights Reserved.

SPDX-License-Identifier: Apache-2.0
*/

// Package follow implements the `fxadmin follow` command, which monitors the
// assembler ledgers until the configured timeout expires.
package follow

import (
	"fmt"
	"time"

	"github.com/cockroachdb/errors"
	"github.com/hyperledger/fabric-lib-go/common/flogging"
)

var logger = flogging.MustGetLogger("fxadmin.follow")

var errNotImplemented = errors.New("not implemented")

// Handler executes the follow command.
type Handler struct{}

// New returns a follow command handler.
func New() *Handler {
	return &Handler{}
}

// Run implements `fxadmin follow`.
func (*Handler) Run(configPath, currentBlockPath string, timeout time.Duration) error {
	logger.Debugf("follow: config=%s current-block=%s timeout=%s", configPath, currentBlockPath, timeout)
	return fmt.Errorf("follow: %w", errNotImplemented)
}
