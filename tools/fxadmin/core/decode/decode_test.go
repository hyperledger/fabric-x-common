/*
Copyright IBM Corp. All Rights Reserved.

SPDX-License-Identifier: Apache-2.0
*/

package decode_test

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	cb "github.com/hyperledger/fabric-protos-go-apiv2/common"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/proto"

	"github.com/hyperledger/fabric-x-common/protolator"
	"github.com/hyperledger/fabric-x-common/protoutil"
	"github.com/hyperledger/fabric-x-common/tools/fxadmin/core/decode"
)

// testChannelID is the channel named by the config block writeConfigBlock
// builds.
const testChannelID = "arma"

// TestRunExtractsConfigToJSON asserts `fxadmin decode` extracts the common.Config
// embedded in a config block and renders it as JSON. the output carries the
// config's top-level channel_group, and round-trips back
// into a common.Config via protolator — the shape `compute-update`
// consumes.
func TestRunExtractsConfigToJSON(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()

	blockPath := writeConfigBlock(t, dir, "SHA256")
	outputPath := filepath.Join(dir, "config.json")

	require.NoError(t, decode.New().Run(blockPath, outputPath))

	out, err := os.ReadFile(outputPath)
	require.NoError(t, err)

	var decoded map[string]any
	require.NoError(t, json.Unmarshal(out, &decoded), "output must be valid JSON")
	require.Contains(t, decoded, "channel_group", "output must be a common.Config, not a block")
	require.NotContains(t, decoded, "data", "output must not carry the block wrapper")
	require.NotContains(t, decoded, "header", "output must not carry the block wrapper")

	// The output must round-trip into a common.Config, proving it is the shape
	// compute-update's encodeConfig accepts.
	config := &cb.Config{}
	require.NoError(t, protolator.DeepUnmarshalJSON(bytes.NewReader(out), config))
	hashingValue := config.GetChannelGroup().GetValues()["HashingAlgorithm"]
	require.NotNil(t, hashingValue, "channel_group values must survive the round-trip")
	hashingAlgorithm := &cb.HashingAlgorithm{}
	require.NoError(t, proto.Unmarshal(hashingValue.GetValue(), hashingAlgorithm))
	require.Equal(t, "SHA256", hashingAlgorithm.GetName())
}

// TestRunRejectsNonConfigBlock asserts a block whose envelope carries no
// ConfigEnvelope is rejected and leaves no output behind.
func TestRunRejectsNonConfigBlock(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()

	// A block whose sole envelope payload has no config data.
	block := protoutil.NewBlock(1, []byte("previous-hash"))
	block.Data.Data = [][]byte{protoutil.MarshalOrPanic(&cb.Envelope{
		Payload: protoutil.MarshalOrPanic(&cb.Payload{
			Header: &cb.Header{
				ChannelHeader: protoutil.MarshalOrPanic(&cb.ChannelHeader{
					Type:      int32(cb.HeaderType_CONFIG),
					ChannelId: testChannelID,
				}),
			},
		}),
	})}
	blockPath := filepath.Join(dir, "block.pb")
	require.NoError(t, os.WriteFile(blockPath, protoutil.MarshalOrPanic(block), 0o600))

	outputPath := filepath.Join(dir, "config.json")
	err := decode.New().Run(blockPath, outputPath)
	require.ErrorContains(t, err, "config envelope has no config")
	require.NoFileExists(t, outputPath)
}

// TestRunRejectsWrongHeaderType asserts a block whose channel header type is not
// HeaderType_CONFIG is rejected up front, even when its payload data decodes into
// a ConfigEnvelope carrying a non-nil Config.
func TestRunRejectsWrongHeaderType(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()

	// A config-shaped payload, but stamped with a non-CONFIG header type.
	config := &cb.Config{ChannelGroup: &cb.ConfigGroup{}}
	block := protoutil.NewBlock(1, []byte("previous-hash"))
	block.Data.Data = [][]byte{protoutil.MarshalOrPanic(&cb.Envelope{
		Payload: protoutil.MarshalOrPanic(&cb.Payload{
			Header: &cb.Header{
				ChannelHeader: protoutil.MarshalOrPanic(&cb.ChannelHeader{
					Type:      int32(cb.HeaderType_ENDORSER_TRANSACTION),
					ChannelId: testChannelID,
				}),
			},
			Data: protoutil.MarshalOrPanic(&cb.ConfigEnvelope{Config: config}),
		}),
	})}
	blockPath := filepath.Join(dir, "block.pb")
	require.NoError(t, os.WriteFile(blockPath, protoutil.MarshalOrPanic(block), 0o600))

	outputPath := filepath.Join(dir, "config.json")
	err := decode.New().Run(blockPath, outputPath)
	require.ErrorContains(t, err, "block is not a config block")
	require.NoFileExists(t, outputPath)
}

// TestRunMissingBlockFile asserts a missing input block is reported and no
// output file is created.
func TestRunMissingBlockFile(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	outputPath := filepath.Join(dir, "out.json")

	err := decode.New().Run(filepath.Join(dir, "absent.pb"), outputPath)
	require.ErrorContains(t, err, "failed to open block")
	require.NoFileExists(t, outputPath)
}

// TestRunMalformedBlock asserts non-protobuf input is rejected with an
// error and leaves any pre-existing output untouched.
func TestRunMalformedBlock(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	blockPath := filepath.Join(dir, "block.pb")
	// Bytes that are not a valid wire-format Block message.
	require.NoError(t, os.WriteFile(blockPath, []byte{0xff, 0xff, 0xff, 0xff}, 0o600))

	// A prior artifact at the destination must survive a failed decode.
	outputPath := filepath.Join(dir, "out.json")
	const priorContent = "prior artifact"
	require.NoError(t, os.WriteFile(outputPath, []byte(priorContent), 0o600))

	err := decode.New().Run(blockPath, outputPath)
	require.ErrorContains(t, err, "failed to unmarshal block input")

	preserved, err := os.ReadFile(outputPath)
	require.NoError(t, err)
	require.Equal(t, priorContent, string(preserved), "failed decode must not affect the destination")
}

// TestRunRejectsSameBlockAndOutput asserts a block and output that resolve to
// the same file are rejected before the block is read, so decoding never
// destroys its own source.
func TestRunRejectsSameBlockAndOutput(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	blockPath := writeConfigBlock(t, dir, "SHA256")
	source, err := os.ReadFile(blockPath)
	require.NoError(t, err)

	// A distinct path string that resolves to the same file.
	outputPath := filepath.Join(dir, ".", filepath.Base(blockPath))
	err = decode.New().Run(blockPath, outputPath)
	require.ErrorContains(t, err, "must be a different file from input")

	// The source block must be left byte-for-byte intact.
	preserved, err := os.ReadFile(blockPath)
	require.NoError(t, err)
	require.Equal(t, source, preserved)
}

// writeConfigBlock writes a marshaled common.Block whose sole envelope carries a
// ConfigEnvelope wrapping a common.Config: a channel header naming testChannelID
// and a single HashingAlgorithm value carrying algo. It returns the block path.
func writeConfigBlock(t *testing.T, dir, algo string) string {
	t.Helper()
	config := &cb.Config{ChannelGroup: &cb.ConfigGroup{
		Values: map[string]*cb.ConfigValue{
			"HashingAlgorithm": {
				ModPolicy: "Admins",
				Value:     protoutil.MarshalOrPanic(&cb.HashingAlgorithm{Name: algo}),
			},
		},
		ModPolicy: "Admins",
	}}
	block := protoutil.NewBlock(0, []byte("previous-hash"))
	block.Data.Data = [][]byte{protoutil.MarshalOrPanic(&cb.Envelope{
		Payload: protoutil.MarshalOrPanic(&cb.Payload{
			Header: &cb.Header{
				ChannelHeader: protoutil.MarshalOrPanic(&cb.ChannelHeader{
					Type:      int32(cb.HeaderType_CONFIG),
					ChannelId: testChannelID,
				}),
			},
			Data: protoutil.MarshalOrPanic(&cb.ConfigEnvelope{Config: config}),
		}),
	})}

	path := filepath.Join(dir, "config_block.pb")
	require.NoError(t, os.WriteFile(path, protoutil.MarshalOrPanic(block), 0o600))
	return path
}
