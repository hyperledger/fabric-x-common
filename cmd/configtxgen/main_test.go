/*
Copyright IBM Corp. 2017 All Rights Reserved.

SPDX-License-Identifier: Apache-2.0
*/

package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/hyperledger/fabric-x-common/core/config/configtest"
	"github.com/hyperledger/fabric-x-common/tools/configtxgen"
)

func TestConfigTxFlags(t *testing.T) {
	configTxDest := filepath.Join(t.TempDir(), "configtx")

	oldArgs := os.Args
	defer func() {
		os.Args = oldArgs
		flag.CommandLine = flag.NewFlagSet(os.Args[0], flag.ExitOnError)
	}()

	configtest.SetDevFabricConfigPath(t)
	devConfigDir := configtest.GetDevConfigDir()

	os.Args = []string{
		"cmd",
		"-channelID=testchannelid",
		"-outputCreateChannelTx=" + configTxDest,
		"-profile=" + configtxgen.SampleSingleMSPChannelProfile,
		"-configPath=" + devConfigDir,
		"-inspectChannelCreateTx=" + configTxDest,
		"-asOrg=" + configtxgen.SampleOrgName,
	}

	main()

	_, err := os.Stat(configTxDest)
	require.NoError(t, err, "Configtx file is written successfully")
}

func TestBlockFlags(t *testing.T) {
	blockDest := filepath.Join(t.TempDir(), "block")
	oldArgs := os.Args
	defer func() {
		os.Args = oldArgs
		flag.CommandLine = flag.NewFlagSet(os.Args[0], flag.ExitOnError)
	}()
	os.Args = []string{
		"cmd",
		"-channelID=testchannelid",
		"-profile=" + configtxgen.SampleSingleMSPSoloProfile,
		"-outputBlock=" + blockDest,
		"-inspectBlock=" + blockDest,
	}
	configtest.SetDevFabricConfigPath(t)

	main()

	_, err := os.Stat(blockDest)
	require.NoError(t, err, "Block file is written successfully")
}

func TestGetVersionInfo(t *testing.T) {
	t.Parallel()
	testSHAs := []string{"", "abcdefg"}

	for _, sha := range testSHAs {
		commitSHA = sha

		expected := fmt.Sprintf("%s:\n Version: %s\n Commit SHA: %s\n Go version: %s\n OS/Arch: %s",
			programName, version, sha, runtime.Version(),
			fmt.Sprintf("%s/%s", runtime.GOOS, runtime.GOARCH))
		require.Equal(t, expected, getVersionInfo())
	}
}
