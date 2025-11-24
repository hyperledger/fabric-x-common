/*
Copyright IBM Corp. All Rights Reserved.

SPDX-License-Identifier: Apache-2.0
*/

package protoutil

import (
	"github.com/hyperledger/fabric-protos-go-apiv2/ledger/rwset"
	"github.com/hyperledger/fabric-protos-go-apiv2/ledger/rwset/kvrwset"
	"github.com/hyperledger/fabric-x-common/api/protocommon"
	"github.com/hyperledger/fabric-x-common/api/protomsp"
	"github.com/hyperledger/fabric-x-common/api/protopeer"
	"github.com/pkg/errors"
	"google.golang.org/protobuf/proto"
)

// the implicit contract of all these unmarshalers is that they
// will return a non-nil pointer whenever the error is nil

// UnmarshalBlock unmarshals bytes to a Block
func UnmarshalBlock(encoded []byte) (*protocommon.Block, error) {
	block := &protocommon.Block{}
	err := proto.Unmarshal(encoded, block)
	return block, errors.Wrap(err, "error unmarshalling Block")
}

// UnmarshalChaincodeDeploymentSpec unmarshals bytes to a ChaincodeDeploymentSpec
func UnmarshalChaincodeDeploymentSpec(code []byte) (*protopeer.ChaincodeDeploymentSpec, error) {
	cds := &protopeer.ChaincodeDeploymentSpec{}
	err := proto.Unmarshal(code, cds)
	return cds, errors.Wrap(err, "error unmarshalling ChaincodeDeploymentSpec")
}

// UnmarshalChaincodeInvocationSpec unmarshals bytes to a ChaincodeInvocationSpec
func UnmarshalChaincodeInvocationSpec(encoded []byte) (*protopeer.ChaincodeInvocationSpec, error) {
	cis := &protopeer.ChaincodeInvocationSpec{}
	err := proto.Unmarshal(encoded, cis)
	return cis, errors.Wrap(err, "error unmarshalling ChaincodeInvocationSpec")
}

// UnmarshalPayload unmarshals bytes to a Payload
func UnmarshalPayload(encoded []byte) (*protocommon.Payload, error) {
	payload := &protocommon.Payload{}
	err := proto.Unmarshal(encoded, payload)
	return payload, errors.Wrap(err, "error unmarshalling Payload")
}

// UnmarshalEnvelope unmarshals bytes to a Envelope
func UnmarshalEnvelope(encoded []byte) (*protocommon.Envelope, error) {
	envelope := &protocommon.Envelope{}
	err := proto.Unmarshal(encoded, envelope)
	return envelope, errors.Wrap(err, "error unmarshalling Envelope")
}

// UnmarshalChannelHeader unmarshals bytes to a ChannelHeader
func UnmarshalChannelHeader(bytes []byte) (*protocommon.ChannelHeader, error) {
	chdr := &protocommon.ChannelHeader{}
	err := proto.Unmarshal(bytes, chdr)
	return chdr, errors.Wrap(err, "error unmarshalling ChannelHeader")
}

// UnmarshalChaincodeID unmarshals bytes to a ChaincodeID
func UnmarshalChaincodeID(bytes []byte) (*protopeer.ChaincodeID, error) {
	ccid := &protopeer.ChaincodeID{}
	err := proto.Unmarshal(bytes, ccid)
	return ccid, errors.Wrap(err, "error unmarshalling ChaincodeID")
}

// UnmarshalSignatureHeader unmarshals bytes to a SignatureHeader
func UnmarshalSignatureHeader(bytes []byte) (*protocommon.SignatureHeader, error) {
	sh := &protocommon.SignatureHeader{}
	err := proto.Unmarshal(bytes, sh)
	return sh, errors.Wrap(err, "error unmarshalling SignatureHeader")
}

// UnmarshalIdentifierHeader unmarshals bytes to an IdentifierHeader
func UnmarshalIdentifierHeader(bytes []byte) (*protocommon.IdentifierHeader, error) {
	ih := &protocommon.IdentifierHeader{}
	err := proto.Unmarshal(bytes, ih)
	return ih, errors.Wrap(err, "error unmarshalling IdentifierHeader")
}

func UnmarshalSerializedIdentity(bytes []byte) (*protomsp.SerializedIdentity, error) {
	sid := &protomsp.SerializedIdentity{}
	err := proto.Unmarshal(bytes, sid)
	return sid, errors.Wrap(err, "error unmarshalling SerializedIdentity")
}

// UnmarshalHeader unmarshals bytes to a Header
func UnmarshalHeader(bytes []byte) (*protocommon.Header, error) {
	hdr := &protocommon.Header{}
	err := proto.Unmarshal(bytes, hdr)
	return hdr, errors.Wrap(err, "error unmarshalling Header")
}

// UnmarshalConfigEnvelope unmarshals bytes to a ConfigEnvelope
func UnmarshalConfigEnvelope(bytes []byte) (*protocommon.ConfigEnvelope, error) {
	cfg := &protocommon.ConfigEnvelope{}
	err := proto.Unmarshal(bytes, cfg)
	return cfg, errors.Wrap(err, "error unmarshalling ConfigEnvelope")
}

// UnmarshalChaincodeHeaderExtension unmarshals bytes to a ChaincodeHeaderExtension
func UnmarshalChaincodeHeaderExtension(hdrExtension []byte) (*protopeer.ChaincodeHeaderExtension, error) {
	chaincodeHdrExt := &protopeer.ChaincodeHeaderExtension{}
	err := proto.Unmarshal(hdrExtension, chaincodeHdrExt)
	return chaincodeHdrExt, errors.Wrap(err, "error unmarshalling ChaincodeHeaderExtension")
}

// UnmarshalProposalResponse unmarshals bytes to a ProposalResponse
func UnmarshalProposalResponse(prBytes []byte) (*protopeer.ProposalResponse, error) {
	proposalResponse := &protopeer.ProposalResponse{}
	err := proto.Unmarshal(prBytes, proposalResponse)
	return proposalResponse, errors.Wrap(err, "error unmarshalling ProposalResponse")
}

// UnmarshalChaincodeAction unmarshals bytes to a ChaincodeAction
func UnmarshalChaincodeAction(caBytes []byte) (*protopeer.ChaincodeAction, error) {
	chaincodeAction := &protopeer.ChaincodeAction{}
	err := proto.Unmarshal(caBytes, chaincodeAction)
	return chaincodeAction, errors.Wrap(err, "error unmarshalling ChaincodeAction")
}

// UnmarshalResponse unmarshals bytes to a Response
func UnmarshalResponse(resBytes []byte) (*protopeer.Response, error) {
	response := &protopeer.Response{}
	err := proto.Unmarshal(resBytes, response)
	return response, errors.Wrap(err, "error unmarshalling Response")
}

// UnmarshalChaincodeEvents unmarshals bytes to a ChaincodeEvent
func UnmarshalChaincodeEvents(eBytes []byte) (*protopeer.ChaincodeEvent, error) {
	chaincodeEvent := &protopeer.ChaincodeEvent{}
	err := proto.Unmarshal(eBytes, chaincodeEvent)
	return chaincodeEvent, errors.Wrap(err, "error unmarshalling ChaicnodeEvent")
}

// UnmarshalProposalResponsePayload unmarshals bytes to a ProposalResponsePayload
func UnmarshalProposalResponsePayload(prpBytes []byte) (*protopeer.ProposalResponsePayload, error) {
	prp := &protopeer.ProposalResponsePayload{}
	err := proto.Unmarshal(prpBytes, prp)
	return prp, errors.Wrap(err, "error unmarshalling ProposalResponsePayload")
}

// UnmarshalProposal unmarshals bytes to a Proposal
func UnmarshalProposal(propBytes []byte) (*protopeer.Proposal, error) {
	prop := &protopeer.Proposal{}
	err := proto.Unmarshal(propBytes, prop)
	return prop, errors.Wrap(err, "error unmarshalling Proposal")
}

// UnmarshalTransaction unmarshals bytes to a Transaction
func UnmarshalTransaction(txBytes []byte) (*protopeer.Transaction, error) {
	tx := &protopeer.Transaction{}
	err := proto.Unmarshal(txBytes, tx)
	return tx, errors.Wrap(err, "error unmarshalling Transaction")
}

// UnmarshalChaincodeActionPayload unmarshals bytes to a ChaincodeActionPayload
func UnmarshalChaincodeActionPayload(capBytes []byte) (*protopeer.ChaincodeActionPayload, error) {
	cap := &protopeer.ChaincodeActionPayload{}
	err := proto.Unmarshal(capBytes, cap)
	return cap, errors.Wrap(err, "error unmarshalling ChaincodeActionPayload")
}

// UnmarshalChaincodeProposalPayload unmarshals bytes to a ChaincodeProposalPayload
func UnmarshalChaincodeProposalPayload(bytes []byte) (*protopeer.ChaincodeProposalPayload, error) {
	cpp := &protopeer.ChaincodeProposalPayload{}
	err := proto.Unmarshal(bytes, cpp)
	return cpp, errors.Wrap(err, "error unmarshalling ChaincodeProposalPayload")
}

// UnmarshalTxReadWriteSet unmarshals bytes to a TxReadWriteSet
func UnmarshalTxReadWriteSet(bytes []byte) (*rwset.TxReadWriteSet, error) {
	rws := &rwset.TxReadWriteSet{}
	err := proto.Unmarshal(bytes, rws)
	return rws, errors.Wrap(err, "error unmarshalling TxReadWriteSet")
}

// UnmarshalKVRWSet unmarshals bytes to a KVRWSet
func UnmarshalKVRWSet(bytes []byte) (*kvrwset.KVRWSet, error) {
	rws := &kvrwset.KVRWSet{}
	err := proto.Unmarshal(bytes, rws)
	return rws, errors.Wrap(err, "error unmarshalling KVRWSet")
}

// UnmarshalHashedRWSet unmarshals bytes to a HashedRWSet
func UnmarshalHashedRWSet(bytes []byte) (*kvrwset.HashedRWSet, error) {
	hrws := &kvrwset.HashedRWSet{}
	err := proto.Unmarshal(bytes, hrws)
	return hrws, errors.Wrap(err, "error unmarshalling HashedRWSet")
}

// UnmarshalSignaturePolicy unmarshals bytes to a SignaturePolicyEnvelope
func UnmarshalSignaturePolicy(bytes []byte) (*protocommon.SignaturePolicyEnvelope, error) {
	sp := &protocommon.SignaturePolicyEnvelope{}
	err := proto.Unmarshal(bytes, sp)
	return sp, errors.Wrap(err, "error unmarshalling SignaturePolicyEnvelope")
}

// UnmarshalPayloadOrPanic unmarshals bytes to a Payload structure or panics
// on error
func UnmarshalPayloadOrPanic(encoded []byte) *protocommon.Payload {
	payload, err := UnmarshalPayload(encoded)
	if err != nil {
		panic(err)
	}
	return payload
}

// UnmarshalEnvelopeOrPanic unmarshals bytes to an Envelope structure or panics
// on error
func UnmarshalEnvelopeOrPanic(encoded []byte) *protocommon.Envelope {
	envelope, err := UnmarshalEnvelope(encoded)
	if err != nil {
		panic(err)
	}
	return envelope
}

// UnmarshalBlockOrPanic unmarshals bytes to an Block or panics
// on error
func UnmarshalBlockOrPanic(encoded []byte) *protocommon.Block {
	block, err := UnmarshalBlock(encoded)
	if err != nil {
		panic(err)
	}
	return block
}

// UnmarshalChannelHeaderOrPanic unmarshals bytes to a ChannelHeader or panics
// on error
func UnmarshalChannelHeaderOrPanic(bytes []byte) *protocommon.ChannelHeader {
	chdr, err := UnmarshalChannelHeader(bytes)
	if err != nil {
		panic(err)
	}
	return chdr
}

// UnmarshalSignatureHeaderOrPanic unmarshals bytes to a SignatureHeader or panics
// on error
func UnmarshalSignatureHeaderOrPanic(bytes []byte) *protocommon.SignatureHeader {
	sighdr, err := UnmarshalSignatureHeader(bytes)
	if err != nil {
		panic(err)
	}
	return sighdr
}
