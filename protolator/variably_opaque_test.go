/*
Copyright IBM Corp. 2017 All Rights Reserved.

SPDX-License-Identifier: Apache-2.0
*/

package protolator

import (
	"bytes"
	"testing"

	. "github.com/onsi/gomega"
	"google.golang.org/protobuf/proto"

	"github.com/hyperledger/fabric-protos-go-apiv2/orderer"

	"github.com/hyperledger/fabric-x-common/api/armapb"
	"github.com/hyperledger/fabric-x-common/protolator/protoext/ordererext"
	"github.com/hyperledger/fabric-x-common/protolator/testprotos"
)

func extractNestedMsgPlainField(source []byte) string {
	result := &testprotos.NestedMsg{}
	err := proto.Unmarshal(source, result)
	if err != nil {
		panic(err)
	}
	return result.PlainNestedField.PlainField
}

func TestPlainVariablyOpaqueMsg(t *testing.T) {
	gt := NewGomegaWithT(t)

	fromPrefix := "from"
	toPrefix := "to"
	tppff := &testProtoPlainFieldFactory{
		fromPrefix: fromPrefix,
		toPrefix:   toPrefix,
	}

	fieldFactories = []protoFieldFactory{tppff}

	pfValue := "foo"
	startMsg := &testprotos.VariablyOpaqueMsg{
		OpaqueType: "NestedMsg",
		PlainOpaqueField: protoMarshalOrPanic(&testprotos.NestedMsg{
			PlainNestedField: &testprotos.SimpleMsg{
				PlainField: pfValue,
			},
		}),
	}

	var buffer bytes.Buffer
	err := DeepMarshalJSON(&buffer, startMsg)
	gt.Expect(err).NotTo(HaveOccurred())
	newMsg := &testprotos.VariablyOpaqueMsg{}
	err = DeepUnmarshalJSON(bytes.NewReader(buffer.Bytes()), newMsg)
	gt.Expect(err).NotTo(HaveOccurred())
	gt.Expect(extractNestedMsgPlainField(newMsg.PlainOpaqueField)).NotTo(Equal(fromPrefix + toPrefix + extractNestedMsgPlainField(startMsg.PlainOpaqueField)))

	fieldFactories = []protoFieldFactory{tppff, nestedFieldFactory{}, variablyOpaqueFieldFactory{}}

	buffer.Reset()
	err = DeepMarshalJSON(&buffer, startMsg)
	gt.Expect(err).NotTo(HaveOccurred())
	err = DeepUnmarshalJSON(bytes.NewReader(buffer.Bytes()), newMsg)
	gt.Expect(err).NotTo(HaveOccurred())
	gt.Expect(extractNestedMsgPlainField(newMsg.PlainOpaqueField)).To(Equal(fromPrefix + toPrefix + extractNestedMsgPlainField(startMsg.PlainOpaqueField)))
}

func TestMapVariablyOpaqueMsg(t *testing.T) {
	gt := NewGomegaWithT(t)

	fromPrefix := "from"
	toPrefix := "to"
	tppff := &testProtoPlainFieldFactory{
		fromPrefix: fromPrefix,
		toPrefix:   toPrefix,
	}

	fieldFactories = []protoFieldFactory{tppff}

	pfValue := "foo"
	mapKey := "bar"
	startMsg := &testprotos.VariablyOpaqueMsg{
		OpaqueType: "NestedMsg",
		MapOpaqueField: map[string][]byte{
			mapKey: protoMarshalOrPanic(&testprotos.NestedMsg{
				PlainNestedField: &testprotos.SimpleMsg{
					PlainField: pfValue,
				},
			}),
		},
	}

	var buffer bytes.Buffer
	err := DeepMarshalJSON(&buffer, startMsg)
	gt.Expect(err).NotTo(HaveOccurred())
	newMsg := &testprotos.VariablyOpaqueMsg{}
	err = DeepUnmarshalJSON(bytes.NewReader(buffer.Bytes()), newMsg)
	gt.Expect(err).NotTo(HaveOccurred())
	gt.Expect(extractNestedMsgPlainField(newMsg.MapOpaqueField[mapKey])).NotTo(Equal(fromPrefix + toPrefix + extractNestedMsgPlainField(startMsg.MapOpaqueField[mapKey])))

	fieldFactories = []protoFieldFactory{tppff, nestedFieldFactory{}, variablyOpaqueMapFieldFactory{}}

	buffer.Reset()
	err = DeepMarshalJSON(&buffer, startMsg)
	gt.Expect(err).NotTo(HaveOccurred())
	err = DeepUnmarshalJSON(bytes.NewReader(buffer.Bytes()), newMsg)
	gt.Expect(err).NotTo(HaveOccurred())
	gt.Expect(extractNestedMsgPlainField(newMsg.MapOpaqueField[mapKey])).To(Equal(fromPrefix + toPrefix + extractNestedMsgPlainField(startMsg.MapOpaqueField[mapKey])))
}

func TestSliceVariablyOpaqueMsg(t *testing.T) {
	gt := NewGomegaWithT(t)

	fromPrefix := "from"
	toPrefix := "to"
	tppff := &testProtoPlainFieldFactory{
		fromPrefix: fromPrefix,
		toPrefix:   toPrefix,
	}

	fieldFactories = []protoFieldFactory{tppff}

	pfValue := "foo"
	startMsg := &testprotos.VariablyOpaqueMsg{
		OpaqueType: "NestedMsg",
		SliceOpaqueField: [][]byte{
			protoMarshalOrPanic(&testprotos.NestedMsg{
				PlainNestedField: &testprotos.SimpleMsg{
					PlainField: pfValue,
				},
			}),
		},
	}

	var buffer bytes.Buffer
	err := DeepMarshalJSON(&buffer, startMsg)
	gt.Expect(err).NotTo(HaveOccurred())
	newMsg := &testprotos.VariablyOpaqueMsg{}
	err = DeepUnmarshalJSON(bytes.NewReader(buffer.Bytes()), newMsg)
	gt.Expect(err).NotTo(HaveOccurred())
	gt.Expect(extractNestedMsgPlainField(newMsg.SliceOpaqueField[0])).NotTo(Equal(fromPrefix + toPrefix + extractNestedMsgPlainField(startMsg.SliceOpaqueField[0])))

	fieldFactories = []protoFieldFactory{tppff, nestedFieldFactory{}, variablyOpaqueSliceFieldFactory{}}

	buffer.Reset()
	err = DeepMarshalJSON(&buffer, startMsg)
	gt.Expect(err).NotTo(HaveOccurred())
	err = DeepUnmarshalJSON(bytes.NewReader(buffer.Bytes()), newMsg)
	gt.Expect(err).NotTo(HaveOccurred())
	gt.Expect(extractNestedMsgPlainField(newMsg.SliceOpaqueField[0])).To(Equal(fromPrefix + toPrefix + extractNestedMsgPlainField(startMsg.SliceOpaqueField[0])))
}

func TestArmaSharedConfigVariablyOpaqueMsg(t *testing.T) {
	gt := NewGomegaWithT(t)

	ct := &ordererext.ConsensusType{
		ConsensusType: &orderer.ConsensusType{
			Type: "arma",
			Metadata: func() []byte {
				metadataProto := &armapb.SharedConfig{
					BatchingConfig: &armapb.BatchingConfig{
						BatchSize: &armapb.BatchSize{
							MaxMessageCount:   10,
							AbsoluteMaxBytes:  1024,
							PreferredMaxBytes: 512,
						},
					},
					PartiesConfig: []*armapb.PartyConfig{
						{
							PartyID: 1,
							ConsenterConfig: &armapb.ConsenterNodeConfig{
								Host: "localhost",
								Port: 7050,
							},
						},
					},
					ConsensusConfig: &armapb.ConsensusConfig{
						SmartBFTConfig: &armapb.SmartBFTConfig{
							RequestBatchMaxCount: 1000,
						},
					},
				}
				marshaled, err := proto.Marshal(metadataProto)
				if err != nil {
					t.Fatalf("Failed to marshal arma ConsensusTypeMetadata: %s", err)
				}
				return marshaled
			}(),
		}}

	var buffer bytes.Buffer
	err := DeepMarshalJSON(&buffer, ct)
	gt.Expect(err).NotTo(HaveOccurred())
	newCt := &ordererext.ConsensusType{ConsensusType: &orderer.ConsensusType{}}
	err = DeepUnmarshalJSON(bytes.NewReader(buffer.Bytes()), newCt)
	gt.Expect(err).NotTo(HaveOccurred())
	gt.Expect(proto.Equal(ct, newCt)).To(BeTrue())
}
