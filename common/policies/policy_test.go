/*
Copyright IBM Corp. All Rights Reserved.

SPDX-License-Identifier: Apache-2.0
*/

package policies

import (
	"fmt"
	"reflect"
	"strconv"
	"testing"

	"github.com/hyperledger/fabric-lib-go/common/flogging/floggingtest"
	cb "github.com/hyperledger/fabric-protos-go-apiv2/common"
	"github.com/hyperledger/fabric-protos-go-apiv2/msp"
	"github.com/pkg/errors"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap/zapcore"
	"google.golang.org/protobuf/proto"

	"github.com/hyperledger/fabric-x-common/api/msppb"
	"github.com/hyperledger/fabric-x-common/common/crypto/tlsgen"
	"github.com/hyperledger/fabric-x-common/common/policies/mocks"
	mspi "github.com/hyperledger/fabric-x-common/msp"
	"github.com/hyperledger/fabric-x-common/protoutil"
)

//go:generate counterfeiter -o mocks/identity_deserializer.go --fake-name IdentityDeserializer . identityDeserializer
type identityDeserializer interface {
	mspi.IdentityDeserializer
}

//go:generate counterfeiter -o mocks/identity.go --fake-name Identity . identity
type identity interface {
	mspi.Identity
}

type mockProvider struct{}

func (mpp mockProvider) NewPolicy(data []byte) (Policy, proto.Message, error) {
	return nil, nil, nil
}

const mockType = int32(0)

func defaultProviders() map[int32]Provider {
	providers := make(map[int32]Provider)
	providers[mockType] = &mockProvider{}
	return providers
}

func TestUnnestedManager(t *testing.T) {
	config := &cb.ConfigGroup{
		Policies: map[string]*cb.ConfigPolicy{
			"1": {Policy: &cb.Policy{Type: mockType}},
			"2": {Policy: &cb.Policy{Type: mockType}},
			"3": {Policy: &cb.Policy{Type: mockType}},
		},
	}

	m, err := NewManagerImpl("test", defaultProviders(), config)
	require.NoError(t, err)
	require.NotNil(t, m)

	_, ok := m.Manager([]string{"subGroup"})
	require.False(t, ok, "Should not have found a subgroup manager")

	r, ok := m.Manager([]string{})
	require.True(t, ok, "Should have found the root manager")
	require.Equal(t, m, r)

	require.Len(t, m.Policies, len(config.Policies))

	for policyName := range config.Policies {
		_, ok := m.GetPolicy(policyName)
		require.True(t, ok, "Should have found policy %s", policyName)
	}
}

func TestNestedManager(t *testing.T) {
	config := &cb.ConfigGroup{
		Policies: map[string]*cb.ConfigPolicy{
			"n0a": {Policy: &cb.Policy{Type: mockType}},
			"n0b": {Policy: &cb.Policy{Type: mockType}},
			"n0c": {Policy: &cb.Policy{Type: mockType}},
		},
		Groups: map[string]*cb.ConfigGroup{
			"nest1": {
				Policies: map[string]*cb.ConfigPolicy{
					"n1a": {Policy: &cb.Policy{Type: mockType}},
					"n1b": {Policy: &cb.Policy{Type: mockType}},
					"n1c": {Policy: &cb.Policy{Type: mockType}},
				},
				Groups: map[string]*cb.ConfigGroup{
					"nest2a": {
						Policies: map[string]*cb.ConfigPolicy{
							"n2a_1": {Policy: &cb.Policy{Type: mockType}},
							"n2a_2": {Policy: &cb.Policy{Type: mockType}},
							"n2a_3": {Policy: &cb.Policy{Type: mockType}},
						},
					},
					"nest2b": {
						Policies: map[string]*cb.ConfigPolicy{
							"n2b_1": {Policy: &cb.Policy{Type: mockType}},
							"n2b_2": {Policy: &cb.Policy{Type: mockType}},
							"n2b_3": {Policy: &cb.Policy{Type: mockType}},
						},
					},
				},
			},
		},
	}

	m, err := NewManagerImpl("nest0", defaultProviders(), config)
	require.NoError(t, err)
	require.NotNil(t, m)

	r, ok := m.Manager([]string{})
	require.True(t, ok, "Should have found the root manager")
	require.Equal(t, m, r)

	n1, ok := m.Manager([]string{"nest1"})
	require.True(t, ok)
	n2a, ok := m.Manager([]string{"nest1", "nest2a"})
	require.True(t, ok)
	n2b, ok := m.Manager([]string{"nest1", "nest2b"})
	require.True(t, ok)

	n2as, ok := n1.Manager([]string{"nest2a"})
	require.True(t, ok)
	require.Equal(t, n2a, n2as)
	n2bs, ok := n1.Manager([]string{"nest2b"})
	require.True(t, ok)
	require.Equal(t, n2b, n2bs)

	absPrefix := PathSeparator + "nest0" + PathSeparator
	for policyName := range config.Policies {
		_, ok := m.GetPolicy(policyName)
		require.True(t, ok, "Should have found policy %s", policyName)

		absName := absPrefix + policyName
		_, ok = m.GetPolicy(absName)
		require.True(t, ok, "Should have found absolute policy %s", absName)
	}

	for policyName := range config.Groups["nest1"].Policies {
		_, ok := n1.GetPolicy(policyName)
		require.True(t, ok, "Should have found policy %s", policyName)

		relPathFromBase := "nest1" + PathSeparator + policyName
		_, ok = m.GetPolicy(relPathFromBase)
		require.True(t, ok, "Should have found policy %s", policyName)

		for i, abs := range []Manager{n1, m} {
			absName := absPrefix + relPathFromBase
			_, ok = abs.GetPolicy(absName)
			require.True(t, ok, "Should have found absolutely policy for manager %d", i)
		}
	}

	for policyName := range config.Groups["nest1"].Groups["nest2a"].Policies {
		_, ok := n2a.GetPolicy(policyName)
		require.True(t, ok, "Should have found policy %s", policyName)

		relPathFromN1 := "nest2a" + PathSeparator + policyName
		_, ok = n1.GetPolicy(relPathFromN1)
		require.True(t, ok, "Should have found policy %s", policyName)

		relPathFromBase := "nest1" + PathSeparator + relPathFromN1
		_, ok = m.GetPolicy(relPathFromBase)
		require.True(t, ok, "Should have found policy %s", policyName)

		for i, abs := range []Manager{n2a, n1, m} {
			absName := absPrefix + relPathFromBase
			_, ok = abs.GetPolicy(absName)
			require.True(t, ok, "Should have found absolutely policy for manager %d", i)
		}
	}

	for policyName := range config.Groups["nest1"].Groups["nest2b"].Policies {
		_, ok := n2b.GetPolicy(policyName)
		require.True(t, ok, "Should have found policy %s", policyName)

		relPathFromN1 := "nest2b" + PathSeparator + policyName
		_, ok = n1.GetPolicy(relPathFromN1)
		require.True(t, ok, "Should have found policy %s", policyName)

		relPathFromBase := "nest1" + PathSeparator + relPathFromN1
		_, ok = m.GetPolicy(relPathFromBase)
		require.True(t, ok, "Should have found policy %s", policyName)

		for i, abs := range []Manager{n2b, n1, m} {
			absName := absPrefix + relPathFromBase
			_, ok = abs.GetPolicy(absName)
			require.True(t, ok, "Should have found absolutely policy for manager %d", i)
		}
	}
}

func TestPrincipalUniqueSet(t *testing.T) {
	var principalSet PrincipalSet
	addPrincipal := func(i int) {
		principalSet = append(principalSet, &msp.MSPPrincipal{
			PrincipalClassification: msp.MSPPrincipal_Classification(i),
			Principal:               []byte(fmt.Sprintf("%d", i)),
		})
	}

	addPrincipal(1)
	addPrincipal(2)
	addPrincipal(2)
	addPrincipal(3)
	addPrincipal(3)
	addPrincipal(3)

	for principal, plurality := range principalSet.UniqueSet() {
		require.Equal(t, int(principal.PrincipalClassification), plurality)
		require.Equal(t, fmt.Sprintf("%d", plurality), string(principal.Principal))
	}

	v := reflect.Indirect(reflect.ValueOf(msp.MSPPrincipal{}))
	// Ensure msp.MSPPrincipal has only 2 fields.
	// This is essential for 'UniqueSet' to work properly
	// XXX This is a rather brittle check and brittle way to fix the test
	// There seems to be an assumption that the number of fields in the proto
	// struct matches the number of fields in the proto message
	require.Equal(t, 5, v.NumField())
}

func TestPrincipalSetContainingOnly(t *testing.T) {
	var principalSets PrincipalSets
	var principalSet PrincipalSet
	for j := range 3 {
		for i := range 10 {
			principalSet = append(principalSet, &msp.MSPPrincipal{
				PrincipalClassification: msp.MSPPrincipal_IDENTITY,
				Principal:               []byte(fmt.Sprintf("%d", j*10+i)),
			})
		}
		principalSets = append(principalSets, principalSet)
		principalSet = nil
	}

	between20And30 := func(principal *msp.MSPPrincipal) bool {
		n, _ := strconv.ParseInt(string(principal.Principal), 10, 32)
		return n >= 20 && n <= 29
	}

	principalSets = principalSets.ContainingOnly(between20And30)

	require.Len(t, principalSets, 1)
	require.True(t, principalSets[0].ContainingOnly(between20And30))
}

func TestSignatureSetToValidIdentities(t *testing.T) {
	id := msppb.NewIdentity("org1", []byte("identity1"))
	sd := []*protoutil.SignedData{
		{
			Data:      []byte("data1"),
			Identity:  id,
			Signature: []byte("signature1"),
		},
		{
			Data:      []byte("data1"),
			Identity:  id,
			Signature: []byte("signature1"),
		},
	}

	fIDDs := &mocks.IdentityDeserializer{}
	fID := &mocks.Identity{}
	fID.VerifyReturns(nil)
	fID.GetIdentifierReturns(&mspi.IdentityIdentifier{
		Id:    "id",
		Mspid: "mspid",
	})
	fIDDs.DeserializeIdentityReturns(fID, nil)

	ids := SignatureSetToValidIdentities(sd, fIDDs)
	require.Len(t, ids, 1)
	require.NotNil(t, ids[0].GetIdentifier())
	require.Equal(t, "id", ids[0].GetIdentifier().Id)
	require.Equal(t, "mspid", ids[0].GetIdentifier().Mspid)
	data, sig := fID.VerifyArgsForCall(0)
	require.Equal(t, []byte("data1"), data)
	require.Equal(t, []byte("signature1"), sig)
	sidBytes := fIDDs.DeserializeIdentityArgsForCall(0)
	require.True(t, proto.Equal(id, sidBytes))
}

func TestSignatureSetToValidIdentitiesDeserializeErr(t *testing.T) {
	oldLogger := logger
	l, recorder := floggingtest.NewTestLogger(t, floggingtest.AtLevel(zapcore.InfoLevel))
	logger = l
	defer func() { logger = oldLogger }()

	fakeIdentityDeserializer := &mocks.IdentityDeserializer{}
	fakeIdentityDeserializer.DeserializeIdentityReturns(nil, errors.New("mango"))

	// generate actual x509 certificate
	ca, err := tlsgen.NewCA()
	require.NoError(t, err)
	client1, err := ca.NewClientCertKeyPair()
	require.NoError(t, err)

	invalidCertIdentity := msppb.NewIdentity("org1", []byte("identity1"))
	tests := []struct {
		spec                     string
		signedData               []*protoutil.SignedData
		expectedLogEntryContains []string
	}{
		{
			spec: "deserialize identity error - identity is random bytes",
			signedData: []*protoutil.SignedData{
				{
					Identity: invalidCertIdentity,
				},
			},
			expectedLogEntryContains: []string{
				"invalid identity", fmt.Sprintf("serialized-identity=%x", invalidCertIdentity), "error=mango",
			},
		},
		{
			spec: "deserialize identity error - actual certificate",
			signedData: []*protoutil.SignedData{
				{
					Identity: msppb.NewIdentity("org1", client1.Cert),
				},
			},
			expectedLogEntryContains: []string{
				"invalid identity",
				fmt.Sprintf("mspid=org1 subject=%s issuer=%s serialnumber=%d",
					client1.TLSCert.Subject, client1.TLSCert.Issuer, client1.TLSCert.SerialNumber),
				"error=mango",
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.spec, func(t *testing.T) {
			ids := SignatureSetToValidIdentities(tc.signedData, fakeIdentityDeserializer)
			require.Len(t, ids, 0)
			assertLogContains(t, recorder, tc.expectedLogEntryContains...)
		})
	}
}

func TestSignatureSetToValidIdentitiesVerifyErr(t *testing.T) {
	id := msppb.NewIdentity("org1", []byte("identity1"))
	sd := []*protoutil.SignedData{
		{
			Data:      []byte("data1"),
			Identity:  id,
			Signature: []byte("signature1"),
		},
	}

	fIDDs := &mocks.IdentityDeserializer{}
	fID := &mocks.Identity{}
	fID.VerifyReturns(errors.New("bad signature"))
	fID.GetIdentifierReturns(&mspi.IdentityIdentifier{
		Id:    "id",
		Mspid: "mspid",
	})
	fIDDs.DeserializeIdentityReturns(fID, nil)

	ids := SignatureSetToValidIdentities(sd, fIDDs)
	require.Len(t, ids, 0)
	data, sig := fID.VerifyArgsForCall(0)
	require.Equal(t, []byte("data1"), data)
	require.Equal(t, []byte("signature1"), sig)
	sidBytes := fIDDs.DeserializeIdentityArgsForCall(0)
	require.True(t, proto.Equal(id, sidBytes))
}

func assertLogContains(t *testing.T, r *floggingtest.Recorder, ss ...string) {
	defer r.Reset()
	entries := r.Entries()
	for _, entry := range entries {
		fmt.Println(entry)
	}
	for _, s := range ss {
		require.NotEmpty(t, r.EntriesContaining(s))
	}
}

func deferredTestSetup(satisfies, verify error) (*mocks.Identity, []mspi.Identity) {
	id := msppb.NewIdentity("org1", []byte("identity1"))
	sd := []*protoutil.SignedData{
		{Data: []byte("data1"), Identity: id, Signature: []byte("signature1")},
		{Data: []byte("data1"), Identity: id, Signature: []byte("signature1")},
	}

	fIDDs := &mocks.IdentityDeserializer{}
	fID := &mocks.Identity{}
	fID.SatisfiesPrincipalReturns(satisfies)
	fID.VerifyReturns(verify)
	fID.GetIdentifierReturns(&mspi.IdentityIdentifier{Id: "id", Mspid: "mspid"})
	fIDDs.DeserializeIdentityReturns(fID, nil)

	return fID, SignatureSetToDeferredVerificationIdentities(sd, fIDDs)
}

func TestSignatureSetToDeferredVerificationIdentitiesDefersTheVerification(t *testing.T) {
	t.Parallel()

	// Scenario:
	// - convert a signature set of two entries from one signer
	// - the set is deduplicated to a single identity
	// - the conversion verifies no signature
	fID, ids := deferredTestSetup(nil, nil)

	require.Len(t, ids, 1)
	require.Equal(t, "id", ids[0].GetIdentifier().Id)
	require.Equal(t, "mspid", ids[0].GetIdentifier().Mspid)
	require.Zero(t, fID.VerifyCallCount())
}

func TestSignatureSetToDeferredVerificationIdentitiesVerifiesOnceWhenAPrincipalMatches(t *testing.T) {
	t.Parallel()

	// Scenario:
	// - convert a signature set whose signer satisfies the principal it is offered
	// - offering a principal verifies the signature, over the payload and signature in the set
	// - offering two more principals does not verify it again
	fID, ids := deferredTestSetup(nil, nil)

	require.NoError(t, ids[0].SatisfiesPrincipal(&msp.MSPPrincipal{}))
	require.Equal(t, 1, fID.VerifyCallCount())

	data, sig := fID.VerifyArgsForCall(0)
	require.Equal(t, []byte("data1"), data)
	require.Equal(t, []byte("signature1"), sig)

	require.NoError(t, ids[0].SatisfiesPrincipal(&msp.MSPPrincipal{}))
	require.NoError(t, ids[0].SatisfiesPrincipal(&msp.MSPPrincipal{}))
	require.Equal(t, 1, fID.VerifyCallCount())
}

func TestSignatureSetToDeferredVerificationIdentitiesDoesNotVerifyWhenNoPrincipalMatches(t *testing.T) {
	t.Parallel()

	// Scenario:
	// - convert a signature set whose signer satisfies no principal it is offered
	// - both principals offered are refused
	// - no signature is verified
	fID, ids := deferredTestSetup(errors.New("mango"), nil)

	require.EqualError(t, ids[0].SatisfiesPrincipal(&msp.MSPPrincipal{}), "mango")
	require.EqualError(t, ids[0].SatisfiesPrincipal(&msp.MSPPrincipal{}), "mango")
	require.Zero(t, fID.VerifyCallCount())
}

func TestSignatureSetToDeferredVerificationIdentitiesRefusesAnInvalidSignature(t *testing.T) {
	t.Parallel()

	// Scenario:
	// - convert a signature set whose signer satisfies its principal but signed badly
	// - the principal is refused with the verification error, and again on a second offer
	// - the signature is verified only once
	fID, ids := deferredTestSetup(nil, errors.New("papaya"))

	require.ErrorContains(t, ids[0].SatisfiesPrincipal(&msp.MSPPrincipal{}), "signature verification failed: papaya")
	require.ErrorContains(t, ids[0].SatisfiesPrincipal(&msp.MSPPrincipal{}), "signature verification failed: papaya")
	require.Equal(t, 1, fID.VerifyCallCount())
}

func TestSignatureSetToDeferredVerificationIdentitiesSkipsAnIdentityItCannotDeserialize(t *testing.T) {
	t.Parallel()

	// Scenario:
	// - convert a signature set whose identity cannot be deserialized
	// - no identity is returned
	fIDDs := &mocks.IdentityDeserializer{}
	fIDDs.DeserializeIdentityReturns(nil, errors.New("guava"))

	ids := SignatureSetToDeferredVerificationIdentities([]*protoutil.SignedData{
		{Data: []byte("data1"), Identity: msppb.NewIdentity("org1", []byte("identity1")), Signature: []byte("sig")},
	}, fIDDs)

	require.Empty(t, ids)
}

func TestSignatureSetToDeferredVerificationIdentitiesVerifiesEachIdentityOnce(t *testing.T) {
	t.Parallel()

	// Scenario:
	// - convert a signature set of three entries from three different signers, each appearing once
	// - the middle signer carries an invalid signature, the other two are valid
	// - offer every identity a principal it satisfies, twice each
	// - each identity is verified exactly once, over its own payload and signature
	// - each identity keeps its own outcome: the middle one is refused, the others accepted
	const signers = 3

	sd := make([]*protoutil.SignedData, 0, signers)
	fakes := make([]*mocks.Identity, 0, signers)
	fIDDs := &mocks.IdentityDeserializer{}

	for i := range signers {
		fID := &mocks.Identity{}
		fID.SatisfiesPrincipalReturns(nil)
		fID.GetIdentifierReturns(&mspi.IdentityIdentifier{Id: strconv.Itoa(i), Mspid: "mspid"})
		if i == 1 {
			fID.VerifyReturns(errors.New("mango"))
		}
		fakes = append(fakes, fID)
		fIDDs.DeserializeIdentityReturnsOnCall(i, fID, nil)

		sd = append(sd, &protoutil.SignedData{
			Data:      []byte("data" + strconv.Itoa(i)),
			Identity:  msppb.NewIdentity("org1", []byte("identity"+strconv.Itoa(i))),
			Signature: []byte("signature" + strconv.Itoa(i)),
		})
	}

	ids := SignatureSetToDeferredVerificationIdentities(sd, fIDDs)
	require.Len(t, ids, signers)
	for _, fID := range fakes {
		require.Zero(t, fID.VerifyCallCount())
	}

	for range 2 {
		require.NoError(t, ids[0].SatisfiesPrincipal(&msp.MSPPrincipal{}))
		require.ErrorContains(t, ids[1].SatisfiesPrincipal(&msp.MSPPrincipal{}), "signature verification failed: mango")
		require.NoError(t, ids[2].SatisfiesPrincipal(&msp.MSPPrincipal{}))
	}

	for i, fID := range fakes {
		require.Equal(t, 1, fID.VerifyCallCount(), "identity %d", i)
		data, sig := fID.VerifyArgsForCall(0)
		require.Equal(t, []byte("data"+strconv.Itoa(i)), data)
		require.Equal(t, []byte("signature"+strconv.Itoa(i)), sig)
	}
}

func TestSignatureSetToDeferredVerificationIdentitiesDeduplicatesBeforeVerifying(t *testing.T) {
	t.Parallel()

	// Scenario:
	// - convert a signature set carrying one signer twice, the first entry signed badly and the
	//   second signed well
	// - the set is deduplicated to the first entry, so the good signature is never reached
	// - offering the identity a principal is refused, and only the bad signature was verified
	// - this is the deliberate difference from SignatureSetToValidIdentities, which records a signer
	//   only after its signature verifies and so would have passed on the second entry
	id := msppb.NewIdentity("org1", []byte("identity1"))
	sd := []*protoutil.SignedData{
		{Data: []byte("data1"), Identity: id, Signature: []byte("bad signature")},
		{Data: []byte("data1"), Identity: id, Signature: []byte("good signature")},
	}

	fIDDs := &mocks.IdentityDeserializer{}
	invalid, valid := &mocks.Identity{}, &mocks.Identity{}
	for i, fID := range []*mocks.Identity{invalid, valid} {
		fID.SatisfiesPrincipalReturns(nil)
		fID.GetIdentifierReturns(&mspi.IdentityIdentifier{Id: "id", Mspid: "mspid"})
		fIDDs.DeserializeIdentityReturnsOnCall(i, fID, nil)
	}
	invalid.VerifyReturns(errors.New("mango"))
	valid.VerifyReturns(nil)

	ids := SignatureSetToDeferredVerificationIdentities(sd, fIDDs)
	require.Len(t, ids, 1)

	require.ErrorContains(t, ids[0].SatisfiesPrincipal(&msp.MSPPrincipal{}), "signature verification failed: mango")
	require.Equal(t, 1, invalid.VerifyCallCount())
	_, sig := invalid.VerifyArgsForCall(0)
	require.Equal(t, []byte("bad signature"), sig)
	require.Zero(t, valid.VerifyCallCount())
}
