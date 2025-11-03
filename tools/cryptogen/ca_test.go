/*
Copyright IBM Corp. All Rights Reserved.

SPDX-License-Identifier: Apache-2.0
*/

package cryptogen

import (
	"crypto/ecdsa"
	"crypto/x509"
	"net"
	"os"
	"path"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

const (
	caTestCAName             = "root0"
	caTestCA2Name            = "root1"
	caTstCA3Name             = "root2"
	caTestName               = "cert0"
	caTestName2              = "cert1"
	caTestName3              = "cert2"
	caTestIP                 = "172.16.10.31"
	caTestCountry            = "US"
	caTestProvince           = "California"
	caTestLocality           = "San Francisco"
	caTestOrganizationalUnit = "Hyperledger Fabric"
	caTestStreetAddress      = "caTestStreetAddress"
	caTestPostalCode         = "123456"
)

func TestLoadCertificateECDSA(t *testing.T) {
	t.Parallel()
	testDir := t.TempDir()

	// generate private key
	certDir := path.Join(testDir, "certs")
	require.NoError(t, os.MkdirAll(certDir, 0o750))
	privGeneric, err := GeneratePrivateKey(certDir, ECDSA)
	require.NoError(t, err, "Failed to generate signed certificate")
	priv, ok := privGeneric.(*ecdsa.PrivateKey)
	require.True(t, ok)

	// create our CA
	caDir := path.Join(testDir, "ca")
	rootCA := defaultCA(t, caTstCA3Name, caDir)

	cert, err := rootCA.SignCertificate(certDir, caTestName3, SignCertParams{
		PublicKey:   &priv.PublicKey,
		KeyUsage:    x509.KeyUsageDigitalSignature | x509.KeyUsageKeyEncipherment,
		ExtKeyUsage: []x509.ExtKeyUsage{x509.ExtKeyUsageAny},
	})
	require.NoError(t, err, "Failed to generate signed certificate")
	// KeyUsage should be x509.KeyUsageDigitalSignature | x509.KeyUsageKeyEncipherment
	require.Equal(t, x509.KeyUsageDigitalSignature|x509.KeyUsageKeyEncipherment, cert.KeyUsage)
	require.Contains(t, cert.ExtKeyUsage, x509.ExtKeyUsageAny)

	loadedCert, err := LoadCertificate(certDir)
	require.NoError(t, err)
	require.NotNil(t, loadedCert, "Should load cert")
	require.Equal(t, cert.SerialNumber, loadedCert.SerialNumber, "Should have same serial number")
	require.Equal(t, cert.Subject.CommonName, loadedCert.Subject.CommonName, "Should have same CN")
}

func TestLoadCertificateECDSA_wrongEncoding(t *testing.T) {
	t.Parallel()
	testDir := t.TempDir()

	filename := path.Join(testDir, "wrong_encoding.pem")
	err := os.WriteFile(filename, []byte("wrong_encoding"), 0o644) // Wrong encoded cert
	require.NoErrorf(t, err, "failed to create file %s", filename)

	_, err = LoadCertificate(testDir)
	require.Error(t, err)
	require.ErrorContains(t, err, "bytes are not PEM encoded")
}

func TestLoadCertificateECDSA_empty_DER_cert(t *testing.T) {
	t.Parallel()
	testDir := t.TempDir()

	filename := path.Join(testDir, "empty.pem")
	emptyCert := "-----BEGIN CERTIFICATE-----\n-----END CERTIFICATE-----"
	err := os.WriteFile(filename, []byte(emptyCert), 0o644)
	require.NoErrorf(t, err, "failed to create file %s", filename)

	cert, err := LoadCertificate(testDir)
	require.Nil(t, cert)
	require.Error(t, err)
	require.ErrorContains(t, err, "wrong DER encoding")
}

func TestNewCA(t *testing.T) {
	t.Parallel()
	testDir := t.TempDir()

	caDir := filepath.Join(testDir, "ca")
	rootCA := defaultCA(t, caTestCAName, caDir)
	require.NotNil(t, rootCA.Signer,
		"rootCA.Signer should not be empty")
	require.IsType(t, &x509.Certificate{}, rootCA.SignCert,
		"rootCA.SignCert should be type x509.Certificate")

	// check to make sure the root public key was stored
	pemFile := filepath.Join(caDir, caTestCAName+"-cert.pem")
	require.FileExists(t, pemFile)

	require.NotEmpty(t, rootCA.SignCert.Subject.Country, "country cannot be empty.")
	require.Equal(t, caTestCountry, rootCA.SignCert.Subject.Country[0], "Failed to match country")
	require.NotEmpty(t, rootCA.SignCert.Subject.Province, "province cannot be empty.")
	require.Equal(t, caTestProvince, rootCA.SignCert.Subject.Province[0], "Failed to match province")
	require.NotEmpty(t, rootCA.SignCert.Subject.Locality, "locality cannot be empty.")
	require.Equal(t, caTestLocality, rootCA.SignCert.Subject.Locality[0], "Failed to match locality")
	require.NotEmpty(t, rootCA.SignCert.Subject.OrganizationalUnit, "organizationalUnit cannot be empty.")
	require.Equal(t, caTestOrganizationalUnit, rootCA.SignCert.Subject.OrganizationalUnit[0],
		"Failed to match organizationalUnit")
	require.NotEmpty(t, rootCA.SignCert.Subject.StreetAddress, "streetAddress cannot be empty.")
	require.Equal(t, caTestStreetAddress, rootCA.SignCert.Subject.StreetAddress[0],
		"Failed to match streetAddress")
	require.NotEmpty(t, rootCA.SignCert.Subject.PostalCode, "postalCode cannot be empty.")
	require.Equal(t, caTestPostalCode, rootCA.SignCert.Subject.PostalCode[0], "Failed to match postalCode")
}

func TestGenerateSignCertificate(t *testing.T) {
	t.Parallel()
	testDir := t.TempDir()

	// generate private key
	certDir := path.Join(testDir, "certs")
	require.NoError(t, os.MkdirAll(certDir, 0o750))
	privGeneric, err := GeneratePrivateKey(certDir, ECDSA)
	require.NoError(t, err, "Failed to generate signed certificate")
	priv, ok := privGeneric.(*ecdsa.PrivateKey)
	require.True(t, ok)

	// create our CA
	caDir := filepath.Join(testDir, "ca")
	rootCA := defaultCA(t, caTestCA2Name, caDir)

	cert, err := rootCA.SignCertificate(certDir, caTestName, SignCertParams{
		PublicKey:   &priv.PublicKey,
		KeyUsage:    x509.KeyUsageDigitalSignature | x509.KeyUsageKeyEncipherment,
		ExtKeyUsage: []x509.ExtKeyUsage{x509.ExtKeyUsageAny},
	})
	require.NoError(t, err, "Failed to generate signed certificate")
	// KeyUsage should be x509.KeyUsageDigitalSignature | x509.KeyUsageKeyEncipherment
	require.Equal(t, x509.KeyUsageDigitalSignature|x509.KeyUsageKeyEncipherment,
		cert.KeyUsage)
	require.Contains(t, cert.ExtKeyUsage, x509.ExtKeyUsageAny)

	cert, err = rootCA.SignCertificate(certDir, caTestName, SignCertParams{
		KeyUsage:    x509.KeyUsageDigitalSignature,
		ExtKeyUsage: []x509.ExtKeyUsage{},
		PublicKey:   &priv.PublicKey,
	})
	require.NoError(t, err, "Failed to generate signed certificate")
	require.Empty(t, cert.ExtKeyUsage)

	// make sure ous are correctly set
	ous := []string{"TestOU", "PeerOU"}
	cert, err = rootCA.SignCertificate(certDir, caTestName, SignCertParams{
		OrgUnits:  ous,
		KeyUsage:  x509.KeyUsageDigitalSignature,
		PublicKey: &priv.PublicKey,
	})
	require.NoError(t, err)
	require.Contains(t, cert.Subject.OrganizationalUnit, ous[0])
	require.Contains(t, cert.Subject.OrganizationalUnit, ous[1])

	// make sure sans are correctly set
	sans := []string{caTestName2, caTestName3, caTestIP}
	cert, err = rootCA.SignCertificate(certDir, caTestName, SignCertParams{
		AlternateNames: sans,
		KeyUsage:       x509.KeyUsageDigitalSignature,
		ExtKeyUsage:    []x509.ExtKeyUsage{},
		PublicKey:      &priv.PublicKey,
	})
	require.NoError(t, err)
	require.Contains(t, cert.DNSNames, caTestName2)
	require.Contains(t, cert.DNSNames, caTestName3)
	require.Contains(t, cert.IPAddresses, net.ParseIP(caTestIP).To4())
	require.Len(t, cert.DNSNames, 2)

	// check to make sure the signed public key was stored
	pemFile := filepath.Join(certDir, caTestName+"-cert.pem")
	require.FileExists(t, pemFile)

	_, err = rootCA.SignCertificate(certDir, "empty/CA", SignCertParams{
		KeyUsage:    x509.KeyUsageKeyEncipherment,
		ExtKeyUsage: []x509.ExtKeyUsage{x509.ExtKeyUsageAny},
		PublicKey:   &priv.PublicKey,
	})
	require.Error(t, err, "Bad name should fail")

	// use an empty CA to test error path
	badCA := &CA{
		Name:     "badCA",
		SignCert: &x509.Certificate{},
	}
	_, err = badCA.SignCertificate(certDir, caTestName, SignCertParams{
		KeyUsage:    x509.KeyUsageKeyEncipherment,
		ExtKeyUsage: []x509.ExtKeyUsage{x509.ExtKeyUsageAny},
		PublicKey:   &ecdsa.PublicKey{},
	})
	require.Error(t, err, "Empty CA should not be able to sign")
}

func defaultCA(t *testing.T, name, caDir string) *CA {
	t.Helper()
	rootCA := CA{
		Organization:       name,
		Name:               name,
		Country:            caTestCountry,
		Province:           caTestProvince,
		Locality:           caTestLocality,
		OrganizationalUnit: caTestOrganizationalUnit,
		StreetAddress:      caTestStreetAddress,
		PostalCode:         caTestPostalCode,
		KeyAlgorithm:       ECDSA,
	}
	err := BuildCA(caDir, &rootCA)
	require.NoError(t, err, "Error generating CA")
	return &rootCA
}
