/*
Copyright IBM Corp. All Rights Reserved.

SPDX-License-Identifier: Apache-2.0
*/

package helpers

import (
	"os"

	"github.com/cockroachdb/errors"
	cb "github.com/hyperledger/fabric-protos-go-apiv2/common"
	"google.golang.org/protobuf/proto"

	"github.com/hyperledger/fabric-x-common/protoutil"
)

// ReadConfigBlock reads and unmarshals a protobuf config block from path. It is
// shared by the commands that take a --current-block file and by the modify
// commands, which edit the block file given by --block.
func ReadConfigBlock(path string) (*cb.Block, error) {
	content, err := os.ReadFile(path)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to read config block %q", path)
	}
	block, err := protoutil.UnmarshalBlock(content)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to unmarshal config block %q", path)
	}
	return block, nil
}

// WriteBlock marshals block and writes it to path.
func WriteBlock(block *cb.Block, path string) error {
	content, err := proto.Marshal(block)
	if err != nil {
		return errors.Wrap(err, "failed to marshal block")
	}
	if err := os.WriteFile(path, content, 0o600); err != nil {
		return errors.Wrapf(err, "failed to write block to %q", path)
	}
	return nil
}
