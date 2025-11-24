/*
Copyright IBM Corp. All Rights Reserved.

SPDX-License-Identifier: Apache-2.0
*/

package protoutil

import (
	"crypto/sha256"
	"encoding/hex"

	"github.com/hyperledger/fabric-x-common/api/protocommon"
	"github.com/hyperledger/fabric-x-common/api/protopeer"
	"github.com/pkg/errors"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/timestamppb"
)

// CreateChaincodeProposal creates a proposal from given input.
// It returns the proposal and the transaction id associated to the proposal
func CreateChaincodeProposal(typ protocommon.HeaderType, channelID string, cis *protopeer.ChaincodeInvocationSpec, creator []byte) (*protopeer.Proposal, string, error) {
	return CreateChaincodeProposalWithTransient(typ, channelID, cis, creator, nil)
}

// CreateChaincodeProposalWithTransient creates a proposal from given input
// It returns the proposal and the transaction id associated to the proposal
func CreateChaincodeProposalWithTransient(typ protocommon.HeaderType, channelID string, cis *protopeer.ChaincodeInvocationSpec, creator []byte, transientMap map[string][]byte) (*protopeer.Proposal, string, error) {
	// generate a random nonce
	nonce, err := getRandomNonce()
	if err != nil {
		return nil, "", err
	}

	// compute txid
	txid := ComputeTxID(nonce, creator)

	return CreateChaincodeProposalWithTxIDNonceAndTransient(txid, typ, channelID, cis, nonce, creator, transientMap)
}

// CreateChaincodeProposalWithTxIDAndTransient creates a proposal from given
// input. It returns the proposal and the transaction id associated with the
// proposal
func CreateChaincodeProposalWithTxIDAndTransient(typ protocommon.HeaderType, channelID string, cis *protopeer.ChaincodeInvocationSpec, creator []byte, txid string, transientMap map[string][]byte) (*protopeer.Proposal, string, error) {
	// generate a random nonce
	nonce, err := getRandomNonce()
	if err != nil {
		return nil, "", err
	}

	// compute txid unless provided by tests
	if txid == "" {
		txid = ComputeTxID(nonce, creator)
	}

	return CreateChaincodeProposalWithTxIDNonceAndTransient(txid, typ, channelID, cis, nonce, creator, transientMap)
}

// CreateChaincodeProposalWithTxIDNonceAndTransient creates a proposal from
// given input
func CreateChaincodeProposalWithTxIDNonceAndTransient(txid string, typ protocommon.HeaderType, channelID string, cis *protopeer.ChaincodeInvocationSpec, nonce, creator []byte, transientMap map[string][]byte) (*protopeer.Proposal, string, error) {
	ccHdrExt := &protopeer.ChaincodeHeaderExtension{ChaincodeId: cis.ChaincodeSpec.ChaincodeId}
	ccHdrExtBytes, err := proto.Marshal(ccHdrExt)
	if err != nil {
		return nil, "", errors.Wrap(err, "error marshaling ChaincodeHeaderExtension")
	}

	cisBytes, err := proto.Marshal(cis)
	if err != nil {
		return nil, "", errors.Wrap(err, "error marshaling ChaincodeInvocationSpec")
	}

	ccPropPayload := &protopeer.ChaincodeProposalPayload{Input: cisBytes, TransientMap: transientMap}
	ccPropPayloadBytes, err := proto.Marshal(ccPropPayload)
	if err != nil {
		return nil, "", errors.Wrap(err, "error marshaling ChaincodeProposalPayload")
	}

	// TODO: epoch is now set to zero. This must be changed once we
	// get a more appropriate mechanism to handle it in.
	var epoch uint64

	hdr := &protocommon.Header{
		ChannelHeader: MarshalOrPanic(
			&protocommon.ChannelHeader{
				Type:      int32(typ),
				TxId:      txid,
				Timestamp: timestamppb.Now(),
				ChannelId: channelID,
				Extension: ccHdrExtBytes,
				Epoch:     epoch,
			},
		),
		SignatureHeader: MarshalOrPanic(
			&protocommon.SignatureHeader{
				Nonce:   nonce,
				Creator: creator,
			},
		),
	}

	hdrBytes, err := proto.Marshal(hdr)
	if err != nil {
		return nil, "", err
	}

	prop := &protopeer.Proposal{
		Header:  hdrBytes,
		Payload: ccPropPayloadBytes,
	}
	return prop, txid, nil
}

// GetBytesProposalResponsePayload gets proposal response payload
func GetBytesProposalResponsePayload(hash []byte, response *protopeer.Response, result []byte, event []byte, ccid *protopeer.ChaincodeID) ([]byte, error) {
	cAct := &protopeer.ChaincodeAction{
		Events: event, Results: result,
		Response:    response,
		ChaincodeId: ccid,
	}
	cActBytes, err := proto.Marshal(cAct)
	if err != nil {
		return nil, errors.Wrap(err, "error marshaling ChaincodeAction")
	}

	prp := &protopeer.ProposalResponsePayload{
		Extension:    cActBytes,
		ProposalHash: hash,
	}
	prpBytes, err := proto.Marshal(prp)
	return prpBytes, errors.Wrap(err, "error marshaling ProposalResponsePayload")
}

// GetBytesChaincodeProposalPayload gets the chaincode proposal payload
func GetBytesChaincodeProposalPayload(cpp *protopeer.ChaincodeProposalPayload) ([]byte, error) {
	if cpp == nil {
		return nil, errors.New("error marshaling ChaincodeProposalPayload: proto: Marshal called with nil")
	}
	cppBytes, err := proto.Marshal(cpp)
	return cppBytes, errors.Wrap(err, "error marshaling ChaincodeProposalPayload")
}

// GetBytesResponse gets the bytes of Response
func GetBytesResponse(res *protopeer.Response) ([]byte, error) {
	if res == nil {
		return nil, errors.New("error marshaling Response: proto: Marshal called with nil")
	}
	resBytes, err := proto.Marshal(res)
	return resBytes, errors.Wrap(err, "error marshaling Response")
}

// GetBytesChaincodeEvent gets the bytes of ChaincodeEvent
func GetBytesChaincodeEvent(event *protopeer.ChaincodeEvent) ([]byte, error) {
	if event == nil {
		return nil, errors.New("error marshaling ChaincodeEvent: proto: Marshal called with nil")
	}
	eventBytes, err := proto.Marshal(event)
	return eventBytes, errors.Wrap(err, "error marshaling ChaincodeEvent")
}

// GetBytesChaincodeActionPayload get the bytes of ChaincodeActionPayload from
// the message
func GetBytesChaincodeActionPayload(cap *protopeer.ChaincodeActionPayload) ([]byte, error) {
	if cap == nil {
		return nil, errors.New("error marshaling ChaincodeActionPayload: proto: Marshal called with nil")
	}
	capBytes, err := proto.Marshal(cap)
	return capBytes, errors.Wrap(err, "error marshaling ChaincodeActionPayload")
}

// GetBytesProposalResponse gets proposal bytes response
func GetBytesProposalResponse(pr *protopeer.ProposalResponse) ([]byte, error) {
	if pr == nil {
		return nil, errors.New("error marshaling ProposalResponse: proto: Marshal called with nil")
	}
	respBytes, err := proto.Marshal(pr)
	return respBytes, errors.Wrap(err, "error marshaling ProposalResponse")
}

// GetBytesHeader get the bytes of Header from the message
func GetBytesHeader(hdr *protocommon.Header) ([]byte, error) {
	if hdr == nil {
		return nil, errors.New("error marshaling Header: proto: Marshal called with nil")
	}
	bytes, err := proto.Marshal(hdr)
	return bytes, errors.Wrap(err, "error marshaling Header")
}

// GetBytesSignatureHeader get the bytes of SignatureHeader from the message
func GetBytesSignatureHeader(hdr *protocommon.SignatureHeader) ([]byte, error) {
	if hdr == nil {
		return nil, errors.New("error marshaling SignatureHeader: proto: Marshal called with nil")
	}
	bytes, err := proto.Marshal(hdr)
	return bytes, errors.Wrap(err, "error marshaling SignatureHeader")
}

// GetBytesTransaction get the bytes of Transaction from the message
func GetBytesTransaction(tx *protopeer.Transaction) ([]byte, error) {
	if tx == nil {
		return nil, errors.New("error marshaling Transaction: proto: Marshal called with nil")
	}
	bytes, err := proto.Marshal(tx)
	return bytes, errors.Wrap(err, "error unmarshalling Transaction")
}

// GetBytesPayload get the bytes of Payload from the message
func GetBytesPayload(payl *protocommon.Payload) ([]byte, error) {
	if payl == nil {
		return nil, errors.New("error marshaling Payload: proto: Marshal called with nil")
	}
	bytes, err := proto.Marshal(payl)
	return bytes, errors.Wrap(err, "error marshaling Payload")
}

// GetBytesEnvelope get the bytes of Envelope from the message
func GetBytesEnvelope(env *protocommon.Envelope) ([]byte, error) {
	if env == nil {
		return nil, errors.New("error marshaling Envelope: proto: Marshal called with nil")
	}
	bytes, err := proto.Marshal(env)
	return bytes, errors.Wrap(err, "error marshaling Envelope")
}

// GetActionFromEnvelope extracts a ChaincodeAction message from a
// serialized Envelope
// TODO: fix function name as per FAB-11831
func GetActionFromEnvelope(envBytes []byte) (*protopeer.ChaincodeAction, error) {
	env, err := GetEnvelopeFromBlock(envBytes)
	if err != nil {
		return nil, err
	}
	return GetActionFromEnvelopeMsg(env)
}

func GetActionFromEnvelopeMsg(env *protocommon.Envelope) (*protopeer.ChaincodeAction, error) {
	payl, err := UnmarshalPayload(env.Payload)
	if err != nil {
		return nil, err
	}

	tx, err := UnmarshalTransaction(payl.Data)
	if err != nil {
		return nil, err
	}

	if len(tx.Actions) == 0 {
		return nil, errors.New("at least one TransactionAction required")
	}

	_, respPayload, err := GetPayloads(tx.Actions[0])
	return respPayload, err
}

// CreateProposalFromCISAndTxid returns a proposal given a serialized identity
// and a ChaincodeInvocationSpec
func CreateProposalFromCISAndTxid(txid string, typ protocommon.HeaderType, channelID string, cis *protopeer.ChaincodeInvocationSpec, creator []byte) (*protopeer.Proposal, string, error) {
	nonce, err := getRandomNonce()
	if err != nil {
		return nil, "", err
	}
	return CreateChaincodeProposalWithTxIDNonceAndTransient(txid, typ, channelID, cis, nonce, creator, nil)
}

// CreateProposalFromCIS returns a proposal given a serialized identity and a
// ChaincodeInvocationSpec
func CreateProposalFromCIS(typ protocommon.HeaderType, channelID string, cis *protopeer.ChaincodeInvocationSpec, creator []byte) (*protopeer.Proposal, string, error) {
	return CreateChaincodeProposal(typ, channelID, cis, creator)
}

// CreateGetChaincodesProposal returns a GETCHAINCODES proposal given a
// serialized identity
func CreateGetChaincodesProposal(channelID string, creator []byte) (*protopeer.Proposal, string, error) {
	ccinp := &protopeer.ChaincodeInput{Args: [][]byte{[]byte("getchaincodes")}}
	lsccSpec := &protopeer.ChaincodeInvocationSpec{
		ChaincodeSpec: &protopeer.ChaincodeSpec{
			Type:        protopeer.ChaincodeSpec_GOLANG,
			ChaincodeId: &protopeer.ChaincodeID{Name: "lscc"},
			Input:       ccinp,
		},
	}
	return CreateProposalFromCIS(protocommon.HeaderType_ENDORSER_TRANSACTION, channelID, lsccSpec, creator)
}

// CreateGetInstalledChaincodesProposal returns a GETINSTALLEDCHAINCODES
// proposal given a serialized identity
func CreateGetInstalledChaincodesProposal(creator []byte) (*protopeer.Proposal, string, error) {
	ccinp := &protopeer.ChaincodeInput{Args: [][]byte{[]byte("getinstalledchaincodes")}}
	lsccSpec := &protopeer.ChaincodeInvocationSpec{
		ChaincodeSpec: &protopeer.ChaincodeSpec{
			Type:        protopeer.ChaincodeSpec_GOLANG,
			ChaincodeId: &protopeer.ChaincodeID{Name: "lscc"},
			Input:       ccinp,
		},
	}
	return CreateProposalFromCIS(protocommon.HeaderType_ENDORSER_TRANSACTION, "", lsccSpec, creator)
}

// CreateInstallProposalFromCDS returns a install proposal given a serialized
// identity and a ChaincodeDeploymentSpec
func CreateInstallProposalFromCDS(ccpack proto.Message, creator []byte) (*protopeer.Proposal, string, error) {
	return createProposalFromCDS("", ccpack, creator, "install")
}

// CreateDeployProposalFromCDS returns a deploy proposal given a serialized
// identity and a ChaincodeDeploymentSpec
func CreateDeployProposalFromCDS(
	channelID string,
	cds *protopeer.ChaincodeDeploymentSpec,
	creator []byte,
	policy []byte,
	escc []byte,
	vscc []byte,
	collectionConfig []byte,
) (*protopeer.Proposal, string, error) {
	if collectionConfig == nil {
		return createProposalFromCDS(channelID, cds, creator, "deploy", policy, escc, vscc)
	}
	return createProposalFromCDS(channelID, cds, creator, "deploy", policy, escc, vscc, collectionConfig)
}

// CreateUpgradeProposalFromCDS returns a upgrade proposal given a serialized
// identity and a ChaincodeDeploymentSpec
func CreateUpgradeProposalFromCDS(
	channelID string,
	cds *protopeer.ChaincodeDeploymentSpec,
	creator []byte,
	policy []byte,
	escc []byte,
	vscc []byte,
	collectionConfig []byte,
) (*protopeer.Proposal, string, error) {
	if collectionConfig == nil {
		return createProposalFromCDS(channelID, cds, creator, "upgrade", policy, escc, vscc)
	}
	return createProposalFromCDS(channelID, cds, creator, "upgrade", policy, escc, vscc, collectionConfig)
}

// createProposalFromCDS returns a deploy or upgrade proposal given a
// serialized identity and a ChaincodeDeploymentSpec
func createProposalFromCDS(channelID string, msg proto.Message, creator []byte, propType string, args ...[]byte) (*protopeer.Proposal, string, error) {
	// in the new mode, cds will be nil, "deploy" and "upgrade" are instantiates.
	var ccinp *protopeer.ChaincodeInput
	var b []byte
	var err error
	if msg != nil {
		if !msg.ProtoReflect().IsValid() {
			return nil, "", errors.New("proto: Marshal called with nil")
		}
		b, err = proto.Marshal(msg)
		if err != nil {
			return nil, "", err
		}
	}
	switch propType {
	case "deploy":
		fallthrough
	case "upgrade":
		cds, ok := msg.(*protopeer.ChaincodeDeploymentSpec)
		if !ok || cds == nil {
			return nil, "", errors.New("invalid message for creating lifecycle chaincode proposal")
		}
		Args := [][]byte{[]byte(propType), []byte(channelID), b}
		Args = append(Args, args...)

		ccinp = &protopeer.ChaincodeInput{Args: Args}
	case "install":
		ccinp = &protopeer.ChaincodeInput{Args: [][]byte{[]byte(propType), b}}
	}

	// wrap the deployment in an invocation spec to lscc...
	lsccSpec := &protopeer.ChaincodeInvocationSpec{
		ChaincodeSpec: &protopeer.ChaincodeSpec{
			Type:        protopeer.ChaincodeSpec_GOLANG,
			ChaincodeId: &protopeer.ChaincodeID{Name: "lscc"},
			Input:       ccinp,
		},
	}

	// ...and get the proposal for it
	return CreateProposalFromCIS(protocommon.HeaderType_ENDORSER_TRANSACTION, channelID, lsccSpec, creator)
}

// ComputeTxID computes TxID as the Hash computed
// over the concatenation of nonce and creator.
func ComputeTxID(nonce, creator []byte) string {
	// TODO: Get the Hash function to be used from
	// channel configuration
	hasher := sha256.New()
	hasher.Write(nonce)
	hasher.Write(creator)
	return hex.EncodeToString(hasher.Sum(nil))
}

// CheckTxID checks that txid is equal to the Hash computed
// over the concatenation of nonce and creator.
func CheckTxID(txid string, nonce, creator []byte) error {
	computedTxID := ComputeTxID(nonce, creator)

	if txid != computedTxID {
		return errors.Errorf("invalid txid. got [%s], expected [%s]", txid, computedTxID)
	}

	return nil
}

// InvokedChaincodeName takes the proposal bytes of a SignedProposal, and unpacks it all the way down,
// until either an error is encountered, or the chaincode name is found. This is useful primarily
// for chaincodes which wish to know the chaincode name originally invoked, in order to deny cc2cc
// invocations (or, perhaps to deny direct invocations and require cc2cc).
func InvokedChaincodeName(proposalBytes []byte) (string, error) {
	proposal := &protopeer.Proposal{}
	err := proto.Unmarshal(proposalBytes, proposal)
	if err != nil {
		return "", errors.WithMessage(err, "could not unmarshal proposal")
	}

	proposalPayload := &protopeer.ChaincodeProposalPayload{}
	err = proto.Unmarshal(proposal.Payload, proposalPayload)
	if err != nil {
		return "", errors.WithMessage(err, "could not unmarshal chaincode proposal payload")
	}

	cis := &protopeer.ChaincodeInvocationSpec{}
	err = proto.Unmarshal(proposalPayload.Input, cis)
	if err != nil {
		return "", errors.WithMessage(err, "could not unmarshal chaincode invocation spec")
	}

	if cis.ChaincodeSpec == nil {
		return "", errors.Errorf("chaincode spec is nil")
	}

	if cis.ChaincodeSpec.ChaincodeId == nil {
		return "", errors.Errorf("chaincode id is nil")
	}

	return cis.ChaincodeSpec.ChaincodeId.Name, nil
}
