/*
Copyright IBM Corp. All Rights Reserved.

SPDX-License-Identifier: Apache-2.0
*/

package main

import (
	_ "net/http/pprof"
	"os"
	"strings"

	"github.com/hyperledger/fabric-lib-go/bccsp/factory"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"github.ibm.com/decentralized-trust-research/fabricx-config/internaltools/peer/chaincode"
	"github.ibm.com/decentralized-trust-research/fabricx-config/internaltools/peer/channel"
	"github.ibm.com/decentralized-trust-research/fabricx-config/internaltools/peer/common"
	"github.ibm.com/decentralized-trust-research/fabricx-config/internaltools/peer/lifecycle"
	"github.ibm.com/decentralized-trust-research/fabricx-config/internaltools/peer/node"
	"github.ibm.com/decentralized-trust-research/fabricx-config/internaltools/peer/snapshot"
	"github.ibm.com/decentralized-trust-research/fabricx-config/internaltools/peer/version"
)

// The main command describes the service and
// defaults to printing the help message.
var mainCmd = &cobra.Command{Use: "peer"}

func main() {
	setEnvConfig(viper.GetViper())

	// Define command-line flags that are valid for all peer commands and
	// subcommands.
	mainFlags := mainCmd.PersistentFlags()

	mainFlags.String("logging-level", "", "Legacy logging level flag")
	viper.BindPFlag("logging_level", mainFlags.Lookup("logging-level"))
	mainFlags.MarkHidden("logging-level")

	cryptoProvider := factory.GetDefault()

	mainCmd.AddCommand(version.Cmd())
	mainCmd.AddCommand(node.Cmd())
	mainCmd.AddCommand(chaincode.Cmd(nil, cryptoProvider))
	mainCmd.AddCommand(channel.Cmd(nil))
	mainCmd.AddCommand(lifecycle.Cmd(cryptoProvider))
	mainCmd.AddCommand(snapshot.Cmd(cryptoProvider))

	// On failure Cobra prints the usage message and error string, so we only
	// need to exit with a non-0 status
	if mainCmd.Execute() != nil {
		os.Exit(1)
	}
}

func setEnvConfig(v *viper.Viper) {
	v.SetEnvPrefix(common.CmdRoot)
	v.AllowEmptyEnv(true)
	v.AutomaticEnv()
	replacer := strings.NewReplacer(".", "_")
	v.SetEnvKeyReplacer(replacer)
}
