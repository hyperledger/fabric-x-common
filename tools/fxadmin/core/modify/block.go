/*
Copyright IBM Corp. All Rights Reserved.

SPDX-License-Identifier: Apache-2.0
*/

package modify

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/cockroachdb/errors"
	"github.com/hyperledger/fabric-lib-go/bccsp/factory"

	"github.com/hyperledger/fabric-x-common/common/channelconfig"
	"github.com/hyperledger/fabric-x-common/tools/fxadmin/core/helpers"
)

// readConfigBlock reads and decodes the config block file at path.
func readConfigBlock(path string) (*helpers.ConfigBlock, error) {
	content, err := os.ReadFile(path)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to read config block %q", path)
	}
	block, err := helpers.UnmarshalConfigBlock(content)
	if err != nil {
		return nil, fmt.Errorf("failed to decode config block %q: %w", path, err)
	}
	return block, nil
}

// writeConfigBlock checks that the block's edited config still builds a
// channel config bundle, and then writes the block back to path. An invalid
// edit is rejected here rather than after the update has collected its
// endorsements. The file is replaced atomically, so an error at any step leaves
// the previous block intact.
func writeConfigBlock(block *helpers.ConfigBlock, path string) error {
	channelID := block.ChannelHeader.GetChannelId()
	if _, err := channelconfig.NewBundle(channelID, block.Config(), factory.GetDefault()); err != nil {
		return fmt.Errorf("the edited config is not a valid channel config: %w", err)
	}
	content, err := block.Marshal()
	if err != nil {
		return err
	}
	return replaceFile(path, content)
}

// replaceFile writes content to a temporary file next to path and renames it
// over path, so path holds either its old or its new content, never a partial
// write. The new file keeps the permissions of the file it replaces.
func replaceFile(path string, content []byte) error {
	info, err := os.Stat(path)
	if err != nil {
		return errors.Wrapf(err, "failed to stat %q", path)
	}
	tmp, err := os.CreateTemp(filepath.Dir(path), filepath.Base(path)+".tmp-*")
	if err != nil {
		return errors.Wrapf(err, "failed to create a temporary file next to %q", path)
	}
	// Removing the temporary file is a no-op once it has been renamed over path.
	defer func() { _ = os.Remove(tmp.Name()) }()

	if _, err := tmp.Write(content); err != nil {
		_ = tmp.Close()
		return errors.Wrapf(err, "failed to write %q", tmp.Name())
	}
	if err := tmp.Chmod(info.Mode().Perm()); err != nil {
		_ = tmp.Close()
		return errors.Wrapf(err, "failed to set the permissions of %q", tmp.Name())
	}
	if err := tmp.Close(); err != nil {
		return errors.Wrapf(err, "failed to close %q", tmp.Name())
	}
	if err := os.Rename(tmp.Name(), path); err != nil {
		return errors.Wrapf(err, "failed to replace %q", path)
	}
	return nil
}
