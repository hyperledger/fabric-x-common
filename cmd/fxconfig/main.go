// Copyright IBM Corp. All Rights Reserved.
//
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"os"

	"github.com/hyperledger/fabric-x-common/cmd/fxconfig/cmd"
)

func main() {
	// flogging.Init(flogging.Config{
	//	Format:  "%{message}",
	//	LogSpec: "grpc=error:debug",
	// })

	if err := cmd.Execute(); err != nil {
		os.Exit(1)
	}
}
