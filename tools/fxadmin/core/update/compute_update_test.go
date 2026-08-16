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

// TestRunComputesConfigUpdate asserts `fxadmin compute-update` encodes both
// JSON configs to common.Config and writes the ConfigUpdate delta between them:
// the changed value lands in the write set with its version bumped, and the
// read set retains the original version.
func TestRunComputesConfigUpdate(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()

	currentPath := writeConfigJSON(t, dir, "current.json", "SHA256")
	modifiedPath := writeConfigJSON(t, dir, "modified.json", "SHA384")
	outputPath := filepath.Join(dir, "update.pb")

	require.NoError(t, update.New().Run(currentPath, modifiedPath, outputPath))

	out, err := os.ReadFile(outputPath)
	require.NoError(t, err)

	configUpdate := &cb.ConfigUpdate{}
	require.NoError(t, proto.Unmarshal(out, configUpdate))

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
	outputPath := filepath.Join(dir, "update.pb")

	err := update.New().Run(currentPath, modifiedPath, outputPath)
	require.ErrorContains(t, err, "no differences detected")
	require.NoFileExists(t, outputPath)
}

// TestRunMissingInput asserts a missing input config is reported and no output
// file is created.
func TestRunMissingInput(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()

	currentPath := writeConfigJSON(t, dir, "current.json", "SHA256")
	outputPath := filepath.Join(dir, "update.pb")

	err := update.New().Run(currentPath, filepath.Join(dir, "absent.json"), outputPath)
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

	outputPath := filepath.Join(dir, "update.pb")
	const priorContent = "prior artifact"
	require.NoError(t, os.WriteFile(outputPath, []byte(priorContent), 0o600))

	err := update.New().Run(currentPath, modifiedPath, outputPath)
	require.ErrorContains(t, err, "failed to decode config")

	preserved, err := os.ReadFile(outputPath)
	require.NoError(t, err)
	require.Equal(t, priorContent, string(preserved), "failed compute must not affect the destination")
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
