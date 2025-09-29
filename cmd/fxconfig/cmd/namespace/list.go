/*
Copyright IBM Corp. All Rights Reserved.

SPDX-License-Identifier: Apache-2.0
*/

package namespace

import (
	"github.com/spf13/cobra"

	"github.com/hyperledger/fabric-x-common/internaltools/fxconfig/namespace"
)

func newListCommand() *cobra.Command {
	// this is our default query service endpoint
	endpoint := "localhost:7001"

	cmd := &cobra.Command{
		Use:   "list",
		Short: "List installed Namespaces",
		Long:  "",
		RunE: func(_ *cobra.Command, _ []string) error {
			return namespace.List(endpoint)
		},
	}

	cmd.PersistentFlags().StringVar(&endpoint, "endpoint", "", "committer query service endpoint")

	return cmd
}
