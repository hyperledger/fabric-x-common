/*
Copyright IBM Corp. All Rights Reserved.

SPDX-License-Identifier: Apache-2.0
*/

package main

import (
	"os"

	"github.com/hyperledger/fabric-lib-go/bccsp/factory"
	"github.ibm.com/decentralized-trust-research/fabricx-config/cmd/common"
	discovery "github.ibm.com/decentralized-trust-research/fabricx-config/discovery/cmd"
)

func main() {
	factory.InitFactories(nil)
	cli := common.NewCLI("discover", "Command line client for fabric discovery service")
	discovery.AddCommands(cli)
	cli.Run(os.Args[1:])
}
