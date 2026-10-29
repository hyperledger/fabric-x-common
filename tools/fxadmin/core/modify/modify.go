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
// The command surface is wired and validated by the CLI. A handler whose change
// logic is not implemented yet returns ErrNotImplemented.
package modify

import (
	"fmt"

	"github.com/cockroachdb/errors"
	"github.com/hyperledger/fabric-lib-go/bccsp/factory"
	"github.com/hyperledger/fabric-lib-go/common/flogging"

	"github.com/hyperledger/fabric-x-common/common/channelconfig"
	"github.com/hyperledger/fabric-x-common/tools/fxadmin/core/helpers"
	"github.com/hyperledger/fabric-x-common/tools/fxadmin/core/modify/change"
)

var logger = flogging.MustGetLogger("fxadmin.modify")

// ErrNotImplemented is returned by every modify command whose change logic is
// not implemented yet.
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

// PartyCA implements `fxadmin modify party ca add|remove|set`.
func (*Handler) PartyCA(ch change.CA) error {
	logger.Debugf("modify party ca: %+v (not implemented)", ch)
	return ErrNotImplemented
}

// readAndDecodeConfigBlock reads the config block file at path and decodes it
// down to its config.
func readAndDecodeConfigBlock(path string) (*helpers.ConfigBlock, error) {
	block, err := helpers.ReadConfigBlock(path)
	if err != nil {
		return nil, err
	}
	configBlock, err := helpers.DecodeConfigBlock(block)
	if err != nil {
		return nil, fmt.Errorf("failed to decode config block %q: %w", path, err)
	}
	return configBlock, nil
}

// validateEncodeAndWriteConfigBlock checks that the block's edited config still
// builds a channel config bundle, encodes the config back into the block, and
// writes the block to the file at path, in place. An invalid edit is rejected
// here, before the file is written, rather than after the update has collected
// its endorsements.
func validateEncodeAndWriteConfigBlock(block *helpers.ConfigBlock, path string) error {
	channelID := block.ChannelHeader.GetChannelId()
	if _, err := channelconfig.NewBundle(channelID, block.Config(), factory.GetDefault()); err != nil {
		return fmt.Errorf("the edited config is not a valid channel config: %w", err)
	}
	if err := block.Encode(); err != nil {
		return err
	}
	return helpers.WriteBlock(block.Block, path)
}
