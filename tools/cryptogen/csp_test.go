/*
Copyright IBM Corp. All Rights Reserved.

SPDX-License-Identifier: Apache-2.0
*/

package cryptogen

import (
	"crypto/ecdsa"
	"crypto/ed25519"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/asn1"
	"encoding/pem"
	"math/big"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestLoadPrivateKey(t *testing.T) {
	t.Parallel()
	testDir := t.TempDir()
	priv, err := GeneratePrivateKey(testDir, ED25519)
	require.NoError(t, err, "failed to generate private key")
	pkFile := filepath.Join(testDir, "priv_sk")
	require.FileExists(t, pkFile, "Expected to find private key file")
	loadedPriv, err := LoadPrivateKey(testDir)
	require.NoError(t, err, "Failed to load private key")
	require.NotNil(t, loadedPriv, "Should have returned an *ecdsa.PrivateKey")
	require.Equal(t, priv, loadedPriv, "Expected private keys to match")
}

func TestLoadPrivateKey_BadPEM(t *testing.T) {
	t.Parallel()

	rsaKey, err := rsa.GenerateKey(rand.Reader, 1024)
	require.NoError(t, err, "failed to generate RSA key")

	pkcs8Encoded, err := x509.MarshalPKCS8PrivateKey(rsaKey)
	require.NoError(t, err, "failed to PKCS8 encode RSA private key")
	pkcs8RSAPem := pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: pkcs8Encoded})

	pkcs1Encoded := x509.MarshalPKCS1PrivateKey(rsaKey)
	require.NotNil(t, pkcs1Encoded, "failed to PKCS1 encode RSA private key")
	pkcs1RSAPem := pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: pkcs1Encoded})

	for _, test := range []struct {
		name   string
		data   []byte
		errMsg string
	}{
		{
			name:   "not pem encoded",
			data:   []byte("wrong_encoding"),
			errMsg: "bytes are not PEM encoded",
		},
		{
			name:   "not EC key",
			data:   pkcs8RSAPem,
			errMsg: "PEM bytes do not contain an ECDSA nor ed25519 private key",
		},
		{
			name:   "not PKCS8 encoded",
			data:   pkcs1RSAPem,
			errMsg: "PEM bytes are not PKCS8 encoded",
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			testDir := t.TempDir()
			badPEMFile := filepath.Join(testDir, "badpem_sk")

			writeErr := os.WriteFile(
				badPEMFile,
				test.data,
				0o755,
			)
			require.NoError(t, writeErr, "failed to write to wrong encoding file")

			_, err = LoadPrivateKey(badPEMFile)
			require.ErrorContains(t, err, test.errMsg)
		})
	}
}

func TestGeneratePrivateKey(t *testing.T) {
	t.Parallel()
	testDir := t.TempDir()

	expectedFile := filepath.Join(testDir, "priv_sk")
	priv, err := GeneratePrivateKey(testDir, ECDSA)
	require.NoError(t, err, "Failed to generate private key")
	require.NotNil(t, priv, "Should have returned an *ecdsa.Key")
	require.FileExists(t, expectedFile, "Expected to find private key file")

	_, err = GeneratePrivateKey("notExist", ECDSA)
	require.Contains(t, err.Error(), "no such file or directory")
}

func TestECDSASigner(t *testing.T) {
	t.Parallel()
	priv, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	require.NoError(t, err, "Failed to generate private key")

	signer := ECDSASigner{
		PrivateKey: priv,
	}
	require.Equal(t, priv.Public(), signer.Public().(*ecdsa.PublicKey))
	digest := []byte{1}
	sig, err := signer.Sign(rand.Reader, digest, nil)
	require.NoError(t, err, "Failed to create signature")

	// unmarshal signature
	ecdsaSig := &ECDSASignature{}
	_, err = asn1.Unmarshal(sig, ecdsaSig)
	require.NoError(t, err, "Failed to unmarshal signature")
	// s should not be greater than half order of curve
	halfOrder := new(big.Int).Div(priv.PublicKey.Curve.Params().N, big.NewInt(2))

	require.NotEqual(t, 1, ecdsaSig.S.Cmp(halfOrder), "Expected signature with Low S")

	// ensure signature is valid by using standard verify function
	ok := ecdsa.Verify(&priv.PublicKey, digest, ecdsaSig.R, ecdsaSig.S)
	require.True(t, ok, "Expected valid signature")
}

func TestED25519Signer(t *testing.T) {
	t.Parallel()
	pub, priv, err := ed25519.GenerateKey(rand.Reader)
	require.NoError(t, err, "Failed to generate private key")

	signer := ED25519Signer{
		PrivateKey: priv,
	}
	require.Equal(t, priv.Public(), signer.Public().(ed25519.PublicKey))
	msg := []byte{1}
	sig, err := signer.Sign(rand.Reader, msg, nil)
	require.NoError(t, err, "Failed to create signature")

	// ensure signature is valid by using standard verify function
	ok := ed25519.Verify(pub, msg, sig)
	require.True(t, ok, "Expected valid signature")
}
