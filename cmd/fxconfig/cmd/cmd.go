/*
Copyright IBM Corp. All Rights Reserved.

SPDX-License-Identifier: Apache-2.0
*/

package cmd

import (
	"github.com/spf13/cobra"

	"github.com/hyperledger/fabric-x-common/cmd/fxconfig/cmd/namespace"
)

func Execute() error {
	rootCmd := &cobra.Command{Use: "fxconfig"}
	rootCmd.AddCommand(NewVersionCmd())
	rootCmd.AddCommand(namespace.NewNamespaceCommand())

	return rootCmd.Execute()
}
