/*
Copyright IBM Corp. All Rights Reserved.

SPDX-License-Identifier: Apache-2.0
*/

package identity

import (
	"github.com/cockroachdb/errors"

	"github.com/hyperledger/fabric-x-common/api/protomsp"
	"github.com/hyperledger/fabric-x-common/msp"
)

// ToSerializedIdentity serilizes the protomsp.Identity to byte representation of msp.SerializedIdentity.
func ToSerializedIdentity(id *protomsp.Identity, d msp.IdentityDeserializer) ([]byte, error) {
	switch id.Creator.(type) {
	case *protomsp.Identity_Certificate:
		cert := id.GetCertificate()
		if cert == nil {
			return nil, errors.New("An empty certificate is provided for the identity")
		}
		return msp.NewSerializedIdentity(id.MspId, cert)
	case *protomsp.Identity_CertificateId:
		certID := id.GetCertificateId()
		if certID == "" {
			return nil, errors.New("An empty certificate ID is provided for the identity")
		}

		identity := d.GetKnownDeserializedIdentity(msp.IdentityIdentifier{
			Mspid: id.MspId,
			Id:    certID,
		})
		if identity == nil {
			return nil, errors.Newf("Invalid certificate identity: %s", certID)
		}

		return identity.Serialize()
	default:
		return nil, errors.New("unknown creator type")
	}
}
