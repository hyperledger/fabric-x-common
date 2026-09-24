/*
Copyright IBM Corp. All Rights Reserved.

SPDX-License-Identifier: Apache-2.0
*/

// Package modify implements the `fxadmin modify` commands, which apply a
// structured configuration change directly to a config block file (automating
// the manual decode/hand-edit step). Each command reads the block, applies the
// change to the embedded common.Config, and writes the block back so further
// modify commands can accumulate onto it; `compute-update --pb` then computes
// the ConfigUpdate from the original and edited blocks.
//
// The command surface is wired and validated by the CLI; the change logic is
// not implemented yet, so every handler currently returns ErrNotImplemented.
package modify

import (
	"github.com/cockroachdb/errors"
	"github.com/hyperledger/fabric-lib-go/common/flogging"

	"github.com/hyperledger/fabric-x-common/tools/fxadmin/core/cli"
)

var logger = flogging.MustGetLogger("fxadmin.modify")

// ErrNotImplemented is returned by every modify command until the change logic
// is implemented.
var ErrNotImplemented = errors.New("not implemented")

// Handler executes the modify commands.
type Handler struct{}

// New returns a modify command handler.
func New() *Handler {
	return &Handler{}
}

// AppAdd implements `fxadmin modify app add`.
func (*Handler) AppAdd(orgPath, blockPath string) error {
	logger.Debugf("modify app add: org=%s block=%s (not implemented)", orgPath, blockPath)
	return ErrNotImplemented
}

// AppRemove implements `fxadmin modify app remove`.
func (*Handler) AppRemove(org, blockPath string) error {
	logger.Debugf("modify app remove: org=%s block=%s (not implemented)", org, blockPath)
	return ErrNotImplemented
}

// AppKnownCertsAdd implements `fxadmin modify app known-certs add`.
func (*Handler) AppKnownCertsAdd(org string, certPaths []string, blockPath string) error {
	logger.Debugf("modify app known-certs add: org=%s certs=%v block=%s (not implemented)", org, certPaths, blockPath)
	return ErrNotImplemented
}

// AppKnownCertsRemove implements `fxadmin modify app known-certs remove`.
func (*Handler) AppKnownCertsRemove(org string, certPaths []string, blockPath string) error {
	logger.Debugf("modify app known-certs remove: org=%s certs=%v block=%s (not impl.)", org, certPaths, blockPath)
	return ErrNotImplemented
}

// PartyAdd implements `fxadmin modify party add`.
func (*Handler) PartyAdd(partyPath, blockPath string) error {
	logger.Debugf("modify party add: party=%s block=%s (not implemented)", partyPath, blockPath)
	return ErrNotImplemented
}

// PartyRemove implements `fxadmin modify party remove`.
func (*Handler) PartyRemove(partyID uint32, blockPath string) error {
	logger.Debugf("modify party remove: partyID=%d block=%s (not implemented)", partyID, blockPath)
	return ErrNotImplemented
}

// PartyNode implements `fxadmin modify party node`.
func (*Handler) PartyNode(change cli.NodeChange) error {
	logger.Debugf("modify party node: %+v (not implemented)", change)
	return ErrNotImplemented
}

// PartyCA implements `fxadmin modify party ca add|remove|set`.
func (*Handler) PartyCA(change cli.CAChange) error {
	logger.Debugf("modify party ca: %+v (not implemented)", change)
	return ErrNotImplemented
}
