/*
Copyright IBM Corp. All Rights Reserved.

SPDX-License-Identifier: Apache-2.0
*/

package cryptogen

import (
	"crypto"
	"crypto/ecdsa"
	"crypto/ed25519"
	"crypto/x509"
	"os"
	"path"

	"github.com/cockroachdb/errors"
	"go.yaml.in/yaml/v3"

	fabricmsp "github.com/hyperledger/fabric-x-common/msp"
)

// MspTree represents the MSP tree structure.
type MspTree struct {
	Root       string
	MSP        string
	TLS        string
	CaCerts    string
	TLSCaCerts string
	KeyStore   string
	AdminCerts string
	SignCerts  string
}

// NodeParameters are used as parameters for the generating methods.
type NodeParameters struct {
	SignCa    *CA
	TLSCa     *CA
	TLSSans   []string
	Name      string
	OU        string
	EnableOUs bool
	KeyAlg    string
}

// Directories.
const (
	MSPDir        = "msp"
	TLSDir        = "tls"
	CACertsDir    = "cacerts"
	TLSCaCertsDir = "tlscacerts"
	KeyStoreDir   = "keystore"
	AdminCertsDir = "admincerts"
	SignCertsDir  = "signcerts"
)

// Files.
const (
	ConfigFile   = "config.yaml"
	CaCertFile   = "ca.crt"
	ServerPrefix = "server"
	ClientPrefix = "client"
)

// Organizational units.
const (
	AdminOU   = "admin"
	ClientOU  = "client"
	OrdererOU = "orderer"
	PeerOU    = "peer"
)

// NewMspTree creates a new MSP tree structure.
func NewMspTree(root string) *MspTree {
	mspDir := path.Join(root, MSPDir)
	return &MspTree{
		Root:       root,
		MSP:        mspDir,
		TLS:        path.Join(root, TLSDir),
		CaCerts:    path.Join(mspDir, CACertsDir),
		TLSCaCerts: path.Join(mspDir, TLSCaCertsDir),
		KeyStore:   path.Join(mspDir, KeyStoreDir),
		AdminCerts: path.Join(mspDir, AdminCertsDir),
		SignCerts:  path.Join(mspDir, SignCertsDir),
	}
}

// IsExist returns true if the root directory already exists.
func (t *MspTree) IsExist() bool {
	_, err := os.Stat(t.Root)
	return !os.IsNotExist(err)
}

// GenerateLocalMSP generates a local MSP.
func (t *MspTree) GenerateLocalMSP(p NodeParameters) error {
	err := t.generateMsp(p)
	if err != nil {
		return err
	}
	return t.generateTLS(p)
}

// GenerateVerifyingMSP generates a verifying MSP.
func (t *MspTree) GenerateVerifyingMSP(p NodeParameters) error {
	defer func() {
		// We remove the local MSP folders.
		for _, dir := range []string{t.KeyStore, t.SignCerts} {
			_ = os.RemoveAll(dir)
		}
	}()
	p.Name = p.SignCa.Name
	return t.generateMsp(p)
}

// generateMsp generates a generic MSP.
func (t *MspTree) generateMsp(p NodeParameters) error {
	err := createAllFolders([]string{t.CaCerts, t.TLSCaCerts, t.AdminCerts, t.KeyStore, t.SignCerts})
	if err != nil {
		return err
	}

	// the signing CA certificate goes into cacerts.
	err = writeCert(x509FilePath(t.CaCerts, p.SignCa.Name), p.SignCa.SignCert)
	if err != nil {
		return err
	}
	// the TLS CA certificate goes into tlscacerts.
	err = writeCert(x509FilePath(t.TLSCaCerts, p.TLSCa.Name), p.TLSCa.SignCert)
	if err != nil {
		return err
	}

	// generate private key.
	priv, err := GeneratePrivateKey(t.KeyStore, p.KeyAlg)
	if err != nil {
		return errors.Wrap(err, "failed to generate private key")
	}

	// generate X509 certificate using signing CA.
	cert, err := p.SignCa.SignCertificate(t.SignCerts, p.Name, SignCertParams{
		OrgUnits:    []string{p.OU},
		KeyUsage:    x509.KeyUsageDigitalSignature,
		ExtKeyUsage: []x509.ExtKeyUsage{},
		PublicKey:   getPublicKey(priv),
	})
	if err != nil {
		return err
	}

	if p.EnableOUs {
		// generate config.yaml if required.
		err = exportConfig(t.MSP, x509FilePath(CACertsDir, p.SignCa.Name), true)
		if err != nil {
			return err
		}
	} else {
		// the signing identity goes into admincerts.
		// This means that the signing identity
		// of this MSP is also an admin of this MSP
		// NOTE: the admincerts folder is going to be
		// cleared up anyway by copyAdminCert, but
		// we leave a valid admin for now for the sake
		// of unit tests.
		err = writeCert(x509FilePath(t.AdminCerts, p.Name), cert)
		if err != nil {
			return err
		}
	}

	return nil
}

// generateTLS generates the TLS artifacts in the TLS folder.
func (t *MspTree) generateTLS(p NodeParameters) error {
	err := createAllFolders([]string{t.TLS})
	if err != nil {
		return err
	}

	// generate private key.
	tlsPrivKey, err := GeneratePrivateKey(t.TLS, p.KeyAlg)
	if err != nil {
		return err
	}

	// generate X509 certificate using TLS CA.
	_, err = p.TLSCa.SignCertificate(t.TLS, p.Name, SignCertParams{
		AlternateNames: p.TLSSans,
		KeyUsage:       x509.KeyUsageDigitalSignature | x509.KeyUsageKeyEncipherment,
		ExtKeyUsage: []x509.ExtKeyUsage{
			x509.ExtKeyUsageServerAuth,
			x509.ExtKeyUsageClientAuth,
		},
		PublicKey: getPublicKey(tlsPrivKey),
	})
	if err != nil {
		return err
	}
	err = writeCert(path.Join(t.TLS, CaCertFile), p.TLSCa.SignCert)
	if err != nil {
		return err
	}

	// Rename the generated TLS X509 cert.
	var tlsFilePrefix string
	switch p.OU {
	case ClientOU, AdminOU:
		tlsFilePrefix = ClientPrefix
	default:
		tlsFilePrefix = ServerPrefix
	}
	err = os.Rename(x509FilePath(t.TLS, p.Name), path.Join(t.TLS, tlsFilePrefix+".crt"))
	if err != nil {
		return errors.Wrap(err, "failed to rename TLS certificate")
	}
	err = os.Rename(path.Join(t.TLS, PrivateKeyFile), path.Join(t.TLS, tlsFilePrefix+".key"))
	if err != nil {
		return errors.Wrap(err, "failed to rename TLS private key")
	}
	return nil
}

func getPublicKey(priv crypto.PrivateKey) crypto.PublicKey {
	switch kk := priv.(type) {
	case *ecdsa.PrivateKey:
		return &(kk.PublicKey)
	case ed25519.PrivateKey:
		return kk.Public()
	default:
		panic("unsupported key algorithm")
	}
}

func exportConfig(mspDir, caFile string, enable bool) error {
	config := &fabricmsp.Configuration{
		NodeOUs: &fabricmsp.NodeOUs{
			Enable: enable,
			ClientOUIdentifier: &fabricmsp.OrganizationalUnitIdentifiersConfiguration{
				Certificate:                  caFile,
				OrganizationalUnitIdentifier: ClientOU,
			},
			PeerOUIdentifier: &fabricmsp.OrganizationalUnitIdentifiersConfiguration{
				Certificate:                  caFile,
				OrganizationalUnitIdentifier: PeerOU,
			},
			AdminOUIdentifier: &fabricmsp.OrganizationalUnitIdentifiersConfiguration{
				Certificate:                  caFile,
				OrganizationalUnitIdentifier: AdminOU,
			},
			OrdererOUIdentifier: &fabricmsp.OrganizationalUnitIdentifiersConfiguration{
				Certificate:                  caFile,
				OrganizationalUnitIdentifier: OrdererOU,
			},
		},
	}

	configBytes, err := yaml.Marshal(config)
	if err != nil {
		return errors.Wrap(err, "failed to marshal configuration")
	}

	file, err := os.Create(path.Join(mspDir, ConfigFile))
	if err != nil {
		return errors.Wrap(err, "failed to create configuration file")
	}
	defer func() {
		_ = file.Close()
	}()
	_, err = file.Write(configBytes)
	return errors.Wrap(err, "failed to write configuration file")
}

func createAllFolders(folders []string) error {
	for _, folder := range folders {
		err := os.MkdirAll(folder, 0o750)
		if err != nil {
			return errors.Wrapf(err, "failed to create folder %s", folder)
		}
	}
	return nil
}
