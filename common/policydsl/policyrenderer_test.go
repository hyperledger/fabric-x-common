/*
Copyright IBM Corp. All Rights Reserved.

SPDX-License-Identifier: Apache-2.0
*/

package policydsl

import (
	"testing"

	"github.com/hyperledger/fabric-protos-go-apiv2/common"
	"github.com/hyperledger/fabric-protos-go-apiv2/msp"
	"github.com/stretchr/testify/require"

	"github.com/hyperledger/fabric-x-common/protoutil"
	"github.com/hyperledger/fabric-x-common/utils/test"
)

// TestToString_RoundTrips asserts that rendering a policy produced by FromString and feeding
// the result back through FromString yields a semantically identical envelope. This is the
// load-bearing assertion: it can't be satisfied by a renderer that merely produces
// plausible-looking text.
func TestToString_RoundTrips(t *testing.T) {
	t.Parallel()

	cases := map[string]string{
		"or of three":  "OR('Org1MSP.member', 'Org2MSP.member', 'Org3MSP.member')",
		"and":          "AND('Org1MSP.admin', 'Org2MSP.admin')",
		"out of":       "OutOf(2, 'A.member', 'B.member', 'C.member')",
		"role member":  "OR('Org1MSP.member')",
		"role admin":   "OR('Org1MSP.admin')",
		"role client":  "OR('Org1MSP.client')",
		"role peer":    "OR('Org1MSP.peer')",
		"role orderer": "OR('Org1MSP.orderer')",
		"nested tree":  "AND(OR(AND('A.member', 'B.member'), AND('A.member', 'C.member')), 'D.member')",
	}

	for name, dsl := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			envelope, err := FromString(dsl)
			require.NoError(t, err)

			rendered, err := ToString(envelope)
			require.NoError(t, err)

			roundTripped, err := FromString(rendered)
			require.NoError(t, err, "rendered DSL %q must parse", rendered)
			test.RequireProtoEqual(t, envelope, roundTripped)
		})
	}
}

// TestToString_ExactRendering pins the exact rendered string for DSL that is already in
// canonical form, catching accidental formatting regressions (spacing, gate casing) that the
// round-trip test alone would not.
func TestToString_ExactRendering(t *testing.T) {
	t.Parallel()

	cases := []string{
		"OR('Org1MSP.member', 'Org2MSP.member', 'Org3MSP.member')",
		"AND('Org1MSP.admin', 'Org2MSP.admin')",
		"OR('Org1MSP.member')",
	}

	for _, dsl := range cases {
		t.Run(dsl, func(t *testing.T) {
			t.Parallel()

			envelope, err := FromString(dsl)
			require.NoError(t, err)

			rendered, err := ToString(envelope)
			require.NoError(t, err)
			require.Equal(t, dsl, rendered)
		})
	}
}

func TestToString_AcceptAllPolicy(t *testing.T) {
	t.Parallel()

	rendered, err := ToString(AcceptAllPolicy)
	require.NoError(t, err)
	require.Equal(t, "OutOf(0)", rendered)
}

func TestToString_RejectAllPolicy(t *testing.T) {
	t.Parallel()

	rendered, err := ToString(RejectAllPolicy)
	require.NoError(t, err)
	require.Equal(t, "OutOf(1)", rendered)
}

func TestToString_IdentityPrincipalFails(t *testing.T) {
	t.Parallel()

	envelope := Envelope(SignedBy(0), [][]byte{[]byte("a serialized identity")})

	_, err := ToString(envelope)
	require.ErrorContains(t, err, "no DSL representation")
}

func TestToString_SignedByOutOfRangeFails(t *testing.T) {
	t.Parallel()

	envelope := &common.SignaturePolicyEnvelope{Rule: SignedBy(0)}

	_, err := ToString(envelope)
	require.ErrorContains(t, err, "out of range")
}

// TestToDisplayString_IdentityPrincipal asserts that, unlike ToString, ToDisplayString never
// fails on a construct with no DSL representation: it substitutes a readable placeholder.
// Envelope() classifies every identity as IDENTITY, so this exercises that placeholder path.
func TestToDisplayString_IdentityPrincipal(t *testing.T) {
	t.Parallel()

	envelope := Envelope(SignedBy(0), [][]byte{[]byte("a serialized identity")})

	require.Equal(t, "<identity:0>", ToDisplayString(envelope))
}

func TestToDisplayString_SignedByOutOfRange(t *testing.T) {
	t.Parallel()

	envelope := &common.SignaturePolicyEnvelope{Rule: SignedBy(0)}

	require.Equal(t, "<principal[0]>", ToDisplayString(envelope))
}

// TestToDisplayString_MalformedPrincipalFallsBackToPlaceholder asserts that a ROLE-classified
// principal whose bytes don't unmarshal into an MSPRole (corrupt data, not a client bug) still
// renders as a placeholder instead of panicking.
func TestToDisplayString_MalformedPrincipalFallsBackToPlaceholder(t *testing.T) {
	t.Parallel()

	envelope := &common.SignaturePolicyEnvelope{
		Rule: SignedBy(0),
		Identities: []*msp.MSPPrincipal{
			{PrincipalClassification: msp.MSPPrincipal_ROLE, Principal: []byte("not an MSPRole")},
		},
	}

	require.Equal(t, "<principal:6e6f7420616e204d>", ToDisplayString(envelope))
}

func TestToDisplayString_OrganizationUnitPrincipal(t *testing.T) {
	t.Parallel()

	envelope := &common.SignaturePolicyEnvelope{
		Rule: SignedBy(0),
		Identities: []*msp.MSPPrincipal{
			{
				PrincipalClassification: msp.MSPPrincipal_ORGANIZATION_UNIT,
				Principal: protoutil.MarshalOrPanic(&msp.OrganizationUnit{
					MspIdentifier:                "Org1MSP",
					OrganizationalUnitIdentifier: "engineering",
				}),
			},
		},
	}

	require.Equal(t, "'Org1MSP.engineering'", ToDisplayString(envelope))
}
