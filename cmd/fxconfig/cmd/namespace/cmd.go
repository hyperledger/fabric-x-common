/*
Copyright IBM Corp. All Rights Reserved.

SPDX-License-Identifier: Apache-2.0
*/

package namespace

import (
	"github.com/spf13/cobra"
)

func NewNamespaceCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "namespace",
		Short: "Perform namespace operations",
		Long:  "",
	}

	cmd.AddCommand(
		newCreateCommand(),
		newListCommand(),
		newUpdateCommand(),
	)

	return cmd
}
