/*
Copyright IBM Corp. All Rights Reserved.

SPDX-License-Identifier: Apache-2.0
*/

// Package ledger implements the `fxadmin ledger` command family: querying the
// assembler ledger for its height, a block, or the last config
// block.
package ledger

import (
	"fmt"
	"os"

	"github.com/cockroachdb/errors"
	"github.com/hyperledger/fabric-lib-go/bccsp"
	"github.com/hyperledger/fabric-lib-go/bccsp/factory"
	"github.com/hyperledger/fabric-lib-go/common/flogging"
	cb "github.com/hyperledger/fabric-protos-go-apiv2/common"
	"google.golang.org/protobuf/proto"

	"github.com/hyperledger/fabric-x-common/protoutil"
	"github.com/hyperledger/fabric-x-common/tools/fxadmin/core/client"
	"github.com/hyperledger/fabric-x-common/tools/fxadmin/core/user"
)

var logger = flogging.MustGetLogger("fxadmin.ledger")

// Handler executes the ledger subcommands. It carries the BCCSP used to build
// the channel config bundle when reading connection information from the config
// block.
type Handler struct {
	csp bccsp.BCCSP
}

// New returns a ledger command handler.
func New() *Handler {
	return &Handler{csp: factory.GetDefault()}
}

// Height implements `fxadmin ledger height`. It prints the ledger height (the
// newest block number plus one).
func (h *Handler) Height(configPath, currentBlockPath string) error {
	logger.Debugf("ledger height: config=%s current-block=%s", configPath, currentBlockPath)
	cl, err := h.newOrdererClient(configPath, currentBlockPath)
	if err != nil {
		return err
	}
	block, err := cl.FetchBlock(seekNewest())
	if err != nil {
		return err
	}
	fmt.Printf("%d\n", block.GetHeader().GetNumber()+1)
	return nil
}

// Block implements `fxadmin ledger block <reference>`, fetching the block named
// by reference ("latest" or a block number) and writing it to outputPath.
func (h *Handler) Block(configPath, currentBlockPath, reference, outputPath string) error {
	logger.Debugf("ledger block %s: config=%s current-block=%s output=%s",
		reference, configPath, currentBlockPath, outputPath)
	seek, err := seekForReference(reference)
	if err != nil {
		return err
	}
	cl, err := h.newOrdererClient(configPath, currentBlockPath)
	if err != nil {
		return err
	}
	block, err := cl.FetchBlock(seek)
	if err != nil {
		return err
	}
	return writeBlock(block, outputPath)
}

// Config implements `fxadmin ledger config <reference>`, fetching the last
// config block and writing it to outputPath. Only the "latest" reference is
// supported.
func (h *Handler) Config(configPath, currentBlockPath, reference, outputPath string) error {
	logger.Debugf("ledger config %s: config=%s current-block=%s output=%s",
		reference, configPath, currentBlockPath, outputPath)
	if reference != latestReference {
		return errors.Newf("unsupported config reference %q: only %q is supported", reference, latestReference)
	}
	cl, err := h.newOrdererClient(configPath, currentBlockPath)
	if err != nil {
		return err
	}

	lastBlock, err := cl.FetchBlock(seekNewest())
	if err != nil {
		return err
	}
	lastConfigIndex, err := protoutil.GetLastConfigIndexFromBlock(lastBlock)
	if err != nil {
		return errors.Wrapf(err, "failed to read last config index from block %d", lastBlock.GetHeader().GetNumber())
	}

	configBlock := lastBlock
	if lastBlock.GetHeader().GetNumber() != lastConfigIndex {
		configBlock, err = cl.FetchBlock(seekByNumber(lastConfigIndex))
		if err != nil {
			return err
		}
	}
	return writeBlock(configBlock, outputPath)
}

// newOrdererClient loads the user configuration and the current config block, then
// assembles an orderer client for the network the block describes.
func (h *Handler) newOrdererClient(configPath, currentBlockPath string) (*client.Client, error) {
	config, err := user.LoadConfig(configPath)
	if err != nil {
		return nil, err
	}
	block, err := readBlock(currentBlockPath)
	if err != nil {
		return nil, err
	}
	return client.Load(config, block, h.csp)
}

// readBlock reads and unmarshals a protobuf block from path.
func readBlock(path string) (*cb.Block, error) {
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

// writeBlock marshals block and writes it to path.
func writeBlock(block *cb.Block, path string) error {
	content, err := proto.Marshal(block)
	if err != nil {
		return errors.Wrap(err, "failed to marshal block")
	}
	if err := os.WriteFile(path, content, 0o600); err != nil {
		return errors.Wrapf(err, "failed to write block to %q", path)
	}
	fmt.Printf("block %d written to %s\n", block.GetHeader().GetNumber(), path)
	return nil
}
