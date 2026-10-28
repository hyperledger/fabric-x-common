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

// ConfigBlock is a config block decoded down to the common.Config it carries.
// It keeps every layer that wraps the config, so that an edited config can be
// encoded back into the same block with Encode.
type ConfigBlock struct {
	Block          *cb.Block
	Envelope       *cb.Envelope
	Payload        *cb.Payload
	ChannelHeader  *cb.ChannelHeader
	ConfigEnvelope *cb.ConfigEnvelope
}

// UnmarshalConfigBlock unmarshals a marshaled common.Block and decodes it with
// DecodeConfigBlock.
func UnmarshalConfigBlock(marshaledBlock []byte) (*ConfigBlock, error) {
	block := &cb.Block{}
	if err := proto.Unmarshal(marshaledBlock, block); err != nil {
		return nil, errors.Wrap(err, "failed to unmarshal block input")
	}
	return DecodeConfigBlock(block)
}

// DecodeConfigBlock decodes the configuration transaction in the block's first
// envelope. It errors if the block is not a config block or carries no config.
func DecodeConfigBlock(block *cb.Block) (*ConfigBlock, error) {
	envelope, err := protoutil.ExtractEnvelope(block, 0)
	if err != nil {
		return nil, errors.Wrap(err, "failed to extract config envelope from block")
	}

	payload, err := protoutil.UnmarshalPayload(envelope.Payload)
	if err != nil {
		return nil, errors.Wrap(err, "failed to unmarshal config payload")
	}

	if payload.GetHeader() == nil {
		return nil, errors.New("block is not a config block: payload has no header")
	}
	channelHeader, err := protoutil.UnmarshalChannelHeader(payload.GetHeader().GetChannelHeader())
	if err != nil {
		return nil, errors.Wrap(err, "failed to unmarshal channel header")
	}
	if channelHeader.GetType() != int32(cb.HeaderType_CONFIG) {
		return nil, errors.Newf("block is not a config block: channel header type is %s, want %s",
			cb.HeaderType(channelHeader.GetType()), cb.HeaderType_CONFIG)
	}

	configEnvelope, err := protoutil.UnmarshalConfigEnvelope(payload.Data)
	if err != nil {
		return nil, errors.Wrap(err, "failed to unmarshal config envelope")
	}

	if configEnvelope.GetConfig() == nil {
		return nil, errors.New("block does not carry a config: config envelope has no config")
	}
	return &ConfigBlock{
		Block:          block,
		Envelope:       envelope,
		Payload:        payload,
		ChannelHeader:  channelHeader,
		ConfigEnvelope: configEnvelope,
	}, nil
}

// Config returns the common.Config the block carries. Changes made to it are
// encoded into the block by Encode.
func (b *ConfigBlock) Config() *cb.Config {
	return b.ConfigEnvelope.GetConfig()
}

// DeterministicMarshal marshals with a fixed order of map entries. The config's
// groups, values, and policies are maps, so a plain marshal can encode the same
// config to different bytes on every run. Every layer that wraps an edited
// config is marshaled with it, up to the block, which has no maps.
var DeterministicMarshal = proto.MarshalOptions{Deterministic: true}

// Encode encodes the config back through every enclosing layer into Block,
// updating each layer in place. The encoding is deterministic, so the same
// config always gives the same block. Only the block's data changes: its header
// data hash and the signatures over the envelope and the block are left as they
// were, so an edited block is fit to be read back for its config but not to be
// submitted.
func (b *ConfigBlock) Encode() error {
	configEnvelope, err := DeterministicMarshal.Marshal(b.ConfigEnvelope)
	if err != nil {
		return errors.Wrap(err, "failed to marshal config envelope")
	}
	b.Payload.Data = configEnvelope

	payload, err := DeterministicMarshal.Marshal(b.Payload)
	if err != nil {
		return errors.Wrap(err, "failed to marshal config payload")
	}
	b.Envelope.Payload = payload

	envelope, err := DeterministicMarshal.Marshal(b.Envelope)
	if err != nil {
		return errors.Wrap(err, "failed to marshal config envelope transaction")
	}
	b.Block.Data.Data[0] = envelope
	return nil
}

// ReadConfigBlock reads and unmarshals a protobuf config block from path. It is
// shared by the commands that take a --current-block file.
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
