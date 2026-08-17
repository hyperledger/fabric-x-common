/*
Copyright IBM Corp. All Rights Reserved.

SPDX-License-Identifier: Apache-2.0
*/

package decode_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	cb "github.com/hyperledger/fabric-protos-go-apiv2/common"
	"github.com/stretchr/testify/require"

	"github.com/hyperledger/fabric-x-common/protoutil"
	"github.com/hyperledger/fabric-x-common/tools/fxadmin/core/decode"
)

// TestRunDecodesBlockToJSON asserts `fxadmin decode` reproduces the
// `configtxlator proto_decode --type common.Block` rendering: the output is
// valid JSON, the block header is preserved, and protolator deep-decodes the
// opaque envelope bytes into their structured form.
func TestRunDecodesBlockToJSON(t *testing.T) {
	t.Parallel()

	const channelID = "arma"
	block := protoutil.NewBlock(7, []byte("previous-hash"))
	block.Data.Data = [][]byte{protoutil.MarshalOrPanic(&cb.Envelope{
		Payload: protoutil.MarshalOrPanic(&cb.Payload{
			Header: &cb.Header{
				ChannelHeader: protoutil.MarshalOrPanic(&cb.ChannelHeader{
					Type:      int32(cb.HeaderType_CONFIG),
					ChannelId: channelID,
				}),
			},
		}),
	})}

	dir := t.TempDir()
	blockPath := filepath.Join(dir, "block.pb")
	require.NoError(t, os.WriteFile(blockPath, protoutil.MarshalOrPanic(block), 0o600))
	outputPath := filepath.Join(dir, "block.json")

	require.NoError(t, decode.New().Run(blockPath, outputPath))

	out, err := os.ReadFile(outputPath)
	require.NoError(t, err)

	var decoded map[string]any
	require.NoError(t, json.Unmarshal(out, &decoded), "output must be valid JSON")

	header, ok := decoded["header"].(map[string]any)
	require.True(t, ok, "decoded block must have a header object")
	require.Equal(t, "7", header["number"], "block number must be preserved")

	data := nestedMap(t, decoded, "data")
	envelopes, ok := data["data"].([]any)
	require.True(t, ok, "block data must be a list of envelopes")
	require.Len(t, envelopes, 1)
	envelope, ok := envelopes[0].(map[string]any)
	require.True(t, ok, "envelope must be an object")
	channelHeader := nestedMap(t, envelope, "payload", "header", "channel_header")
	require.Equal(t, channelID, channelHeader["channel_id"])
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
	require.ErrorContains(t, err, "failed to unmarshal input")

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
	blockPath := filepath.Join(dir, "block.pb")
	require.NoError(t, os.WriteFile(blockPath, protoutil.MarshalOrPanic(protoutil.NewBlock(1, nil)), 0o600))

	// A distinct path string that resolves to the same file.
	otuputPath := filepath.Join(dir, ".", "block.pb")
	err := decode.New().Run(blockPath, otuputPath)
	require.ErrorContains(t, err, "must be a different file from input")

	// The source block must be left byte-for-byte intact.
	preserved, err := os.ReadFile(blockPath)
	require.NoError(t, err)
	require.Equal(t, protoutil.MarshalOrPanic(protoutil.NewBlock(1, nil)), preserved)
}

// nestedMap walks a chain of object keys through a decoded JSON map, failing
// the test if any key is missing or not an object.
func nestedMap(t *testing.T, data map[string]any, path ...string) map[string]any {
	t.Helper()
	curr := data
	for i, key := range path {
		v, ok := curr[key]
		require.Truef(t, ok, "key %q not found at path %v", key, path[:i])
		curr, ok = v.(map[string]any)
		require.Truef(t, ok, "value at %q is not an object at path %v", key, path[:i+1])
	}
	return curr
}
