/*
Copyright IBM Corp. All Rights Reserved.

SPDX-License-Identifier: Apache-2.0
*/

// Package update implements the `fxadmin compute-update` command, which
// computes the ConfigUpdate delta between the original and modified
// configuration JSON. It reproduces the logic of `configtxlator proto_encode
// --type common.Config` on each input followed by `configtxlator
// compute_update`: each JSON config is encoded to a common.Config, and the
// delta between the two is written as a marshaled common.ConfigUpdate.
// The channel ID is not part of common.Config, so it is read
// from the current config block supplied to Run.
package update

import (
	"os"

	"github.com/cockroachdb/errors"
	"github.com/hyperledger/fabric-lib-go/common/flogging"
	cb "github.com/hyperledger/fabric-protos-go-apiv2/common"
	"google.golang.org/protobuf/proto"

	"github.com/hyperledger/fabric-x-common/protolator"
	"github.com/hyperledger/fabric-x-common/protoutil"
	"github.com/hyperledger/fabric-x-common/tools/configtxlator/update"
	"github.com/hyperledger/fabric-x-common/tools/fxadmin/core/helpers"
)

var logger = flogging.MustGetLogger("fxadmin.compute-update")

// Handler executes the compute-update command.
type Handler struct{}

// New returns a compute-update command handler.
func New() *Handler {
	return &Handler{}
}

// Run implements `fxadmin compute-update`. It encodes the current and modified
// configuration JSON at currentPath and modifiedPath to common.Config messages,
// computes the ConfigUpdate delta between them, stamps it with the channel ID
// read from the config block at currentBlockPath, and writes the marshaled
// ConfigUpdate to outputPath.
//
// The three inputs must all belong to the same channel: currentPath is expected
// to be the JSON decoded from currentBlockPath, and modifiedPath its edited
// copy. The delta is computed only from the two JSON configs, while the channel
// ID is taken only from the block. common.Config carries no channel ID, so a
// mismatch cannot be detected here: pass a block from a different channel and
// the resulting ConfigUpdate carries one channel's changes stamped with another
// channel's ID, which the orderer will reject on submission.
func (*Handler) Run(currentPath, modifiedPath, currentBlockPath, outputPath string) error {
	logger.Debugf("compute-update: current=%s modified=%s current-block=%s output=%s",
		currentPath, modifiedPath, currentBlockPath, outputPath)

	if err := helpers.RequireDistinctOutput(outputPath, currentPath, modifiedPath, currentBlockPath); err != nil {
		return err
	}

	current, err := encodeConfig(currentPath)
	if err != nil {
		return err
	}
	modified, err := encodeConfig(modifiedPath)
	if err != nil {
		return err
	}

	channelID, err := channelIDFromBlock(currentBlockPath)
	if err != nil {
		return err
	}

	configUpdate, err := update.Compute(current, modified)
	if err != nil {
		return errors.Wrap(err, "failed to compute config update")
	}
	configUpdate.ChannelId = channelID

	out, err := proto.Marshal(configUpdate)
	if err != nil {
		return errors.Wrap(err, "failed to marshal config update")
	}
	if err := os.WriteFile(outputPath, out, 0o600); err != nil {
		return errors.Wrapf(err, "failed to write output %q", outputPath)
	}

	logger.Infof("computed config update for channel %s from %s and %s to %s",
		channelID, currentPath, modifiedPath, outputPath)
	return nil
}

// channelIDFromBlock reads the config block at path and returns the channel ID
// from its channel header, which is the channel the ConfigUpdate targets. An
// empty channel ID is rejected.
func channelIDFromBlock(path string) (string, error) {
	content, err := os.ReadFile(path)
	if err != nil {
		return "", errors.Wrapf(err, "failed to read config block %q", path)
	}
	channelID, err := protoutil.GetChannelIDFromBlockBytes(content)
	if err != nil {
		return "", errors.Wrapf(err, "failed to read channel id from config block %q", path)
	}
	if channelID == "" {
		return "", errors.Newf("config block %q has an empty channel id", path)
	}
	return channelID, nil
}

// encodeConfig reads the configuration JSON at path and encodes it to a
// common.Config, mirroring `configtxlator proto_encode --type common.Config`.
func encodeConfig(path string) (*cb.Config, error) {
	input, err := os.Open(path)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to open config %q", path)
	}
	defer func() { _ = input.Close() }()

	config := &cb.Config{}
	if err := protolator.DeepUnmarshalJSON(input, config); err != nil {
		return nil, errors.Wrapf(err, "failed to decode config %q", path)
	}
	return config, nil
}
