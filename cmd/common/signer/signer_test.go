/*
Copyright IBM Corp. All Rights Reserved.

SPDX-License-Identifier: Apache-2.0
*/

package signer

import (
	"crypto/ecdsa"
	"crypto/ed25519"
	"crypto/x509"
	"encoding/pem"
	"os"
	"path/filepath"
	"testing"

	"github.com/hyperledger/fabric-lib-go/bccsp/utils"
	"github.com/stretchr/testify/require"

	"github.com/hyperledger/fabric-x-common/common/util"
)

func TestEcdsaSigner(t *testing.T) {
	conf := Config{
		MSPID:        "SampleOrg",
		IdentityPath: filepath.Join("testdata", "signer", "cert.pem"),
		KeyPath:      filepath.Join("testdata", "signer", "8150cb2d09628ccc89727611ebb736189f6482747eff9b8aaaa27e9a382d2e93_sk"),
	}

	signer, err := NewSigner(conf)
	require.NoError(t, err)

	msg := []byte("foo")
	sig, err := signer.Sign(msg)
	require.NoError(t, err)

	r, s, err := utils.UnmarshalECDSASignature(sig)
	require.NoError(t, err)
	verify := ecdsa.Verify(&signer.key.(*ecdsa.PrivateKey).PublicKey, util.ComputeSHA256(msg), r, s)
	require.True(t, verify)
}

func TestEd25519Signer(t *testing.T) {
	conf := Config{
		MSPID:        "SampleOrg",
		IdentityPath: filepath.Join("testdata", "signer", "ed25519.pem"),
		KeyPath:      filepath.Join("testdata", "signer", "ed25519_sk"),
	}

	signer, err := NewSigner(conf)
	require.NoError(t, err)

	msg := []byte("foo")
	sig, err := signer.Sign(msg)
	require.NoError(t, err)

	require.NoError(t, err)
	verify := ed25519.Verify(signer.key.(ed25519.PrivateKey).Public().(ed25519.PublicKey), msg, sig)
	require.True(t, verify)
}

func TestSignerDifferentFormats(t *testing.T) {
	key := `-----BEGIN EC PRIVATE KEY-----
MHcCAQEEIOwCtOQIkowasuWoDQpXHgC547VHq+aBFaSyPOoV8mnGoAoGCCqGSM49
AwEHoUQDQgAEEsrroAkPez9reWvJukufUqyfouJjakrKuhNBYuclkldqsLZ/TO+w
ZsQXrlIqlmNalfYPX+NDDELqlpXQBeEqnA==
-----END EC PRIVATE KEY-----`

	pemBlock, _ := pem.Decode([]byte(key))
	require.NotNil(t, pemBlock)

	ecPK, err := x509.ParseECPrivateKey(pemBlock.Bytes)
	require.NoError(t, err)

	ec1, err := x509.MarshalECPrivateKey(ecPK)
	require.NoError(t, err)

	pkcs8, err := x509.MarshalPKCS8PrivateKey(ecPK)
	require.NoError(t, err)

	for _, testCase := range []struct {
		description string
		keyBytes    []byte
	}{
		{
			description: "EC1",
			keyBytes:    pem.EncodeToMemory(&pem.Block{Type: "EC Private Key", Bytes: ec1}),
		},
		{
			description: "PKCS8",
			keyBytes:    pem.EncodeToMemory(&pem.Block{Type: "Private Key", Bytes: pkcs8}),
		},
	} {
		t.Run(testCase.description, func(t *testing.T) {
			tmpFile, err := os.CreateTemp("", "key")
			require.NoError(t, err)

			defer os.Remove(tmpFile.Name())

			err = os.WriteFile(tmpFile.Name(), testCase.keyBytes, 0o600)
			require.NoError(t, err)

			signer, err := NewSigner(Config{
				MSPID:        "MSPID",
				IdentityPath: filepath.Join("testdata", "signer", "cert.pem"),
				KeyPath:      tmpFile.Name(),
			})

			require.NoError(t, err)
			require.NotNil(t, signer)
		})
	}
}

func TestSignerBadConfig(t *testing.T) {
	conf := Config{
		MSPID:        "SampleOrg",
		IdentityPath: filepath.Join("testdata", "signer", "non_existent_cert"),
	}

	signer, err := NewSigner(conf)
	require.EqualError(t, err, "open testdata/signer/non_existent_cert: no such file or directory")
	require.Nil(t, signer)

	conf = Config{
		MSPID:        "SampleOrg",
		IdentityPath: filepath.Join("testdata", "signer", "cert.pem"),
		KeyPath:      filepath.Join("testdata", "signer", "non_existent_cert"),
	}

	signer, err = NewSigner(conf)
	require.EqualError(t, err, "open testdata/signer/non_existent_cert: no such file or directory")
	require.Nil(t, signer)

	conf = Config{
		MSPID:        "SampleOrg",
		IdentityPath: filepath.Join("testdata", "signer", "cert.pem"),
		KeyPath:      filepath.Join("testdata", "signer", "broken_private_key"),
	}

	signer, err = NewSigner(conf)
	require.EqualError(t, err, "failed to decode PEM block from testdata/signer/broken_private_key")
	require.Nil(t, signer)

	conf = Config{
		MSPID:        "SampleOrg",
		IdentityPath: filepath.Join("testdata", "signer", "cert.pem"),
		KeyPath:      filepath.Join("testdata", "signer", "empty_private_key"),
	}

	signer, err = NewSigner(conf)
	require.EqualError(t, err, "failed to parse private key: x509: failed to parse EC private key: asn1: syntax error: sequence truncated")
	require.Nil(t, signer)

	conf = Config{
		MSPID:        "SampleOrg",
		IdentityPath: filepath.Join("testdata", "signer", "cert_invalid_PEM.pem"),
		KeyPath:      filepath.Join("testdata", "signer", ""),
	}

	signer, err = NewSigner(conf)
	require.EqualError(t, err, "enrollment certificate isn't a valid PEM block")
	require.Nil(t, signer)

	conf = Config{
		MSPID:        "SampleOrg",
		IdentityPath: filepath.Join("testdata", "signer", "cert_invalid_type.pem"),
		KeyPath:      filepath.Join("testdata", "signer", ""),
	}

	signer, err = NewSigner(conf)
	require.EqualError(t, err, "enrollment certificate should be a certificate, got a public key instead")
	require.Nil(t, signer)

	conf = Config{
		MSPID:        "SampleOrg",
		IdentityPath: filepath.Join("testdata", "signer", "cert_invalid_certificate.pem"),
		KeyPath:      filepath.Join("testdata", "signer", ""),
	}

	signer, err = NewSigner(conf)
	require.EqualError(t, err, "enrollment certificate is not a valid x509 certificate: x509: malformed certificate")
	require.Nil(t, signer)
}
