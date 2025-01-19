/*
Copyright IBM Corp. All Rights Reserved.

SPDX-License-Identifier: Apache-2.0
*/

package node

import (
	"path/filepath"

	"github.com/pkg/errors"
	"github.com/spf13/cobra"
	coreconfig "github.ibm.com/decentralized-trust-research/fabricx-config/core/config"
	"github.ibm.com/decentralized-trust-research/fabricx-config/core/ledger/kvledger"
	"github.ibm.com/decentralized-trust-research/fabricx-config/core/transientstore"
	"github.ibm.com/decentralized-trust-research/fabricx-config/internaltools/peer/common"
)

func unjoinCmd() *cobra.Command {
	var channelID string

	cmd := &cobra.Command{
		Use:   "unjoin",
		Short: "Unjoin the peer from a channel.",
		Long:  "Unjoin the peer from a channel.  When the command is executed, the peer must be offline.",
		RunE: func(cmd *cobra.Command, args []string) error {
			if channelID == common.UndefinedParamValue {
				return errors.New("Must supply channel ID")
			}

			if err := unjoinChannel(channelID); err != nil {
				return err
			}

			return nil
		},
	}
	flags := cmd.Flags()
	flags.StringVarP(&channelID, "channelID", "c", common.UndefinedParamValue, "Channel to unjoin.")

	return cmd
}

// unjoin the peer from a channel.
func unjoinChannel(channelID string) error {
	// transient storage must be scrubbed prior to removing the kvledger for the channel.  Once the
	// kvledger storage has been removed, a subsequent ledger removal will return a "no such ledger" error.
	// By removing the transient storage prior to deleting the ledger, a crash may be recovered by re-running
	// the peer unjoin.
	transientStoragePath := filepath.Join(coreconfig.GetPath("peer.fileSystemPath"), "transientstore")
	if err := transientstore.Drop(transientStoragePath, channelID); err != nil {
		return err
	}

	config := ledgerConfig()
	if err := kvledger.UnjoinChannel(config, channelID); err != nil {
		return err
	}

	return nil
}
