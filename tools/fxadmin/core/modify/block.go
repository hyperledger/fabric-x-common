/*
Copyright IBM Corp. All Rights Reserved.

SPDX-License-Identifier: Apache-2.0
*/

package modify

import (
	"fmt"

	"github.com/hyperledger/fabric-lib-go/bccsp/factory"

	"github.com/hyperledger/fabric-x-common/common/channelconfig"
	"github.com/hyperledger/fabric-x-common/tools/fxadmin/core/helpers"
)

// readConfigBlock reads and decodes the config block file at path.
func readConfigBlock(path string) (*helpers.ConfigBlock, error) {
	block, err := helpers.ReadConfigBlock(path)
	if err != nil {
		return nil, err
	}
	configBlock, err := helpers.DecodeConfigBlock(block)
	if err != nil {
		return nil, fmt.Errorf("failed to decode config block %q: %w", path, err)
	}
	return configBlock, nil
}

// writeConfigBlock checks that the block's edited config still builds a
// channel config bundle, and then writes the block back to the file at path,
// in place. An invalid edit is rejected here, before the file is written, rather
// than after the update has collected its endorsements.
func writeConfigBlock(block *helpers.ConfigBlock, path string) error {
	channelID := block.ChannelHeader.GetChannelId()
	if _, err := channelconfig.NewBundle(channelID, block.Config(), factory.GetDefault()); err != nil {
		return fmt.Errorf("the edited config is not a valid channel config: %w", err)
	}
	if err := block.Encode(); err != nil {
		return err
	}
	return helpers.WriteBlock(block.Block, path)
}
