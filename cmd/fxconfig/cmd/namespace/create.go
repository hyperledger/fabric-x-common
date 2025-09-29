/*
Copyright IBM Corp. All Rights Reserved.

SPDX-License-Identifier: Apache-2.0
*/

package namespace

import (
	"errors"
	"time"

	"github.com/spf13/cobra"

	"github.com/hyperledger/fabric-x-common/internaltools/fxconfig/namespace"
)

func newCreateCommand() *cobra.Command {
	var ordererCfg namespace.OrdererConfig
	var mspCfg namespace.MSPConfig
	var pkPath string

	cmd := &cobra.Command{
		Use:   "create NAMESPACE_NAME",
		Short: "Create Namespace",
		Long:  "",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			nsID := args[0]

			channelName, err := cmd.Flags().GetString("channel")
			if err != nil {
				return err
			}

			if channelName == "" {
				return errors.New("you must specify a channel name '--channel channelName'")
			}

			return namespace.DeployNamespace(channelName, nsID, -1, ordererCfg, mspCfg, pkPath)
		},
	}

	cmd.PersistentFlags().String("channel", "", "The name of the channel")

	// adds flags for orderer-related commands
	cmd.PersistentFlags().StringVarP(&ordererCfg.OrderingEndpoint, "orderer", "o", "",
		"Ordering service endpoint")
	cmd.PersistentFlags().StringVarP(&ordererCfg.Config.CertPath, "cafile", "", "",
		"Path to file containing PEM-encoded trusted certificate(s) for the ordering endpoint")
	cmd.PersistentFlags().StringVarP(&ordererCfg.Config.KeyPath, "keyfile", "", "",
		"Path to file containing PEM-encoded private key to use for mutual TLS communication with the orderer endpoint")
	cmd.PersistentFlags().StringVarP(&ordererCfg.Config.CertPath, "certfile", "", "",
		"Path to file containing PEM-encoded X509 public key to use for mutual TLS communication with the orderer endpoint")
	cmd.PersistentFlags().DurationVarP(&ordererCfg.Config.Timeout, "connTimeout", "", 3*time.Second,
		"Timeout for client to connect")

	// adds flags to specify the MSP that will sign the requests
	cmd.PersistentFlags().StringVarP(&mspCfg.MSPConfigPath, "mspConfigPath", "", "", "The path to the MSP config directory")
	cmd.PersistentFlags().StringVarP(&mspCfg.MSPID, "mspID", "", "", "The name of the MSP")

	cmd.PersistentFlags().StringVarP(&pkPath, "pk", "", "", "The path to the public key of the endorser")
	_ = cmd.PersistentFlags().MarkDeprecated("pk", "This flag is deprecated and will be removed in future versions.")

	return cmd
}
