/*
Copyright IBM Corp. All Rights Reserved.

SPDX-License-Identifier: Apache-2.0
*/

package cauthdsl

import (
	"testing"

	cb "github.com/hyperledger/fabric-protos-go-apiv2/common"
	mb "github.com/hyperledger/fabric-protos-go-apiv2/msp"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/proto"

	"github.com/hyperledger/fabric-x-common/api/msppb"
	"github.com/hyperledger/fabric-x-common/common/policydsl"
	"github.com/hyperledger/fabric-x-common/protoutil"
)

var signers = []*msppb.Identity{
	msppb.NewIdentity("org1", []byte("signer0")),
	msppb.NewIdentity("org1", []byte("signer1")),
}

var signersBytes = [][]byte{
	protoutil.MarshalOrPanic(signers[0]),
	protoutil.MarshalOrPanic(signers[1]),
}

func TestSimpleSignature(t *testing.T) {
	t.Parallel()
	policy := policydsl.Envelope(policydsl.SignedBy(0), signersBytes)

	spe, err := compile(policy.Rule, policy.Identities)
	require.NoError(t, err, "Could not create a new SignaturePolicyEvaluator using the given policy, crypto-helper")

	if !spe(ToIdentities(signers[0:1], &MockIdentityDeserializer{})) {
		t.Errorf("Expected authentication to succeed with valid signatures")
	}
	if spe(ToIdentities(signers[1:2], &MockIdentityDeserializer{})) {
		t.Errorf("Expected authentication to fail because signers[1] is not authorized in the policy, despite his valid signature")
	}
}

func TestMultipleSignature(t *testing.T) {
	t.Parallel()
	policy := policydsl.Envelope(policydsl.And(policydsl.SignedBy(0), policydsl.SignedBy(1)), signersBytes)

	spe, err := compile(policy.Rule, policy.Identities)
	if err != nil {
		t.Fatalf("Could not create a new SignaturePolicyEvaluator using the given policy, crypto-helper: %s", err)
	}

	if !spe(ToIdentities(signers, &MockIdentityDeserializer{})) {
		t.Errorf("Expected authentication to succeed with  valid signatures")
	}
	if spe(ToIdentities([]*msppb.Identity{signers[0], signers[0]}, &MockIdentityDeserializer{})) {
		t.Errorf("Expected authentication to fail because although there were two valid signatures, one was duplicated")
	}
}

func TestComplexNestedSignature(t *testing.T) {
	t.Parallel()
	policy := policydsl.Envelope(policydsl.And(
		policydsl.Or(
			policydsl.And(policydsl.SignedBy(0), policydsl.SignedBy(1)),
			policydsl.And(policydsl.SignedBy(0), policydsl.SignedBy(0)),
		),
		policydsl.SignedBy(0),
	), signersBytes)

	spe, err := compile(policy.Rule, policy.Identities)
	if err != nil {
		t.Fatalf("Could not create a new SignaturePolicyEvaluator using the given policy, crypto-helper: %s", err)
	}

	if !spe(ToIdentities(append(signers, msppb.NewIdentity("org1", []byte("signer0"))),
		&MockIdentityDeserializer{})) {
		t.Errorf("Expected authentication to succeed with valid signatures")
	}
	if !spe(ToIdentities([]*msppb.Identity{signers[0], signers[0], signers[0]},
		&MockIdentityDeserializer{})) {
		t.Errorf("Expected authentication to succeed with valid signatures")
	}
	if spe(ToIdentities(signers, &MockIdentityDeserializer{})) {
		t.Errorf("Expected authentication to fail with too few signatures")
	}
	if spe(ToIdentities(append(signers, msppb.NewIdentity("org1", []byte("signer1"))),
		&MockIdentityDeserializer{})) {
		t.Errorf("Expected authentication failure as there was a signature from signer[0] missing")
	}
}

func TestNegatively(t *testing.T) {
	t.Parallel()
	rpolicy := policydsl.Envelope(policydsl.And(policydsl.SignedBy(0), policydsl.SignedBy(1)), signersBytes)
	rpolicy.Rule.Type = nil
	b, _ := proto.Marshal(rpolicy)
	policy := &cb.SignaturePolicyEnvelope{}
	_ = proto.Unmarshal(b, policy)
	_, err := compile(policy.Rule, policy.Identities)
	if err == nil {
		t.Fatal("Should have errored compiling because the Type field was nil")
	}
}

func TestNilSignaturePolicyEnvelope(t *testing.T) {
	t.Parallel()
	_, err := compile(nil, nil)
	require.Error(t, err, "Fail to compile")
}

func TestSignedByMspClient(t *testing.T) {
	t.Parallel()
	e := policydsl.SignedByMspClient("A")
	require.Len(t, e.Identities, 1)

	role := &mb.MSPRole{}
	err := proto.Unmarshal(e.Identities[0].Principal, role)
	require.NoError(t, err)

	require.Equal(t, "A", role.MspIdentifier)
	require.Equal(t, mb.MSPRole_CLIENT, role.Role)

	e = policydsl.SignedByAnyClient([]string{"A"})
	require.Len(t, e.Identities, 1)

	role = &mb.MSPRole{}
	err = proto.Unmarshal(e.Identities[0].Principal, role)
	require.NoError(t, err)

	require.Equal(t, "A", role.MspIdentifier)
	require.Equal(t, mb.MSPRole_CLIENT, role.Role)
}

func TestSignedByMspPeer(t *testing.T) {
	t.Parallel()
	e := policydsl.SignedByMspPeer("A")
	require.Len(t, e.Identities, 1)

	role := &mb.MSPRole{}
	err := proto.Unmarshal(e.Identities[0].Principal, role)
	require.NoError(t, err)

	require.Equal(t, "A", role.MspIdentifier)
	require.Equal(t, mb.MSPRole_PEER, role.Role)

	e = policydsl.SignedByAnyPeer([]string{"A"})
	require.Len(t, e.Identities, 1)

	role = &mb.MSPRole{}
	err = proto.Unmarshal(e.Identities[0].Principal, role)
	require.NoError(t, err)

	require.Equal(t, "A", role.MspIdentifier)
	require.Equal(t, mb.MSPRole_PEER, role.Role)
}

func TestReturnNil(t *testing.T) {
	t.Parallel()
	policy := policydsl.Envelope(policydsl.And(policydsl.SignedBy(-1), policydsl.SignedBy(-2)), signersBytes)

	spe, err := compile(policy.Rule, policy.Identities)
	require.Nil(t, spe)
	require.EqualError(t, err, "identity index out of range, requested -1, but identities length is 2")
}
