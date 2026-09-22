/*
Copyright IBM Corp. All Rights Reserved.

SPDX-License-Identifier: Apache-2.0
*/

package cauthdsl

import (
	"fmt"
	"strconv"
	"testing"

	cb "github.com/hyperledger/fabric-protos-go-apiv2/common"
	mb "github.com/hyperledger/fabric-protos-go-apiv2/msp"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/proto"

	"github.com/hyperledger/fabric-x-common/api/msppb"
	"github.com/hyperledger/fabric-x-common/common/policies"
	"github.com/hyperledger/fabric-x-common/common/policydsl"
	"github.com/hyperledger/fabric-x-common/msp"
	"github.com/hyperledger/fabric-x-common/protoutil"
)

var (
	acceptAllPolicy *cb.Policy
	rejectAllPolicy *cb.Policy
)

func init() {
	acceptAllPolicy = makePolicySource(true)
	rejectAllPolicy = makePolicySource(false)
}

// The proto utils has become a dumping ground of cyclic imports, it's easier to define this locally
func marshalOrPanic(msg proto.Message) []byte {
	data, err := proto.Marshal(msg)
	if err != nil {
		panic(fmt.Errorf("Error marshaling messages: %s, %w", msg, err))
	}
	return data
}

func makePolicySource(policyResult bool) *cb.Policy { //nolint:revive // policyResult is not a control flag.
	var policyData *cb.SignaturePolicyEnvelope
	if policyResult {
		policyData = policydsl.AcceptAllPolicy
	} else {
		policyData = policydsl.RejectAllPolicy
	}
	return &cb.Policy{
		Type:  int32(cb.Policy_SIGNATURE),
		Value: marshalOrPanic(policyData),
	}
}

func providerMap() map[int32]policies.Provider {
	r := make(map[int32]policies.Provider)
	r[int32(cb.Policy_SIGNATURE)] = NewPolicyProvider(&MockIdentityDeserializer{})
	return r
}

func TestAccept(t *testing.T) {
	t.Parallel()
	policyID := "policyID"
	m, err := policies.NewManagerImpl("test", providerMap(), &cb.ConfigGroup{
		Policies: map[string]*cb.ConfigPolicy{
			policyID: {Policy: acceptAllPolicy},
		},
	})
	require.NoError(t, err)
	require.NotNil(t, m)

	policy, ok := m.GetPolicy(policyID)
	require.True(t, ok, "Should have found policy which was just added, but did not")
	err = policy.EvaluateSignedData(newSignedData("org1", "identity", "data", "sign"))
	require.NoError(t, err, "Should not have errored evaluating an acceptAll policy")
}

func TestReject(t *testing.T) {
	t.Parallel()
	policyID := "policyID"
	m, err := policies.NewManagerImpl("test", providerMap(), &cb.ConfigGroup{
		Policies: map[string]*cb.ConfigPolicy{
			policyID: {Policy: rejectAllPolicy},
		},
	})
	require.NoError(t, err)
	require.NotNil(t, m)
	policy, ok := m.GetPolicy(policyID)
	require.True(t, ok, "Should have found policy which was just added, but did not")
	err = policy.EvaluateSignedData(newSignedData("org1", "identity", "data", "sign"))
	require.Error(t, err, "Should have errored evaluating an rejectAll policy")
}

func TestRejectOnUnknown(t *testing.T) {
	t.Parallel()
	m, err := policies.NewManagerImpl("test", providerMap(), &cb.ConfigGroup{})
	require.NoError(t, err)
	require.NotNil(t, m)
	policy, ok := m.GetPolicy("FakePolicyID")
	require.False(t, ok, "Should not have found policy which was never added, but did")
	err = policy.EvaluateSignedData(newSignedData("org1", "identity", "data", "sign"))
	require.Error(t, err, "Should have errored evaluating the default policy")
}

func TestNewPolicyErrorCase(t *testing.T) {
	t.Parallel()
	provider := NewPolicyProvider(nil)

	pol1, msg1, err1 := provider.NewPolicy([]byte{0})
	require.Nil(t, pol1)
	require.Nil(t, msg1)
	require.ErrorContains(t, err1, "error unmarshalling to SignaturePolicy")

	sigPolicy2 := &cb.SignaturePolicyEnvelope{Version: -1}
	data2 := marshalOrPanic(sigPolicy2)
	pol2, msg2, err2 := provider.NewPolicy(data2)
	require.Nil(t, pol2)
	require.Nil(t, msg2)
	require.EqualError(t, err2, "this evaluator only understands messages of version 0, but version was -1")

	pol3, msg3, err3 := provider.NewPolicy([]byte{})
	require.Nil(t, pol3)
	require.Nil(t, msg3)
	require.EqualError(t, err3, "empty policy element")

	var pol4 *policy
	err4 := pol4.EvaluateSignedData([]*protoutil.SignedData{})
	require.EqualError(t, err4, "no such policy")
}

func TestEnvelopeBasedPolicyProvider(t *testing.T) {
	t.Parallel()
	pp := &EnvelopeBasedPolicyProvider{Deserializer: &MockIdentityDeserializer{}}
	p, err := pp.NewPolicy(nil)
	require.Nil(t, p)
	require.Error(t, err, "invalid arguments")

	p, err = pp.NewPolicy(&cb.SignaturePolicyEnvelope{})
	require.Nil(t, p)
	require.Error(t, err, "empty policy element")

	p, err = pp.NewPolicy(policydsl.SignedByMspPeer("primus inter pares"))
	require.NotNil(t, p)
	require.NoError(t, err)
}

func TestConverter(t *testing.T) {
	t.Parallel()
	p := policy{}

	cp, err := p.Convert()
	require.Nil(t, cp)
	require.Error(t, err)
	require.Contains(t, err.Error(), "nil policy field")

	p.signaturePolicyEnvelope = policydsl.RejectAllPolicy

	cp, err = p.Convert()
	require.NotNil(t, cp)
	require.NoError(t, err)
	require.Equal(t, cp, policydsl.RejectAllPolicy)
}

func newSignedData(mspID, cert, data, sign string) []*protoutil.SignedData {
	return []*protoutil.SignedData{{
		Identity: msppb.NewIdentity(mspID, []byte(cert)),
		Data:     []byte(data), Signature: []byte(sign),
	}}
}

// countingIdentity counts the signature verifications performed on the identity it wraps.
type countingIdentity struct {
	msp.Identity
	verifies *int
}

func (id *countingIdentity) Verify(msg, sig []byte) error {
	*id.verifies++
	return id.Identity.Verify(msg, sig)
}

// countingDeserializer hands out identities that count how often they are verified.
type countingDeserializer struct {
	mspID    string
	idBytes  []byte
	verifies int
}

func (d *countingDeserializer) DeserializeIdentity(_ *msppb.Identity) (msp.Identity, error) { //nolint:ireturn
	return &countingIdentity{
		Identity: &MockIdentity{MspID: d.mspID, IDBytes: d.idBytes},
		verifies: &d.verifies,
	}, nil
}

func (*countingDeserializer) GetKnownDeserializedIdentity(msp.IdentityIdentifier) msp.Identity { //nolint:ireturn
	return nil
}

func (*countingDeserializer) IsWellFormed(*msppb.Identity) error { return nil }

// implicitMetaOverSignaturePolicies builds an implicit meta policy of the given rule with one
// signature sub-policy per principal, each requiring its own principal.
func implicitMetaOverSignaturePolicies( //nolint:ireturn // policies.NewImplicitMetaPolicy returns this interface
	t *testing.T,
	rule cb.ImplicitMetaPolicy_Rule,
	principals [][]byte,
	deserializer msp.IdentityDeserializer,
) policies.Policy {
	t.Helper()

	const subPolicyName = "SubPolicy"
	managers := make(map[string]*policies.ManagerImpl, len(principals))
	for i, principal := range principals {
		policy, _, err := NewPolicyProvider(deserializer).NewPolicy(protoutil.MarshalOrPanic(
			&cb.SignaturePolicyEnvelope{
				Version:    0,
				Rule:       policydsl.SignedBy(0),
				Identities: []*mb.MSPPrincipal{{Principal: principal}},
			}))
		require.NoError(t, err)
		managers[strconv.Itoa(i)] = &policies.ManagerImpl{
			Policies: map[string]policies.Policy{subPolicyName: policy},
		}
	}

	imp, err := policies.NewImplicitMetaPolicy(protoutil.MarshalOrPanic(&cb.ImplicitMetaPolicy{
		Rule:      rule,
		SubPolicy: subPolicyName,
	}), managers)
	require.NoError(t, err)

	return imp
}

// signerPrincipal is the principal that the identities countingDeserializer hands out satisfy.
func signerPrincipal(t *testing.T, d *countingDeserializer) []byte {
	t.Helper()

	identity, err := d.DeserializeIdentity(nil)
	require.NoError(t, err)
	principal, err := identity.Serialize()
	require.NoError(t, err)

	return principal
}

func signedDataFromSigner() []*protoutil.SignedData {
	return []*protoutil.SignedData{{
		Data:      []byte("data1"),
		Identity:  msppb.NewIdentity("org1", []byte("identity1")),
		Signature: []byte("signature1"),
	}}
}

func TestImplicitMetaPolicyVerifiesTheSignatureOnlyForTheSubPolicyThatMatches(t *testing.T) {
	t.Parallel()

	// Scenario:
	// - an implicit meta ANY policy over four sub-policies, only the last of which requires the
	//   signer's principal
	// - evaluate a signature set carrying that signer once
	// - the policy is satisfied by that one sub-policy
	// - the signature is verified once however many sub-policies were tried before it, because a
	//   sub-policy whose principal does not match never reaches the verification
	deserializer := &countingDeserializer{mspID: "org1", idBytes: []byte("identity1")}
	principal := signerPrincipal(t, deserializer)
	other := []byte("a principal this signer cannot satisfy")

	policy := implicitMetaOverSignaturePolicies(
		t, cb.ImplicitMetaPolicy_ANY, [][]byte{other, other, other, principal}, deserializer)

	require.NoError(t, policy.EvaluateSignedData(signedDataFromSigner()))
	require.Equal(t, 1, deserializer.verifies)
}

func TestImplicitMetaPolicyVerifiesNoSignatureWhenNoPrincipalMatches(t *testing.T) {
	t.Parallel()

	// Scenario:
	// - an implicit meta ANY policy over four sub-policies, none requiring the signer's principal
	// - evaluate a signature set carrying that signer once
	// - every sub-policy is tried and none is satisfied, so the policy is refused
	// - no signature is verified at all, because no principal ever matched
	deserializer := &countingDeserializer{mspID: "org1", idBytes: []byte("identity1")}
	other := []byte("a principal this signer cannot satisfy")

	policy := implicitMetaOverSignaturePolicies(
		t, cb.ImplicitMetaPolicy_ANY, [][]byte{other, other, other, other}, deserializer)

	require.Error(t, policy.EvaluateSignedData(signedDataFromSigner()))
	require.Zero(t, deserializer.verifies)
}
