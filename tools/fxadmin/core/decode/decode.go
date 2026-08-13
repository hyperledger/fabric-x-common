/*
Copyright IBM Corp. All Rights Reserved.

SPDX-License-Identifier: Apache-2.0
*/

// Package decode implements the `fxadmin decode` command, which converts a
// binary config block into human-readable JSON. It reproduces the logic of
// `configtxlator proto_decode --type common.Block`.
package decode

import (
	"bytes"
	"io"
	"os"
	"path/filepath"
	"reflect"

	"github.com/cockroachdb/errors"
	"github.com/hyperledger/fabric-lib-go/common/flogging"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protoreflect"
	"google.golang.org/protobuf/reflect/protoregistry"

	"github.com/hyperledger/fabric-x-common/protolator"
)

var logger = flogging.MustGetLogger("fxadmin.decode")

const blockMessageType protoreflect.FullName = "common.Block"

// Handler executes the decode command.
type Handler struct{}

// New returns a decode command handler.
func New() *Handler {
	return &Handler{}
}

// Run implements `fxadmin decode`. It decodes the binary config block at
// blockPath and writes its JSON rendering to outputPath. The output is written
// only after the block decodes successfully, so a malformed block never
// clobbers an existing destination.
func (*Handler) Run(blockPath, outputPath string) error {
	logger.Debugf("decode: block=%s output=%s", blockPath, outputPath)

	if err := requireDistinctPaths(blockPath, outputPath); err != nil {
		return err
	}

	input, err := os.Open(blockPath)
	if err != nil {
		return errors.Wrapf(err, "failed to open block %q", blockPath)
	}
	defer func() { _ = input.Close() }()

	var rendered bytes.Buffer
	if err := decodeProto(input, &rendered); err != nil {
		return err
	}

	if err := os.WriteFile(outputPath, rendered.Bytes(), 0o600); err != nil {
		return errors.Wrapf(err, "failed to write output %q", outputPath)
	}

	logger.Infof("decoded %s to %s\n", blockPath, outputPath)
	return nil
}

// requireDistinctPaths rejects a block and output that resolve to the same
// file, so decoding never overwrites its own source.
func requireDistinctPaths(blockPath, outputPath string) error {
	blockAbs, err := filepath.Abs(blockPath)
	if err != nil {
		return errors.Wrapf(err, "failed to resolve block path %q", blockPath)
	}
	outputAbs, err := filepath.Abs(outputPath)
	if err != nil {
		return errors.Wrapf(err, "failed to resolve output path %q", outputPath)
	}
	if blockAbs == outputAbs {
		return errors.Newf("block and output must be different files, both resolve to %q", blockAbs)
	}
	return nil
}

func decodeProto(input io.Reader, output io.Writer) error {
	mt, err := protoregistry.GlobalTypes.FindMessageByName(blockMessageType)
	if err != nil {
		return errors.Wrapf(err, "failed to find message type %q", blockMessageType)
	}

	msgType := reflect.TypeOf(mt.Zero().Interface())
	if msgType == nil {
		return errors.Newf("message of type %q unknown", blockMessageType)
	}
	msg, ok := reflect.New(msgType.Elem()).Interface().(proto.Message)
	if !ok {
		return errors.Newf("message of type %q is not a proto.Message", blockMessageType)
	}

	in, err := io.ReadAll(input)
	if err != nil {
		return errors.Wrap(err, "failed to read input")
	}
	if err := proto.Unmarshal(in, msg); err != nil {
		return errors.Wrap(err, "failed to unmarshal input")
	}
	if err := protolator.DeepMarshalJSON(output, msg); err != nil {
		return errors.Wrap(err, "failed to encode output")
	}

	return nil
}
