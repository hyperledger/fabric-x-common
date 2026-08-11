/*
Copyright IBM Corp. All Rights Reserved.

SPDX-License-Identifier: Apache-2.0
*/

package policydsl

import (
	"encoding/hex"
	"fmt"
	"strings"

	cb "github.com/hyperledger/fabric-protos-go-apiv2/common"
	mb "github.com/hyperledger/fabric-protos-go-apiv2/msp"
	"google.golang.org/protobuf/proto"
)

// renderMode selects how ToString and ToDisplayString handle a construct that has no DSL
// representation: strictMode fails the render, lenientMode substitutes a placeholder.
type renderMode int

const (
	strictMode renderMode = iota
	lenientMode
)

// principalPlaceholderHexLen bounds how many bytes of a principal are hex-dumped into a
// placeholder: an IDENTITY principal carries a full certificate, so dumping it untruncated
// would put a couple of KB of hex into the rendered string.
const principalPlaceholderHexLen = 8

// ToString renders an envelope as a policy DSL expression accepted by FromString. It returns
// an error if any part of the policy has no DSL representation.
//
// Example: the envelope produced by FromString("OR('Org1MSP.member', 'Org2MSP.member')")
// renders back to "OR('Org1MSP.member', 'Org2MSP.member')".
func ToString(env *cb.SignaturePolicyEnvelope) (string, error) {
	return renderRule(env.GetRule(), env.GetIdentities(), strictMode)
}

// ToDisplayString renders an envelope for human consumption, substituting readable
// placeholders for anything with no DSL representation. It never fails.
//
// Example: an envelope built with Envelope(SignedBy(0), [][]byte{identityBytes}) — a single
// IDENTITY principal, which has no DSL syntax — renders to "<identity:0>" instead of erroring.
func ToDisplayString(env *cb.SignaturePolicyEnvelope) string {
	rendered, err := renderRule(env.GetRule(), env.GetIdentities(), lenientMode)
	if err != nil {
		// renderRule never errors in lenientMode; this is unreachable.
		return err.Error()
	}
	return rendered
}

// renderRule renders a single SignaturePolicy node, dispatching on its oneof type: an NOutOf
// node (e.g. NOutOf{N: 1, Rules: [SignedBy(0), SignedBy(1)]}) renders via renderNOutOf (e.g.
// to "OR('Org1MSP.member', 'Org2MSP.member')"), a SignedBy node (e.g. SignedBy(0)) renders via
// renderSignedBy (e.g. to "'Org1MSP.member'"), and anything else (there is no third oneof case
// today) renders as "<unknown-rule>" in lenientMode or errors in strictMode.
func renderRule(rule *cb.SignaturePolicy, identities []*mb.MSPPrincipal, mode renderMode) (string, error) {
	switch t := rule.GetType().(type) {
	case *cb.SignaturePolicy_NOutOf_:
		return renderNOutOf(t.NOutOf, identities, mode)
	case *cb.SignaturePolicy_SignedBy:
		return renderSignedBy(t.SignedBy, identities, mode)
	default:
		if mode == strictMode {
			return "", fmt.Errorf("policy rule has no DSL representation: %T", t)
		}
		return "<unknown-rule>", nil
	}
}

// renderNOutOf renders an NOutOf node as one of the three DSL gates, picking the gate from N
// relative to len(Rules):
//   - no rules, e.g. NOutOf{N: 0, Rules: []} (AcceptAllPolicy) -> "OutOf(0)"
//   - N == 1, e.g. NOutOf{N: 1, Rules: [SignedBy(0), SignedBy(1)]} -> "OR('A.member', 'B.member')"
//   - N == len(Rules), e.g. NOutOf{N: 2, Rules: [SignedBy(0), SignedBy(1)]} -> "AND('A.member', 'B.member')"
//   - otherwise, e.g. NOutOf{N: 2, Rules: [SignedBy(0), SignedBy(1), SignedBy(2)]} ->
//     "OutOf(2, 'A.member', 'B.member', 'C.member')"
//
// N == 1 is checked before N == len(Rules) so a single-rule NOutOf renders as "OR(...)" rather
// than "AND(...)"; both parse back to the identical envelope, so either spelling is faithful.
func renderNOutOf(nOutOf *cb.SignaturePolicy_NOutOf, identities []*mb.MSPPrincipal, mode renderMode) (string, error) {
	n := nOutOf.GetN()
	rules := nOutOf.GetRules()

	if len(rules) == 0 {
		return fmt.Sprintf("%s(%d)", GateOutOfDisplay, n), nil
	}

	rendered, err := renderAll(rules, identities, mode)
	if err != nil {
		return "", err
	}

	switch {
	case n == 1:
		return fmt.Sprintf("%s(%s)", GateOrDisplay, strings.Join(rendered, ", ")), nil
	case int(n) == len(rules):
		return fmt.Sprintf("%s(%s)", GateAndDisplay, strings.Join(rendered, ", ")), nil
	default:
		return fmt.Sprintf("%s(%d, %s)", GateOutOfDisplay, n, strings.Join(rendered, ", ")), nil
	}
}

// renderAll renders each rule in rules independently and returns the rendered strings in the
// same order, e.g. [SignedBy(0), SignedBy(1)] -> []string{"'Org1MSP.member'", "'Org2MSP.member'"}.
// It stops at the first error, so a single unrenderable rule fails the whole slice in strictMode.
func renderAll(rules []*cb.SignaturePolicy, identities []*mb.MSPPrincipal, mode renderMode) ([]string, error) {
	rendered := make([]string, len(rules))
	for i, rule := range rules {
		r, err := renderRule(rule, identities, mode)
		if err != nil {
			return nil, err
		}
		rendered[i] = r
	}
	return rendered, nil
}

// renderSignedBy resolves a SignedBy index into the corresponding identities entry and renders
// that principal, e.g. SignedBy(0) against identities[0] = {MspIdentifier: "Org1MSP",
// Role: MEMBER} -> "'Org1MSP.member'". An index outside the identities slice (a malformed
// envelope) renders as "<principal[N]>" in lenientMode or errors in strictMode.
func renderSignedBy(index int32, identities []*mb.MSPPrincipal, mode renderMode) (string, error) {
	if index < 0 || int(index) >= len(identities) {
		if mode == strictMode {
			return "", fmt.Errorf("signed_by index %d out of range of %d identities", index, len(identities))
		}
		return fmt.Sprintf("<principal[%d]>", index), nil
	}
	return renderPrincipal(identities[index], index, mode)
}

// renderPrincipal renders an MSPPrincipal, dispatching on its PrincipalClassification:
//   - ROLE, e.g. {MspIdentifier: "Org1MSP", Role: MEMBER} -> "'Org1MSP.member'" (renderRolePrincipal)
//   - ORGANIZATION_UNIT, e.g. {MspIdentifier: "Org1MSP", OrganizationalUnitIdentifier: "eng"}
//     -> "'Org1MSP.eng'" (renderOUPrincipal)
//   - IDENTITY, which carries a full serialized identity with no DSL syntax -> "<identity:N>"
//     in lenientMode (N is the identities index) or an error in strictMode
//   - ANONYMITY / COMBINED (no DSL syntax either) -> "<principal:a1b2c3d4>" in lenientMode
//     (hex-truncated raw bytes) or an error in strictMode
func renderPrincipal(principal *mb.MSPPrincipal, index int32, mode renderMode) (string, error) {
	switch principal.GetPrincipalClassification() {
	case mb.MSPPrincipal_ROLE:
		return renderRolePrincipal(principal, mode)
	case mb.MSPPrincipal_ORGANIZATION_UNIT:
		return renderOUPrincipal(principal, mode)
	case mb.MSPPrincipal_IDENTITY:
		if mode == strictMode {
			return "", fmt.Errorf("identity principal at index %d has no DSL representation", index)
		}
		return fmt.Sprintf("<identity:%d>", index), nil
	default:
		if mode == strictMode {
			return "", fmt.Errorf(
				"principal classification %s has no DSL representation", principal.GetPrincipalClassification(),
			)
		}
		return placeholderPrincipal(principal.GetPrincipal()), nil
	}
}

// renderRolePrincipal unmarshals a ROLE principal's bytes into an MSPRole and renders it as
// 'mspID.role', lowercasing the role, e.g. MSPRole{MspIdentifier: "Org1MSP", Role: ADMIN} ->
// "'Org1MSP.admin'". Bytes that fail to unmarshal (corrupt data, not a client bug) render as
// a hex placeholder in lenientMode or error in strictMode.
func renderRolePrincipal(principal *mb.MSPPrincipal, mode renderMode) (string, error) {
	role := &mb.MSPRole{}
	if err := proto.Unmarshal(principal.GetPrincipal(), role); err != nil {
		if mode == strictMode {
			return "", fmt.Errorf("could not unmarshal MSPRole: %w", err)
		}
		return placeholderPrincipal(principal.GetPrincipal()), nil
	}
	return fmt.Sprintf("'%s.%s'", role.GetMspIdentifier(), strings.ToLower(role.GetRole().String())), nil
}

// renderOUPrincipal unmarshals an ORGANIZATION_UNIT principal's bytes into an OrganizationUnit
// and renders it as 'mspID.ouID', e.g. OrganizationUnit{MspIdentifier: "Org1MSP",
// OrganizationalUnitIdentifier: "engineering"} -> "'Org1MSP.engineering'". Bytes that fail to
// unmarshal render as a hex placeholder in lenientMode or error in strictMode.
func renderOUPrincipal(principal *mb.MSPPrincipal, mode renderMode) (string, error) {
	ou := &mb.OrganizationUnit{}
	if err := proto.Unmarshal(principal.GetPrincipal(), ou); err != nil {
		if mode == strictMode {
			return "", fmt.Errorf("could not unmarshal OrganizationUnit: %w", err)
		}
		return placeholderPrincipal(principal.GetPrincipal()), nil
	}
	return fmt.Sprintf("'%s.%s'", ou.GetMspIdentifier(), ou.GetOrganizationalUnitIdentifier()), nil
}

// placeholderPrincipal renders raw principal bytes as a hex placeholder, truncated to
// principalPlaceholderHexLen bytes, e.g. []byte("not an MSPRole") -> "<principal:6e6f7420616e204d>".
// Truncation matters for IDENTITY principals, which carry a full certificate: dumping it
// untruncated would put a couple of KB of hex into the rendered string.
func placeholderPrincipal(raw []byte) string {
	n := min(len(raw), principalPlaceholderHexLen)
	return "<principal:" + hex.EncodeToString(raw[:n]) + ">"
}

// Gate*Display are the canonical, human-readable gate spellings used by ToString and
// ToDisplayString. They deliberately do not reuse GateAnd/GateOr/GateOutOf, which are the
// lowercase parser tokens ("and"/"or"/"outof") rather than the display spelling; Visit
// lowercases gate identifiers during AST traversal, so this casing still round-trips through
// FromString.
const (
	GateAndDisplay   = "AND"
	GateOrDisplay    = "OR"
	GateOutOfDisplay = "OutOf"
)
