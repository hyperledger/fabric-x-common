/*
Copyright IBM Corp. All Rights Reserved.

SPDX-License-Identifier: Apache-2.0
*/

// Package decode implements the `fxadmin decode` command, which converts a
// binary config block into the human-editable JSON of the common.Config it
// carries. It reproduces `configtxlator proto_decode --type common.Block`
// followed by extracting the embedded common.Config, so the output is the shape
// `fxadmin compute-update` consumes: an admin edits this JSON and feeds the
// original and edited copies back to compute-update.
package decode

import (
	"bytes"
	"io"
	"os"

	"github.com/cockroachdb/errors"
	"github.com/hyperledger/fabric-lib-go/common/flogging"
	cb "github.com/hyperledger/fabric-protos-go-apiv2/common"
	"google.golang.org/protobuf/proto"

	"github.com/hyperledger/fabric-x-common/protolator"
	"github.com/hyperledger/fabric-x-common/protoutil"
	"github.com/hyperledger/fabric-x-common/tools/fxadmin/core/helpers"
)

var logger = flogging.MustGetLogger("fxadmin.decode")

// Handler executes the decode command.
type Handler struct{}

// New returns a decode command handler.
func New() *Handler {
	return &Handler{}
}

// Run implements `fxadmin decode`. It reads the binary config block at
// blockPath, extracts the embedded common.Config, and writes its JSON rendering
// to outputPath. The output is written only after the block decodes
// successfully, so a malformed block never clobbers an existing destination.
func (*Handler) Run(blockPath, outputPath string) error {
	logger.Debugf("decode: block=%s output=%s", blockPath, outputPath)

	if err := helpers.RequireDistinctOutput(outputPath, blockPath); err != nil {
		return err
	}

	input, err := os.Open(blockPath)
	if err != nil {
		return errors.Wrapf(err, "failed to open block %q", blockPath)
	}
	defer func() { _ = input.Close() }()

	var rendered bytes.Buffer
	if err := decodeConfig(input, &rendered); err != nil {
		return err
	}

	if err := os.WriteFile(outputPath, rendered.Bytes(), 0o600); err != nil {
		return errors.Wrapf(err, "failed to write output %q", outputPath)
	}

	logger.Infof("decoded config from %s to %s\n", blockPath, outputPath)
	return nil
}

// decodeConfig reads a marshaled common.Block from input, extracts the
// common.Config carried by its configuration envelope, and writes the config's
// JSON rendering to output.
func decodeConfig(input io.Reader, output io.Writer) error {
	in, err := io.ReadAll(input)
	if err != nil {
		return errors.Wrap(err, "failed to read input")
	}

	config, err := configFromBlock(in)
	if err != nil {
		return err
	}

	if err := protolator.DeepMarshalJSON(output, config); err != nil {
		return errors.Wrap(err, "failed to encode output")
	}
	return nil
}

// configFromBlock unmarshals a marshaled common.Block and returns the
// common.Config carried by the configuration transaction in its first envelope.
// It errors if the block is not a config block.
func configFromBlock(marshaledBlock []byte) (*cb.Config, error) {
	block := &cb.Block{}
	if err := proto.Unmarshal(marshaledBlock, block); err != nil {
		return nil, errors.Wrap(err, "failed to unmarshal block input")
	}

	envelope, err := protoutil.ExtractEnvelope(block, 0)
	if err != nil {
		return nil, errors.Wrap(err, "failed to extract config envelope from block")
	}

	payload, err := protoutil.UnmarshalPayload(envelope.Payload)
	if err != nil {
		return nil, errors.Wrap(err, "failed to unmarshal config payload")
	}

	configEnvelope, err := protoutil.UnmarshalConfigEnvelope(payload.Data)
	if err != nil {
		return nil, errors.Wrap(err, "failed to unmarshal config envelope")
	}

	if configEnvelope.GetConfig() == nil {
		return nil, errors.New("block does not carry a config: config envelope has no config")
	}
	return configEnvelope.GetConfig(), nil
}
