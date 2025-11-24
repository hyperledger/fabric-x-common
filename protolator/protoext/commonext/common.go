/*
Copyright IBM Corp. All Rights Reserved.

SPDX-License-Identifier: Apache-2.0
*/

package commonext

import (
	"fmt"

	"github.com/hyperledger/fabric-x-common/api/protocommon"
	"github.com/hyperledger/fabric-x-common/api/protomsp"
	"github.com/hyperledger/fabric-x-common/api/protopeer"
	"google.golang.org/protobuf/proto"
)

type Envelope struct{ *protocommon.Envelope }

func (e *Envelope) Underlying() proto.Message {
	return e.Envelope
}

func (e *Envelope) StaticallyOpaqueFields() []string {
	return []string{"payload"}
}

func (e *Envelope) StaticallyOpaqueFieldProto(name string) (proto.Message, error) {
	if name != e.StaticallyOpaqueFields()[0] {
		return nil, fmt.Errorf("not a marshaled field: %s", name)
	}
	return &protocommon.Payload{}, nil
}

type Payload struct{ *protocommon.Payload }

func (p *Payload) Underlying() proto.Message {
	return p.Payload
}

func (p *Payload) VariablyOpaqueFields() []string {
	return []string{"data"}
}

func (p *Payload) VariablyOpaqueFieldProto(name string) (proto.Message, error) {
	if name != p.VariablyOpaqueFields()[0] {
		return nil, fmt.Errorf("not a marshaled field: %s", name)
	}
	if p.Header == nil {
		return nil, fmt.Errorf("cannot determine payload type when header is missing")
	}
	ch := &protocommon.ChannelHeader{}
	if err := proto.Unmarshal(p.Header.ChannelHeader, ch); err != nil {
		return nil, fmt.Errorf("corrupt channel header: %s", err)
	}

	switch ch.Type {
	case int32(protocommon.HeaderType_CONFIG):
		return &protocommon.ConfigEnvelope{}, nil
	case int32(protocommon.HeaderType_ORDERER_TRANSACTION):
		return &protocommon.Envelope{}, nil
	case int32(protocommon.HeaderType_CONFIG_UPDATE):
		return &protocommon.ConfigUpdateEnvelope{}, nil
	case int32(protocommon.HeaderType_MESSAGE):
		// Only used by broadcast_msg sample client
		return &protocommon.ConfigValue{}, nil
	case int32(protocommon.HeaderType_ENDORSER_TRANSACTION):
		return &protopeer.Transaction{}, nil
	default:
		return nil, fmt.Errorf("decoding type %v is unimplemented", ch.Type)
	}
}

type ChannelHeader struct{ *protocommon.ChannelHeader }

func (ch *ChannelHeader) Underlying() proto.Message {
	return ch.ChannelHeader
}

func (ch *ChannelHeader) VariablyOpaqueFields() []string {
	return []string{"extension"}
}

func (ch *ChannelHeader) VariablyOpaqueFieldProto(name string) (proto.Message, error) {
	if name != "extension" {
		return nil, fmt.Errorf("not an opaque field")
	}

	switch ch.Type {
	case int32(protocommon.HeaderType_ENDORSER_TRANSACTION):
		return &protopeer.ChaincodeHeaderExtension{}, nil
	default:
		return nil, fmt.Errorf("channel header extension only valid for endorser transactions")
	}
}

type Header struct{ *protocommon.Header }

func (h *Header) Underlying() proto.Message {
	return h.Header
}

func (h *Header) StaticallyOpaqueFields() []string {
	return []string{"channel_header", "signature_header"}
}

func (h *Header) StaticallyOpaqueFieldProto(name string) (proto.Message, error) {
	switch name {
	case h.StaticallyOpaqueFields()[0]: // channel_header
		return &protocommon.ChannelHeader{}, nil
	case h.StaticallyOpaqueFields()[1]: // signature_header
		return &protocommon.SignatureHeader{}, nil
	default:
		return nil, fmt.Errorf("unknown header field: %s", name)
	}
}

type SignatureHeader struct{ *protocommon.SignatureHeader }

func (sh *SignatureHeader) Underlying() proto.Message {
	return sh.SignatureHeader
}

func (sh *SignatureHeader) StaticallyOpaqueFields() []string {
	return []string{"creator"}
}

func (sh *SignatureHeader) StaticallyOpaqueFieldProto(name string) (proto.Message, error) {
	switch name {
	case sh.StaticallyOpaqueFields()[0]: // creator
		return &protomsp.SerializedIdentity{}, nil
	default:
		return nil, fmt.Errorf("unknown header field: %s", name)
	}
}

type BlockData struct{ *protocommon.BlockData }

func (bd *BlockData) Underlying() proto.Message {
	return bd.BlockData
}

func (bd *BlockData) StaticallyOpaqueSliceFields() []string {
	return []string{"data"}
}

func (bd *BlockData) StaticallyOpaqueSliceFieldProto(fieldName string, index int) (proto.Message, error) {
	if fieldName != bd.StaticallyOpaqueSliceFields()[0] {
		return nil, fmt.Errorf("not an opaque slice field: %s", fieldName)
	}

	return &protocommon.Envelope{}, nil
}
