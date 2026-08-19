/*
Copyright IBM Corp. All Rights Reserved.

SPDX-License-Identifier: Apache-2.0
*/

package update_test

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"

	cb "github.com/hyperledger/fabric-protos-go-apiv2/common"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/proto"

	"github.com/hyperledger/fabric-x-common/protolator"
	"github.com/hyperledger/fabric-x-common/protoutil"
	"github.com/hyperledger/fabric-x-common/tools/fxadmin/core/update"
)

// testChannelID is the channel the config block written by writeConfigBlock
// names, and the channel ID the computed update is expected to target.
const testChannelID = "arma"

// TestRunComputesConfigUpdate asserts `fxadmin compute-update` encodes both
// JSON configs to common.Config and writes the ConfigUpdate delta between them:
// the changed value lands in the write set with its version bumped, the read
// set retains the original version, and the update carries the channel ID read
// from the current config block.
func TestRunComputesConfigUpdate(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()

	currentPath := writeConfigJSON(t, dir, "current.json", "SHA256")
	modifiedPath := writeConfigJSON(t, dir, "modified.json", "SHA384")
	blockPath := writeConfigBlock(t, dir)
	outputPath := filepath.Join(dir, "update.pb")

	require.NoError(t, update.New().Run(currentPath, modifiedPath, blockPath, outputPath))

	out, err := os.ReadFile(outputPath)
	require.NoError(t, err)

	configUpdate := &cb.ConfigUpdate{}
	require.NoError(t, proto.Unmarshal(out, configUpdate))

	require.Equal(t, testChannelID, configUpdate.GetChannelId(), "update must target the block's channel")

	writeValue := configUpdate.GetWriteSet().GetValues()["HashingAlgorithm"]
	require.NotNil(t, writeValue, "changed value must be in the write set")
	require.EqualValues(t, 1, writeValue.GetVersion(), "changed value version must be bumped")
	hashingAlgorithm := &cb.HashingAlgorithm{}
	require.NoError(t, proto.Unmarshal(writeValue.GetValue(), hashingAlgorithm))
	require.Equal(t, "SHA384", hashingAlgorithm.GetName(), "changed value must carry the new algorithm")

	require.NotContains(t, configUpdate.GetReadSet().GetValues(), "HashingAlgorithm")
	require.EqualValues(t, 0, configUpdate.GetReadSet().GetVersion())
	require.EqualValues(t, 0, configUpdate.GetWriteSet().GetVersion())
}

// TestRunNoDifferences asserts identical configs are rejected with the
// underlying "no differences" error and no output file is produced.
func TestRunNoDifferences(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()

	currentPath := writeConfigJSON(t, dir, "current.json", "SHA256")
	modifiedPath := writeConfigJSON(t, dir, "modified.json", "SHA256")
	blockPath := writeConfigBlock(t, dir)
	outputPath := filepath.Join(dir, "update.pb")

	err := update.New().Run(currentPath, modifiedPath, blockPath, outputPath)
	require.ErrorContains(t, err, "no differences detected")
	require.NoFileExists(t, outputPath)
}

// TestRunMissingInput asserts a missing input config is reported and no output
// file is created.
func TestRunMissingInput(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()

	currentPath := writeConfigJSON(t, dir, "current.json", "SHA256")
	blockPath := writeConfigBlock(t, dir)
	outputPath := filepath.Join(dir, "update.pb")

	err := update.New().Run(currentPath, filepath.Join(dir, "absent.json"), blockPath, outputPath)
	require.ErrorContains(t, err, "failed to open config")
	require.NoFileExists(t, outputPath)
}

// TestRunMalformedInput asserts non-JSON input is rejected with a decode error
// and leaves any pre-existing output untouched.
func TestRunMalformedInput(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()

	currentPath := writeConfigJSON(t, dir, "current.json", "SHA256")
	modifiedPath := filepath.Join(dir, "modified.json")
	require.NoError(t, os.WriteFile(modifiedPath, []byte("not valid json"), 0o600))
	blockPath := writeConfigBlock(t, dir)

	outputPath := filepath.Join(dir, "update.pb")
	const priorContent = "prior artifact"
	require.NoError(t, os.WriteFile(outputPath, []byte(priorContent), 0o600))

	err := update.New().Run(currentPath, modifiedPath, blockPath, outputPath)
	require.ErrorContains(t, err, "failed to decode config")

	preserved, err := os.ReadFile(outputPath)
	require.NoError(t, err)
	require.Equal(t, priorContent, string(preserved), "failed compute must not affect the destination")
}

// TestRunMissingBlock asserts a missing current block is reported and no output
// file is produced.
func TestRunMissingBlock(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()

	currentPath := writeConfigJSON(t, dir, "current.json", "SHA256")
	modifiedPath := writeConfigJSON(t, dir, "modified.json", "SHA384")
	outputPath := filepath.Join(dir, "update.pb")

	err := update.New().Run(currentPath, modifiedPath, filepath.Join(dir, "absent.pb"), outputPath)
	require.ErrorContains(t, err, "failed to read config block")
	require.NoFileExists(t, outputPath)
}

// TestRunMalformedBlock asserts a current block that is not a valid block is
// rejected and no output file is produced.
func TestRunMalformedBlock(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()

	currentPath := writeConfigJSON(t, dir, "current.json", "SHA256")
	modifiedPath := writeConfigJSON(t, dir, "modified.json", "SHA384")
	blockPath := filepath.Join(dir, "current_block.pb")
	require.NoError(t, os.WriteFile(blockPath, []byte("not a block"), 0o600))
	outputPath := filepath.Join(dir, "update.pb")

	err := update.New().Run(currentPath, modifiedPath, blockPath, outputPath)
	require.ErrorContains(t, err, "failed to read channel id from config block")
	require.NoFileExists(t, outputPath)
}

// TestRunEmptyChannelID asserts a config block whose channel header carries an
// empty channel ID is rejected up front, rather than stamping the ConfigUpdate
// with an empty channel and deferring the failure to submission.
func TestRunEmptyChannelID(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()

	currentPath := writeConfigJSON(t, dir, "current.json", "SHA256")
	modifiedPath := writeConfigJSON(t, dir, "modified.json", "SHA384")
	blockPath := writeConfigBlockWithChannelID(t, dir, "")
	outputPath := filepath.Join(dir, "update.pb")

	err := update.New().Run(currentPath, modifiedPath, blockPath, outputPath)
	require.ErrorContains(t, err, "empty channel id")
	require.NoFileExists(t, outputPath)
}

// TestRunRejectsOutputCollidingWithInput asserts an output that resolves to one
// of the inputs is rejected before any work, so the command never overwrites
// its own source.
func TestRunRejectsOutputCollidingWithInput(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()

	currentPath := writeConfigJSON(t, dir, "current.json", "SHA256")
	modifiedPath := writeConfigJSON(t, dir, "modified.json", "SHA384")
	blockPath := writeConfigBlock(t, dir)
	modifiedContent, err := os.ReadFile(modifiedPath)
	require.NoError(t, err)

	// A distinct path string that resolves to the modified input.
	outputPath := filepath.Join(dir, ".", "modified.json")
	err = update.New().Run(currentPath, modifiedPath, blockPath, outputPath)
	require.ErrorContains(t, err, "must be a different file from input")

	// The input must be left byte-for-byte intact.
	preserved, err := os.ReadFile(modifiedPath)
	require.NoError(t, err)
	require.Equal(t, modifiedContent, preserved, "collision check must run before any write")
}

// writeConfigJSON writes a minimal common.Config JSON whose single
// HashingAlgorithm value carries algo, and returns its path.
func writeConfigJSON(t *testing.T, dir, name, algo string) string {
	t.Helper()
	config := &cb.Config{ChannelGroup: &cb.ConfigGroup{
		Version: 0,
		Values: map[string]*cb.ConfigValue{
			"HashingAlgorithm": {
				Version:   0,
				ModPolicy: "Admins",
				Value:     protoutil.MarshalOrPanic(&cb.HashingAlgorithm{Name: algo}),
			},
		},
		ModPolicy: "Admins",
	}}

	var buf bytes.Buffer
	require.NoError(t, protolator.DeepMarshalJSON(&buf, config))

	path := filepath.Join(dir, name)
	require.NoError(t, os.WriteFile(path, buf.Bytes(), 0o600))
	return path
}

// writeConfigBlock writes a marshaled common.Block whose sole envelope carries a
// channel header naming testChannelID, and returns its path. This is the shape
// from which compute-update reads the channel ID the update targets.
func writeConfigBlock(t *testing.T, dir string) string {
	t.Helper()
	return writeConfigBlockWithChannelID(t, dir, testChannelID)
}

// writeConfigBlockWithChannelID writes a marshaled common.Block whose sole
// envelope carries a channel header naming channelID, and returns its path.
func writeConfigBlockWithChannelID(t *testing.T, dir, channelID string) string {
	t.Helper()
	block := protoutil.NewBlock(0, []byte("previous-hash"))
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

	path := filepath.Join(dir, "current_block.pb")
	require.NoError(t, os.WriteFile(path, protoutil.MarshalOrPanic(block), 0o600))
	return path
}
