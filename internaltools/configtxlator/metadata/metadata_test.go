/*
Copyright IBM Corp. All Rights Reserved.

SPDX-License-Identifier: Apache-2.0
*/

package metadata_test

import (
	"fmt"
	"runtime"
	"testing"

	"github.com/stretchr/testify/require"
	"github.ibm.com/decentralized-trust-research/fabricx-config/internaltools/configtxlator/metadata"
)

func TestGetVersionInfo(t *testing.T) {
	testSHA := "abcdefg"
	metadata.CommitSHA = testSHA

	expected := fmt.Sprintf("%s:\n Version: %s\n Commit SHA: %s\n Go version: %s\n OS/Arch: %s",
		metadata.ProgramName, metadata.Version, testSHA, runtime.Version(),
		fmt.Sprintf("%s/%s", runtime.GOOS, runtime.GOARCH))
	require.Equal(t, expected, metadata.GetVersionInfo())
}
