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

func TestOutOf1(t *testing.T) {
	p1, err := FromString("OutOf(1, 'A.member', 'B.member')")
	require.NoError(t, err)

	principals := make([]*msp.MSPPrincipal, 0)

	principals = append(principals, &msp.MSPPrincipal{
		PrincipalClassification: msp.MSPPrincipal_ROLE,
		Principal:               protoutil.MarshalOrPanic(&msp.MSPRole{Role: msp.MSPRole_MEMBER, MspIdentifier: "A"}),
	})

	principals = append(principals, &msp.MSPPrincipal{
		PrincipalClassification: msp.MSPPrincipal_ROLE,
		Principal:               protoutil.MarshalOrPanic(&msp.MSPRole{Role: msp.MSPRole_MEMBER, MspIdentifier: "B"}),
	})

	p2 := &common.SignaturePolicyEnvelope{
		Version:    0,
		Rule:       NOutOf(1, []*common.SignaturePolicy{SignedBy(0), SignedBy(1)}),
		Identities: principals,
	}

	test.RequireProtoEqual(t, p1, p2)
}

func TestOutOf2(t *testing.T) {
	p1, err := FromString("OutOf(2, 'A.member', 'B.member')")
	require.NoError(t, err)

	principals := make([]*msp.MSPPrincipal, 0)

	principals = append(principals, &msp.MSPPrincipal{
		PrincipalClassification: msp.MSPPrincipal_ROLE,
		Principal:               protoutil.MarshalOrPanic(&msp.MSPRole{Role: msp.MSPRole_MEMBER, MspIdentifier: "A"}),
	})

	principals = append(principals, &msp.MSPPrincipal{
		PrincipalClassification: msp.MSPPrincipal_ROLE,
		Principal:               protoutil.MarshalOrPanic(&msp.MSPRole{Role: msp.MSPRole_MEMBER, MspIdentifier: "B"}),
	})

	p2 := &common.SignaturePolicyEnvelope{
		Version:    0,
		Rule:       NOutOf(2, []*common.SignaturePolicy{SignedBy(0), SignedBy(1)}),
		Identities: principals,
	}

	test.RequireProtoEqual(t, p1, p2)
}

func TestAnd(t *testing.T) {
	p1, err := FromString("AND('A.member', 'B.member')")
	require.NoError(t, err)

	principals := make([]*msp.MSPPrincipal, 0)

	principals = append(principals, &msp.MSPPrincipal{
		PrincipalClassification: msp.MSPPrincipal_ROLE,
		Principal:               protoutil.MarshalOrPanic(&msp.MSPRole{Role: msp.MSPRole_MEMBER, MspIdentifier: "A"}),
	})

	principals = append(principals, &msp.MSPPrincipal{
		PrincipalClassification: msp.MSPPrincipal_ROLE,
		Principal:               protoutil.MarshalOrPanic(&msp.MSPRole{Role: msp.MSPRole_MEMBER, MspIdentifier: "B"}),
	})

	p2 := &common.SignaturePolicyEnvelope{
		Version:    0,
		Rule:       And(SignedBy(0), SignedBy(1)),
		Identities: principals,
	}

	test.RequireProtoEqual(t, p1, p2)
}

func TestAndClientPeerOrderer(t *testing.T) {
	p1, err := FromString("AND('A.client', 'B.peer')")
	require.NoError(t, err)

	principals := make([]*msp.MSPPrincipal, 0)

	principals = append(principals, &msp.MSPPrincipal{
		PrincipalClassification: msp.MSPPrincipal_ROLE,
		Principal:               protoutil.MarshalOrPanic(&msp.MSPRole{Role: msp.MSPRole_CLIENT, MspIdentifier: "A"}),
	})

	principals = append(principals, &msp.MSPPrincipal{
		PrincipalClassification: msp.MSPPrincipal_ROLE,
		Principal:               protoutil.MarshalOrPanic(&msp.MSPRole{Role: msp.MSPRole_PEER, MspIdentifier: "B"}),
	})

	p2 := &common.SignaturePolicyEnvelope{
		Version:    0,
		Rule:       And(SignedBy(0), SignedBy(1)),
		Identities: principals,
	}

	test.RequireProtoEqual(t, p1, p2)
}

func TestOr(t *testing.T) {
	p1, err := FromString("OR('A.member', 'B.member')")
	require.NoError(t, err)

	principals := make([]*msp.MSPPrincipal, 0)

	principals = append(principals, &msp.MSPPrincipal{
		PrincipalClassification: msp.MSPPrincipal_ROLE,
		Principal:               protoutil.MarshalOrPanic(&msp.MSPRole{Role: msp.MSPRole_MEMBER, MspIdentifier: "A"}),
	})

	principals = append(principals, &msp.MSPPrincipal{
		PrincipalClassification: msp.MSPPrincipal_ROLE,
		Principal:               protoutil.MarshalOrPanic(&msp.MSPRole{Role: msp.MSPRole_MEMBER, MspIdentifier: "B"}),
	})

	p2 := &common.SignaturePolicyEnvelope{
		Version:    0,
		Rule:       Or(SignedBy(0), SignedBy(1)),
		Identities: principals,
	}

	test.RequireProtoEqual(t, p1, p2)
}

func TestComplex1(t *testing.T) {
	p1, err := FromString("OR('A.member', AND('B.member', 'C.member'))")
	require.NoError(t, err)

	principals := make([]*msp.MSPPrincipal, 0)

	principals = append(principals, &msp.MSPPrincipal{
		PrincipalClassification: msp.MSPPrincipal_ROLE,
		Principal:               protoutil.MarshalOrPanic(&msp.MSPRole{Role: msp.MSPRole_MEMBER, MspIdentifier: "B"}),
	})

	principals = append(principals, &msp.MSPPrincipal{
		PrincipalClassification: msp.MSPPrincipal_ROLE,
		Principal:               protoutil.MarshalOrPanic(&msp.MSPRole{Role: msp.MSPRole_MEMBER, MspIdentifier: "C"}),
	})

	principals = append(principals, &msp.MSPPrincipal{
		PrincipalClassification: msp.MSPPrincipal_ROLE,
		Principal:               protoutil.MarshalOrPanic(&msp.MSPRole{Role: msp.MSPRole_MEMBER, MspIdentifier: "A"}),
	})

	p2 := &common.SignaturePolicyEnvelope{
		Version:    0,
		Rule:       Or(SignedBy(2), And(SignedBy(0), SignedBy(1))),
		Identities: principals,
	}

	test.RequireProtoEqual(t, p1, p2)
}

func TestComplex2(t *testing.T) {
	p1, err := FromString("OR(AND('A.member', 'B.member'), OR('C.admin', 'D.member'))")
	require.NoError(t, err)

	principals := make([]*msp.MSPPrincipal, 0)

	principals = append(principals, &msp.MSPPrincipal{
		PrincipalClassification: msp.MSPPrincipal_ROLE,
		Principal:               protoutil.MarshalOrPanic(&msp.MSPRole{Role: msp.MSPRole_MEMBER, MspIdentifier: "A"}),
	})

	principals = append(principals, &msp.MSPPrincipal{
		PrincipalClassification: msp.MSPPrincipal_ROLE,
		Principal:               protoutil.MarshalOrPanic(&msp.MSPRole{Role: msp.MSPRole_MEMBER, MspIdentifier: "B"}),
	})

	principals = append(principals, &msp.MSPPrincipal{
		PrincipalClassification: msp.MSPPrincipal_ROLE,
		Principal:               protoutil.MarshalOrPanic(&msp.MSPRole{Role: msp.MSPRole_ADMIN, MspIdentifier: "C"}),
	})

	principals = append(principals, &msp.MSPPrincipal{
		PrincipalClassification: msp.MSPPrincipal_ROLE,
		Principal:               protoutil.MarshalOrPanic(&msp.MSPRole{Role: msp.MSPRole_MEMBER, MspIdentifier: "D"}),
	})

	p2 := &common.SignaturePolicyEnvelope{
		Version:    0,
		Rule:       Or(And(SignedBy(0), SignedBy(1)), Or(SignedBy(2), SignedBy(3))),
		Identities: principals,
	}

	test.RequireProtoEqual(t, p1, p2)
}

func TestMSPIDWIthSpecialChars(t *testing.T) {
	p1, err := FromString("OR('MSP.member', 'MSP.WITH.DOTS.member', 'MSP-WITH-DASHES.member')")
	require.NoError(t, err)

	principals := make([]*msp.MSPPrincipal, 0)

	principals = append(principals, &msp.MSPPrincipal{
		PrincipalClassification: msp.MSPPrincipal_ROLE,
		Principal: protoutil.MarshalOrPanic(&msp.MSPRole{
			Role:          msp.MSPRole_MEMBER,
			MspIdentifier: "MSP",
		}),
	})

	principals = append(principals, &msp.MSPPrincipal{
		PrincipalClassification: msp.MSPPrincipal_ROLE,
		Principal: protoutil.MarshalOrPanic(&msp.MSPRole{
			Role:          msp.MSPRole_MEMBER,
			MspIdentifier: "MSP.WITH.DOTS",
		}),
	})

	principals = append(principals, &msp.MSPPrincipal{
		PrincipalClassification: msp.MSPPrincipal_ROLE,
		Principal: protoutil.MarshalOrPanic(&msp.MSPRole{
			Role:          msp.MSPRole_MEMBER,
			MspIdentifier: "MSP-WITH-DASHES",
		}),
	})

	p2 := &common.SignaturePolicyEnvelope{
		Version:    0,
		Rule:       NOutOf(1, []*common.SignaturePolicy{SignedBy(0), SignedBy(1), SignedBy(2)}),
		Identities: principals,
	}

	test.RequireProtoEqual(t, p1, p2)
}

func TestBadStringsNoPanic(t *testing.T) {
	_, err := FromString("OR('A.member', Bmember)")
	require.ErrorContains(t, err, "cannot fetch Bmember")

	_, err = FromString("OR('A.member', 'Bmember')")
	require.ErrorContains(t, err, "unrecognized token 'Bmember' in policy string")

	_, err = FromString(`OR('A.member', '\'Bmember\'')`)
	require.ErrorContains(t, err, "unrecognized token ''Bmember'' in policy string")
}

func TestNodeOUs(t *testing.T) {
	p1, err := FromString("OR('A.peer', 'B.admin', 'C.orderer', 'D.client')")
	require.NoError(t, err)

	principals := make([]*msp.MSPPrincipal, 0)

	principals = append(principals, &msp.MSPPrincipal{
		PrincipalClassification: msp.MSPPrincipal_ROLE,
		Principal:               protoutil.MarshalOrPanic(&msp.MSPRole{Role: msp.MSPRole_PEER, MspIdentifier: "A"}),
	})

	principals = append(principals, &msp.MSPPrincipal{
		PrincipalClassification: msp.MSPPrincipal_ROLE,
		Principal:               protoutil.MarshalOrPanic(&msp.MSPRole{Role: msp.MSPRole_ADMIN, MspIdentifier: "B"}),
	})

	principals = append(principals, &msp.MSPPrincipal{
		PrincipalClassification: msp.MSPPrincipal_ROLE,
		Principal:               protoutil.MarshalOrPanic(&msp.MSPRole{Role: msp.MSPRole_ORDERER, MspIdentifier: "C"}),
	})

	principals = append(principals, &msp.MSPPrincipal{
		PrincipalClassification: msp.MSPPrincipal_ROLE,
		Principal:               protoutil.MarshalOrPanic(&msp.MSPRole{Role: msp.MSPRole_CLIENT, MspIdentifier: "D"}),
	})

	p2 := &common.SignaturePolicyEnvelope{
		Version:    0,
		Rule:       NOutOf(1, []*common.SignaturePolicy{SignedBy(0), SignedBy(1), SignedBy(2), SignedBy(3)}),
		Identities: principals,
	}

	test.RequireProtoEqual(t, p1, p2)
}

func TestOutOfNumIsString(t *testing.T) {
	p1, err := FromString("OutOf('1', 'A.member', 'B.member')")
	require.NoError(t, err)

	principals := make([]*msp.MSPPrincipal, 0)

	principals = append(principals, &msp.MSPPrincipal{
		PrincipalClassification: msp.MSPPrincipal_ROLE,
		Principal:               protoutil.MarshalOrPanic(&msp.MSPRole{Role: msp.MSPRole_MEMBER, MspIdentifier: "A"}),
	})

	principals = append(principals, &msp.MSPPrincipal{
		PrincipalClassification: msp.MSPPrincipal_ROLE,
		Principal:               protoutil.MarshalOrPanic(&msp.MSPRole{Role: msp.MSPRole_MEMBER, MspIdentifier: "B"}),
	})

	p2 := &common.SignaturePolicyEnvelope{
		Version:    0,
		Rule:       NOutOf(1, []*common.SignaturePolicy{SignedBy(0), SignedBy(1)}),
		Identities: principals,
	}

	test.RequireProtoEqual(t, p1, p2)
}

func TestOutOfErrorCase(t *testing.T) {
	p1, err1 := FromString("")
	require.Nil(t, p1)
	require.ErrorContains(t, err1, "unexpected token EOF")

	p2, err2 := FromString("OutOf(1)") // outof() if len(args)<2
	require.Nil(t, p2)
	require.ErrorContains(t, err2, "expected at least two arguments to NOutOf. Given 1")

	p2a, err2a := FromString("And()") // and() if len(args)<1
	require.Nil(t, p2a)
	require.ErrorContains(t, err2a, "at least one policy arguments expected, got 0")

	p2b, err2b := FromString("Or()") // or() if len(args)<1
	require.Nil(t, p2b)
	require.ErrorContains(t, err2b, "at least one policy arguments expected, got 0")

	p3, err3 := FromString("OutOf(true, 'A.member')") // outof() 1st arg is non of float, int, string
	require.Nil(t, p3)
	require.ErrorContains(t, err3, "unrecognized type, expected a number, got bool")

	p4, err4 := FromString("OutOf(1, 2)") // oufof() switch default. 2nd arg is not string.
	require.Nil(t, p4)
	require.ErrorContains(t, err4, "unrecognized type, expected a principal or a policy, got int")

	p5, err5 := FromString("OutOf(1, 'true')") // switch default
	require.Nil(t, p5)
	require.ErrorContains(t, err5, "unrecognized token 'true' in policy string")

	p6, err6 := FromString(`OutOf('\'\\\'A\\\'\'', 'B.member')`) // switch default
	require.Nil(t, p6)
	require.ErrorContains(t, err6, "unrecognized type, expected a number, got string")

	p7, err7 := FromString(`OutOf(1, '\'1\'')`) // switch default
	require.Nil(t, p7)
	require.ErrorContains(t, err7, "unrecognized token ''1'' in policy string")

	p8, err8 := FromString(`''`)
	require.Nil(t, p8)
	require.ErrorContains(t, err8, "invalid policy string")

	p9, err9 := FromString(`'\'\''`)
	require.Nil(t, p9)
	require.ErrorContains(t, err9, "invalid policy string")
}

func TestBadStringBeforeFAB11404_ThisCanDeleteAfterFAB11404HasMerged(t *testing.T) {
	s1 := "1" // ineger in string
	p1, err1 := FromString(s1)
	require.Nil(t, p1)
	require.EqualError(t, err1, `invalid policy string '1'`)

	s2 := "'1'" // quoted ineger in string
	p2, err2 := FromString(s2)
	require.Nil(t, p2)
	require.EqualError(t, err2, `invalid policy string ''1''`)

	s3 := `'\'1\''` // nested quoted ineger in string
	p3, err3 := FromString(s3)
	require.Nil(t, p3)
	require.EqualError(t, err3, `invalid policy string ''\'1\'''`)
}

func TestSecondPassBoundaryCheck(t *testing.T) {
	// Check lower boundary
	// Prohibit t<0
	p0, err0 := FromString("OutOf(-1, 'A.member', 'B.member')")
	require.Nil(t, p0)
	require.ErrorContains(t, err0, "invalid t-out-of-n predicate, t -1, n 2")

	// Permit t==0 : always satisfied policy
	// There is no clear usecase of t=0, but somebody may already use it, so we don't treat as an error.
	p1, err1 := FromString("OutOf(0, 'A.member', 'B.member')")
	require.NoError(t, err1)
	principals := make([]*msp.MSPPrincipal, 0)
	principals = append(principals, &msp.MSPPrincipal{
		PrincipalClassification: msp.MSPPrincipal_ROLE,
		Principal:               protoutil.MarshalOrPanic(&msp.MSPRole{Role: msp.MSPRole_MEMBER, MspIdentifier: "A"}),
	})
	principals = append(principals, &msp.MSPPrincipal{
		PrincipalClassification: msp.MSPPrincipal_ROLE,
		Principal:               protoutil.MarshalOrPanic(&msp.MSPRole{Role: msp.MSPRole_MEMBER, MspIdentifier: "B"}),
	})
	expected1 := &common.SignaturePolicyEnvelope{
		Version:    0,
		Rule:       NOutOf(0, []*common.SignaturePolicy{SignedBy(0), SignedBy(1)}),
		Identities: principals,
	}
	test.RequireProtoEqual(t, expected1, p1)

	// Check upper boundary
	// Permit t==n+1 : never satisfied policy
	// Usecase: To create immutable ledger key
	p2, err2 := FromString("OutOf(3, 'A.member', 'B.member')")
	require.NoError(t, err2)
	expected2 := &common.SignaturePolicyEnvelope{
		Version:    0,
		Rule:       NOutOf(3, []*common.SignaturePolicy{SignedBy(0), SignedBy(1)}),
		Identities: principals,
	}
	test.RequireProtoEqual(t, expected2, p2)

	// Prohibit t>n + 1
	p3, err3 := FromString("OutOf(4, 'A.member', 'B.member')")
	require.Nil(t, p3)
	require.ErrorContains(t, err3, "invalid t-out-of-n predicate, t 4, n 2")
}

func TestPrincipalDeduplication(t *testing.T) {
	t.Parallel()

	// Test that duplicate principals are deduplicated
	// AND('A.member', OR('B.member', 'A.member')) should only have 2 identities, not 3
	p1, err := FromString("AND('A.member', OR('B.member', 'A.member'))")
	require.NoError(t, err)

	// Verify we only have 2 principals (A.member and B.member), not 3
	require.Len(t, p1.Identities, 2, "expected 2 unique principals after deduplication")

	principals := []*msp.MSPPrincipal{
		{ // B.member appears first during parsing (inside the nested OR), so it gets index 0
			PrincipalClassification: msp.MSPPrincipal_ROLE,
			Principal: protoutil.MarshalOrPanic(&msp.MSPRole{
				Role: msp.MSPRole_MEMBER, MspIdentifier: "B",
			}),
		},
		{ // A.member appears second, so it gets index 1
			PrincipalClassification: msp.MSPPrincipal_ROLE,
			Principal: protoutil.MarshalOrPanic(&msp.MSPRole{
				Role: msp.MSPRole_MEMBER, MspIdentifier: "A",
			}),
		},
	}

	// The expected structure:
	// - B.member is at index 0
	// - A.member is at index 1
	// - The rule is AND(SignedBy(1), OR(SignedBy(0), SignedBy(1)))
	// Note: A.member appears twice in the rule but only once in identities
	p2 := &common.SignaturePolicyEnvelope{
		Version: 0,
		Rule: NOutOf(2, []*common.SignaturePolicy{
			SignedBy(1), NOutOf(1, []*common.SignaturePolicy{
				SignedBy(0), SignedBy(1),
			}),
		}),
		Identities: principals,
	}

	test.RequireProtoEqual(t, p1, p2)
}
