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

	"github.com/hyperledger/fabric-x-common/protolator"
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

	block, err := helpers.UnmarshalAndDecodeConfigBlock(in)
	if err != nil {
		return err
	}

	if err := protolator.DeepMarshalJSON(output, block.Config()); err != nil {
		return errors.Wrap(err, "failed to encode output")
	}
	return nil
}
