/*
Copyright IBM Corp. All Rights Reserved.

SPDX-License-Identifier: Apache-2.0
*/

package applicationpb

// NewIdentityWithCertificate creates a applicationpb.Identity with the certificate.
func NewIdentityWithCertificate(mspID string, certificate []byte) *Identity {
	return &Identity{
		MspId:   mspID,
		Creator: &Identity_Certificate{Certificate: certificate},
	}
}

// NewIdentityWithCertificateID creates a applicationpb.Identity with the certificateID.
func NewIdentityWithCertificateID(mspID, certificateID string) *Identity {
	return &Identity{
		MspId:   mspID,
		Creator: &Identity_CertificateId{CertificateId: certificateID},
	}
}
