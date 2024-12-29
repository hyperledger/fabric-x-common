/*
Copyright IBM Corp. All Rights Reserved.

SPDX-License-Identifier: Apache-2.0
*/

package node

import (
	"github.com/pkg/errors"
	"github.com/spf13/cobra"
	"github.ibm.com/decentralized-trust-research/fabricx-config/core/ledger/kvledger"
	"github.ibm.com/decentralized-trust-research/fabricx-config/internaltools/peer/common"
)

func resumeCmd() *cobra.Command {
	resumeChannelCmd.ResetFlags()
	flags := resumeChannelCmd.Flags()
	flags.StringVarP(&channelID, "channelID", "c", common.UndefinedParamValue, "Channel to resume.")

	return resumeChannelCmd
}

var resumeChannelCmd = &cobra.Command{
	Use:   "resume",
	Short: "Resumes a channel on the peer.",
	Long:  `Resumes a channel on the peer. When the command is executed, the peer must be offline. When the peer starts after resume, it will receive blocks for the resumed channel.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		if channelID == common.UndefinedParamValue {
			return errors.New("Must supply channel ID")
		}

		config := ledgerConfig()
		return kvledger.ResumeChannel(config.RootFSPath, channelID)
	},
}
