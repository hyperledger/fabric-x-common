/*
Copyright IBM Corp. All Rights Reserved.

SPDX-License-Identifier: Apache-2.0
*/

package identity

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/hyperledger/fabric-x-common/api/protomsp"
	"github.com/hyperledger/fabric-x-common/common/cauthdsl"
	"github.com/hyperledger/fabric-x-common/msp"
)

func TestToSerializedIdentity(t *testing.T) {
	t.Parallel()

	const testMspID = "Org1MSP"
	const testCertID = "cert-001"
	testCertBytes := []byte("dummy-certificate-bytes")

	expectedSerialized, _ := msp.NewSerializedIdentity(testMspID, testCertBytes)

	validMockIdentity := &cauthdsl.MockIdentity{
		MspID:   testMspID,
		IDBytes: testCertBytes,
	}

	tests := []struct {
		name              string
		identity          *protomsp.Identity
		setupDeserializer func(*cauthdsl.MockIdentityDeserializer)
		expectedBytes     []byte
		expectedError     string
	}{
		{
			name: "Success: Direct Certificate",
			identity: &protomsp.Identity{
				MspId: testMspID,
				Creator: &protomsp.Identity_Certificate{
					Certificate: testCertBytes,
				},
			},
			setupDeserializer: func(*cauthdsl.MockIdentityDeserializer) {},
			expectedBytes:     expectedSerialized,
			expectedError:     "",
		},
		{
			name: "Failure: Empty Certificate in Direct Mode",
			identity: &protomsp.Identity{
				MspId: testMspID,
				Creator: &protomsp.Identity_Certificate{
					Certificate: nil,
				},
			},
			setupDeserializer: func(*cauthdsl.MockIdentityDeserializer) {},
			expectedBytes:     nil,
			expectedError:     "An empty certificate is provided for the identity",
		},
		{
			name: "Success: Certificate ID lookup",
			identity: &protomsp.Identity{
				MspId: testMspID,
				Creator: &protomsp.Identity_CertificateId{
					CertificateId: testCertID,
				},
			},
			setupDeserializer: func(d *cauthdsl.MockIdentityDeserializer) {
				d.KnownIdentities = map[msp.IdentityIdentifier]msp.Identity{
					{Mspid: testMspID, Id: testCertID}: validMockIdentity,
				}
			},
			expectedBytes: expectedSerialized,
			expectedError: "",
		},
		{
			name: "Failure: Empty Certificate ID string",
			identity: &protomsp.Identity{
				MspId: testMspID,
				Creator: &protomsp.Identity_CertificateId{
					CertificateId: "",
				},
			},
			setupDeserializer: func(*cauthdsl.MockIdentityDeserializer) {},
			expectedBytes:     nil,
			expectedError:     "An empty certificate ID is provided for the identity",
		},
		{
			name: "Failure: Certificate ID not found in Deserializer",
			identity: &protomsp.Identity{
				MspId: testMspID,
				Creator: &protomsp.Identity_CertificateId{
					CertificateId: "unknown-id",
				},
			},
			setupDeserializer: func(d *cauthdsl.MockIdentityDeserializer) {
				d.KnownIdentities = map[msp.IdentityIdentifier]msp.Identity{}
			},
			expectedBytes: nil,
			expectedError: "Invalid certificate identity: unknown-id",
		},
		{
			name: "Failure: Unknown Creator Type (nil)",
			identity: &protomsp.Identity{
				MspId:   testMspID,
				Creator: nil, // This falls into the default case
			},
			setupDeserializer: func(*cauthdsl.MockIdentityDeserializer) {},
			expectedBytes:     nil,
			expectedError:     "unknown creator type",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			mockDeserializer := &cauthdsl.MockIdentityDeserializer{}
			if tt.setupDeserializer != nil {
				tt.setupDeserializer(mockDeserializer)
			}

			actualBytes, err := ToSerializedIdentity(tt.identity, mockDeserializer)

			if tt.expectedError != "" {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.expectedError)
				assert.Nil(t, actualBytes)
			} else {
				require.NoError(t, err)
				assert.Equal(t, tt.expectedBytes, actualBytes)
			}
		})
	}
}
