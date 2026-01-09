/*
Copyright IBM Corp. All Rights Reserved.

SPDX-License-Identifier: Apache-2.0
*/

package mocks

import (
	"bytes"
	"errors"
	"fmt"
	"time"

	mspproto "github.com/hyperledger/fabric-protos-go-apiv2/msp"
	"google.golang.org/protobuf/proto"

	"github.com/hyperledger/fabric-x-common/api/applicationpb"
	"github.com/hyperledger/fabric-x-common/common/policies"
	"github.com/hyperledger/fabric-x-common/msp"
	"github.com/hyperledger/fabric-x-common/protoutil"
)

type MockChannelPolicyManagerGetter struct {
	Managers map[string]policies.Manager
}

func (c *MockChannelPolicyManagerGetter) Manager(channelID string) policies.Manager {
	return c.Managers[channelID]
}

type MockChannelPolicyManager struct {
	MockPolicy policies.Policy
}

func (m *MockChannelPolicyManager) GetPolicy(id string) (policies.Policy, bool) {
	return m.MockPolicy, true
}

func (m *MockChannelPolicyManager) Manager(path []string) (policies.Manager, bool) {
	panic("Not implemented")
}

type MockPolicy struct {
	Deserializer msp.IdentityDeserializer
}

// EvaluateSignedData takes a set of SignedData and evaluates whether this set of signatures satisfies the policy
func (m *MockPolicy) EvaluateSignedData(signatureSet []*protoutil.SignedData) error {
	fmt.Printf("Evaluate [%s], [% x], [% x]\n", signatureSet[0].Identity.String(),
		string(signatureSet[0].Data), string(signatureSet[0].Signature))
	identity, err := m.Deserializer.DeserializeIdentity(signatureSet[0].Identity)
	if err != nil {
		return err
	}

	return identity.Verify(signatureSet[0].Data, signatureSet[0].Signature)
}

// EvaluateIdentities takes an array of identities and evaluates whether
// they satisfy the policy
func (m *MockPolicy) EvaluateIdentities(identities []msp.Identity) error {
	return nil
}

type MockIdentityDeserializer struct {
	Identity *applicationpb.Identity
	Msg      []byte
}

// DeserializeIdentity converts a proto identity into a msp.Identity.
//
//nolint:ireturn
func (d *MockIdentityDeserializer) DeserializeIdentity(identity *applicationpb.Identity) (msp.Identity, error) {
	fmt.Printf("[DeserializeIdentity] id : [%s], [%s]\n", identity.String(), d.Identity.String())
	if proto.Equal(d.Identity, identity) {
		fmt.Printf("GOT : [%s], [%s]\n", identity.String(), d.Identity.String())
		return &MockIdentity{identity: d.Identity, msg: d.Msg}, nil
	}

	return nil, errors.New("Invalid Identity")
}

// GetKnownDeserializedIdentity returns a known identity matching the given IdentityIdentifier.
func (*MockIdentityDeserializer) GetKnownDeserializedIdentity( //nolint:ireturn //Identity is an interface.
	msp.IdentityIdentifier,
) msp.Identity {
	return nil
}

// IsWellFormed checks whether the certificate provided in the identity is valid.
func (*MockIdentityDeserializer) IsWellFormed(_ *applicationpb.Identity) error {
	return nil
}

type MockIdentity struct {
	identity *applicationpb.Identity
	msg      []byte
}

func (id *MockIdentity) Anonymous() bool {
	panic("implement me")
}

func (id *MockIdentity) SatisfiesPrincipal(p *mspproto.MSPPrincipal) error {
	fmt.Printf("[SatisfiesPrincipal] id : [%s], [%s]\n", id.identity.String(), string(p.Principal))
	idBytes, err := proto.Marshal(id.identity)
	if err != nil {
		return err
	}
	if !bytes.Equal(idBytes, p.Principal) {
		return fmt.Errorf("Different identities [% x]!=[% x]", id.identity, p.Principal)
	}
	return nil
}

func (id *MockIdentity) ExpiresAt() time.Time {
	return time.Time{}
}

func (id *MockIdentity) GetIdentifier() *msp.IdentityIdentifier {
	return &msp.IdentityIdentifier{Mspid: "mock", Id: "mock"}
}

func (id *MockIdentity) GetMSPIdentifier() string {
	return "mock"
}

func (id *MockIdentity) Validate() error {
	return nil
}

func (id *MockIdentity) GetOrganizationalUnits() []*msp.OUIdentifier {
	return nil
}

func (id *MockIdentity) Verify(msg []byte, sig []byte) error {
	fmt.Printf("VERIFY [% x], [% x], [% x]\n", string(id.msg), string(msg), string(sig))
	if bytes.Equal(id.msg, msg) {
		if bytes.Equal(msg, sig) {
			return nil
		}
	}

	return errors.New("Invalid Signature")
}

// SerializeWithIDOfCert converts identity to bytes.
func (*MockIdentity) SerializeWithIDOfCert() ([]byte, error) {
	return []byte("cert"), nil
}

// Serialize converts identity to bytes.
func (*MockIdentity) Serialize() ([]byte, error) {
	return []byte("cert"), nil
}

// GetCertificatePEM return the certificate in PEM format.
func (*MockIdentity) GetCertificatePEM() ([]byte, error) {
	return []byte("cert"), nil
}

type MockMSPPrincipalGetter struct {
	Principal []byte
}

func (m *MockMSPPrincipalGetter) Get(role string) (*mspproto.MSPPrincipal, error) {
	return &mspproto.MSPPrincipal{Principal: m.Principal}, nil
}
