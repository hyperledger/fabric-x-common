/*
Copyright IBM Corp. All Rights Reserved.

SPDX-License-Identifier: Apache-2.0
*/

package helpers

import (
	"github.com/cockroachdb/errors"
	cb "github.com/hyperledger/fabric-protos-go-apiv2/common"
	"google.golang.org/protobuf/proto"

	"github.com/hyperledger/fabric-x-common/protoutil"
)

// ConfigBlock is a config block decoded down to the common.Config it carries.
// It keeps every layer that wraps the config, so that an edited config can be
// encoded back into the same block with Marshal.
type ConfigBlock struct {
	Block          *cb.Block
	Envelope       *cb.Envelope
	Payload        *cb.Payload
	ChannelHeader  *cb.ChannelHeader
	ConfigEnvelope *cb.ConfigEnvelope
}

// UnmarshalConfigBlock unmarshals a marshaled common.Block and decodes the
// configuration transaction in its first envelope. It errors if the block is
// not a config block or carries no config.
func UnmarshalConfigBlock(marshaledBlock []byte) (*ConfigBlock, error) {
	block := &cb.Block{}
	if err := proto.Unmarshal(marshaledBlock, block); err != nil {
		return nil, errors.Wrap(err, "failed to unmarshal block input")
	}

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
// encoded into the block by Marshal.
func (b *ConfigBlock) Config() *cb.Config {
	return b.ConfigEnvelope.GetConfig()
}

// deterministic marshals with a fixed order of map entries. The config's groups,
// values, and policies are maps, so a plain marshal can encode the same config
// to different bytes on every run.
var deterministic = proto.MarshalOptions{Deterministic: true}

// Marshal encodes the config back through every enclosing layer, updating each
// layer in place, and returns the marshaled block. The encoding is
// deterministic, so the same config always gives the same bytes. Only the
// block's data changes: its header data hash and the signatures over the
// envelope and the block are left as they were, so an edited block is fit to be
// read back for its config but not to be submitted.
func (b *ConfigBlock) Marshal() ([]byte, error) {
	configEnvelope, err := deterministic.Marshal(b.ConfigEnvelope)
	if err != nil {
		return nil, errors.Wrap(err, "failed to marshal config envelope")
	}
	b.Payload.Data = configEnvelope

	payload, err := deterministic.Marshal(b.Payload)
	if err != nil {
		return nil, errors.Wrap(err, "failed to marshal config payload")
	}
	b.Envelope.Payload = payload

	envelope, err := deterministic.Marshal(b.Envelope)
	if err != nil {
		return nil, errors.Wrap(err, "failed to marshal config envelope transaction")
	}
	b.Block.Data.Data[0] = envelope

	block, err := deterministic.Marshal(b.Block)
	if err != nil {
		return nil, errors.Wrap(err, "failed to marshal block")
	}
	return block, nil
}
