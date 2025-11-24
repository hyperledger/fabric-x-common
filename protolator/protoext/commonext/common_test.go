/*
Copyright IBM Corp. All Rights Reserved.

SPDX-License-Identifier: Apache-2.0
*/

package commonext

import (
	"testing"

	"github.com/hyperledger/fabric-x-common/api/protocommon"
	. "github.com/onsi/gomega"
	"google.golang.org/protobuf/proto"
)

func TestCommonProtolator(t *testing.T) {
	gt := NewGomegaWithT(t)

	// Envelope
	env := &Envelope{Envelope: &protocommon.Envelope{}}
	gt.Expect(env.StaticallyOpaqueFields()).To(Equal([]string{"payload"}))
	msg, err := env.StaticallyOpaqueFieldProto("badproto")
	gt.Expect(msg).To(BeNil())
	gt.Expect(err).To(MatchError("not a marshaled field: badproto"))
	msg, err = env.StaticallyOpaqueFieldProto("payload")
	gt.Expect(err).NotTo(HaveOccurred())
	gt.Expect(msg).To(Equal(&protocommon.Payload{}))

	// Payload
	payload := &Payload{Payload: &protocommon.Payload{}}
	gt.Expect(payload.VariablyOpaqueFields()).To(Equal([]string{"data"}))
	msg, err = payload.VariablyOpaqueFieldProto("badproto")
	gt.Expect(msg).To(BeNil())
	gt.Expect(err).To(MatchError("not a marshaled field: badproto"))
	msg, err = payload.VariablyOpaqueFieldProto("data")
	gt.Expect(msg).To(BeNil())
	gt.Expect(err).To(MatchError("cannot determine payload type when header is missing"))

	payload = &Payload{
		Payload: &protocommon.Payload{
			Header: &protocommon.Header{
				ChannelHeader: []byte("badbytes"),
			},
		},
	}
	msg, err = payload.VariablyOpaqueFieldProto("data")
	gt.Expect(msg).To(BeNil())
	gt.Expect(err.Error()).To(ContainSubstring("corrupt channel header: proto:"))
	gt.Expect(err.Error()).To(ContainSubstring("cannot parse invalid wire-format data"))

	ch := &protocommon.ChannelHeader{
		Type: int32(protocommon.HeaderType_CONFIG),
	}
	chbytes, _ := proto.Marshal(ch)
	payload = &Payload{
		Payload: &protocommon.Payload{
			Header: &protocommon.Header{
				ChannelHeader: chbytes,
			},
		},
	}
	msg, err = payload.VariablyOpaqueFieldProto("data")
	gt.Expect(msg).To(Equal(&protocommon.ConfigEnvelope{}))
	gt.Expect(err).NotTo(HaveOccurred())

	ch = &protocommon.ChannelHeader{
		Type: int32(protocommon.HeaderType_CONFIG_UPDATE),
	}
	chbytes, _ = proto.Marshal(ch)
	payload = &Payload{
		Payload: &protocommon.Payload{
			Header: &protocommon.Header{
				ChannelHeader: chbytes,
			},
		},
	}
	msg, err = payload.VariablyOpaqueFieldProto("data")
	gt.Expect(msg).To(Equal(&protocommon.ConfigUpdateEnvelope{}))
	gt.Expect(err).NotTo(HaveOccurred())

	ch = &protocommon.ChannelHeader{
		Type: int32(protocommon.HeaderType_CHAINCODE_PACKAGE),
	}
	chbytes, _ = proto.Marshal(ch)
	payload = &Payload{
		Payload: &protocommon.Payload{
			Header: &protocommon.Header{
				ChannelHeader: chbytes,
			},
		},
	}
	msg, err = payload.VariablyOpaqueFieldProto("data")
	gt.Expect(msg).To(BeNil())
	gt.Expect(err).To(MatchError("decoding type 6 is unimplemented"))

	// Header
	var header *Header
	gt.Expect(header.StaticallyOpaqueFields()).To(Equal(
		[]string{"channel_header", "signature_header"}))

	msg, err = header.StaticallyOpaqueFieldProto("badproto")
	gt.Expect(msg).To(BeNil())
	gt.Expect(err).To(MatchError("unknown header field: badproto"))

	msg, err = header.StaticallyOpaqueFieldProto("channel_header")
	gt.Expect(msg).To(Equal(&protocommon.ChannelHeader{}))
	gt.Expect(err).NotTo(HaveOccurred())

	msg, err = header.StaticallyOpaqueFieldProto("signature_header")
	gt.Expect(msg).To(Equal(&protocommon.SignatureHeader{}))
	gt.Expect(err).NotTo(HaveOccurred())

	// BlockData
	var bd *BlockData
	gt.Expect(bd.StaticallyOpaqueSliceFields()).To(Equal([]string{"data"}))

	msg, err = bd.StaticallyOpaqueSliceFieldProto("badslice", 0)
	gt.Expect(msg).To(BeNil())
	gt.Expect(err).To(MatchError("not an opaque slice field: badslice"))
	msg, err = bd.StaticallyOpaqueSliceFieldProto("data", 0)
	gt.Expect(msg).To(Equal(&protocommon.Envelope{}))
	gt.Expect(err).NotTo(HaveOccurred())
}
