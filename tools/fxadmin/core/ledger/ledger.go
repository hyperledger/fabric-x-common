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

	"github.com/cockroachdb/errors"
	"github.com/hyperledger/fabric-lib-go/bccsp"
	"github.com/hyperledger/fabric-lib-go/bccsp/factory"
	"github.com/hyperledger/fabric-lib-go/common/flogging"

	"github.com/hyperledger/fabric-x-common/protoutil"
	"github.com/hyperledger/fabric-x-common/tools/fxadmin/core/client"
	"github.com/hyperledger/fabric-x-common/tools/fxadmin/core/seek"
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
	block, err := cl.FetchBlock(seek.Newest())
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
	seekInfo, err := seek.ForReference(reference)
	if err != nil {
		return err
	}
	cl, err := h.newOrdererClient(configPath, currentBlockPath)
	if err != nil {
		return err
	}
	block, err := cl.FetchBlock(seekInfo)
	if err != nil {
		return err
	}
	if err := client.WriteBlock(block, outputPath); err != nil {
		return err
	}
	fmt.Printf("block %d written to %s\n", block.GetHeader().GetNumber(), outputPath)
	return nil
}

// Config implements `fxadmin ledger config <reference>`, fetching the last
// config block and writing it to outputPath. Only the "latest" reference is
// supported.
func (h *Handler) Config(configPath, currentBlockPath, reference, outputPath string) error {
	logger.Debugf("ledger config %s: config=%s current-block=%s output=%s",
		reference, configPath, currentBlockPath, outputPath)
	if reference != seek.LatestReference {
		return errors.Newf("unsupported config reference %q: only %q is supported", reference, seek.LatestReference)
	}
	cl, err := h.newOrdererClient(configPath, currentBlockPath)
	if err != nil {
		return err
	}

	lastBlock, err := cl.FetchBlock(seek.Newest())
	if err != nil {
		return err
	}
	lastConfigIndex, err := protoutil.GetLastConfigIndexFromBlock(lastBlock)
	if err != nil {
		return errors.Wrapf(err, "failed to read last config index from block %d", lastBlock.GetHeader().GetNumber())
	}

	configBlock := lastBlock
	if lastBlock.GetHeader().GetNumber() != lastConfigIndex {
		configBlock, err = cl.FetchBlock(seek.ByNumber(lastConfigIndex))
		if err != nil {
			return err
		}
	}
	if err := client.WriteBlock(configBlock, outputPath); err != nil {
		return err
	}
	fmt.Printf("block %d written to %s\n", configBlock.GetHeader().GetNumber(), outputPath)
	return nil
}

// newOrdererClient loads the user configuration and the current config block, then
// assembles an orderer client for the network the block describes.
func (h *Handler) newOrdererClient(configPath, currentBlockPath string) (*client.Client, error) {
	return client.LoadFromFiles(configPath, currentBlockPath, h.csp)
}
