/*
Copyright IBM Corp. All Rights Reserved.

SPDX-License-Identifier: Apache-2.0
*/

package cryptogen

import (
	"fmt"
	"os"
	"path"
	"path/filepath"
	"slices"

	"github.com/cockroachdb/errors"
)

// OrgCryptoTree represents a cryptogen's organization tree structure.
type OrgCryptoTree struct {
	*MspTree
	OrgSpec       *OrgSpec
	CA            string
	Users         string
	TLSCa         string
	OrderingNodes string
	PeerNodes     string
}

// Crypto collects all the generated crypto material.
type Crypto struct {
	OrdererOrgs []*OrgCryptoTree
	PeerOrgs    []*OrgCryptoTree
	GenericOrgs []*OrgCryptoTree
}

const (
	userBaseName            = "User"
	adminBaseName           = "Admin"
	defaultHostnameTemplate = "{{.Prefix}}{{.Index}}"
	defaultCNTemplate       = "{{.Hostname}}.{{.Domain}}"
)

// Tree names.
const (
	CaDir                   = "ca"
	UsersDir                = "users"
	TLSCaDir                = "tlsca"
	PeerNodesDir            = "peers"
	OrdererNodesDir         = "orderers"
	OrdererOrganizationsDir = "ordererOrganizations"
	PeerOrganizationsDir    = "peerOrganizations"
	GenericOrganizationsDir = "organizations"

	TLSCaPrefix = "tls"

	DefaultCaHostname = "ca"
)

// Generate generates crypto in the given directory using the given config.
func Generate(rootDir string, config *Config) (*Crypto, error) {
	c, err := prepareAllCryptoSpecs(rootDir, config)
	if err != nil {
		return nil, err
	}
	for _, c := range allTrees(c) {
		err = c.GenerateOrg()
		if err != nil {
			return nil, err
		}
	}
	return c, nil
}

// Extend extends a crypto in the given directory using the given config.
func Extend(rootDir string, config *Config) (*Crypto, error) {
	c, err := prepareAllCryptoSpecs(rootDir, config)
	if err != nil {
		return nil, err
	}
	for _, c := range allTrees(c) {
		err = c.ExtendOrg()
		if err != nil {
			return nil, err
		}
	}
	return c, nil
}

func prepareAllCryptoSpecs(rootDir string, config *Config) (*Crypto, error) {
	c := &Crypto{
		OrdererOrgs: make([]*OrgCryptoTree, len(config.OrdererOrgs)),
		PeerOrgs:    make([]*OrgCryptoTree, len(config.PeerOrgs)),
		GenericOrgs: make([]*OrgCryptoTree, len(config.GenericOrgs)),
	}
	for i := range config.OrdererOrgs {
		s := &config.OrdererOrgs[i]
		err := renderOrgSpecForOrgUnitWithTemplate(s, OrdererOU)
		if err != nil {
			return nil, err
		}
		c.OrdererOrgs[i] = NewOrgCryptoTree(path.Join(rootDir, OrdererOrganizationsDir), s)
	}
	for i := range config.PeerOrgs {
		s := &config.PeerOrgs[i]
		err := renderOrgSpecForOrgUnitWithTemplate(s, PeerOU)
		if err != nil {
			return nil, err
		}
		c.PeerOrgs[i] = NewOrgCryptoTree(path.Join(rootDir, PeerOrganizationsDir), &config.PeerOrgs[i])
	}
	for i := range config.GenericOrgs {
		s := &config.GenericOrgs[i]
		// We do not render templates for generic organizations.
		err := renderOrgSpec(s)
		if err != nil {
			return nil, err
		}
		c.GenericOrgs[i] = NewOrgCryptoTree(path.Join(rootDir, GenericOrganizationsDir), s)
	}
	return c, nil
}

func allTrees(c *Crypto) []*OrgCryptoTree {
	return slices.Concat(c.OrdererOrgs, c.PeerOrgs, c.GenericOrgs)
}

// NewOrgCryptoTree creates a new organization tree.
func NewOrgCryptoTree(root string, org *OrgSpec) *OrgCryptoTree {
	root = filepath.Join(root, org.Name)
	return &OrgCryptoTree{
		MspTree:       NewMspTree(root),
		OrgSpec:       org,
		CA:            filepath.Join(root, CaDir),
		Users:         filepath.Join(root, UsersDir),
		TLSCa:         filepath.Join(root, TLSCaDir),
		OrderingNodes: filepath.Join(root, OrdererNodesDir),
		PeerNodes:     filepath.Join(root, PeerNodesDir),
	}
}

// SubUser returns a sub MSP tree of a specific user.
func (c *OrgCryptoTree) SubUser(name string) *MspTree {
	return NewMspTree(filepath.Join(c.Users, name))
}

// SubNode returns a sub MSP tree of a specific node.
func (c *OrgCryptoTree) SubNode(party, name, nodeOU string) *MspTree {
	var nodeDir string
	switch nodeOU {
	case OrdererOU:
		nodeDir = c.OrderingNodes
	case PeerOU:
		nodeDir = c.PeerNodes
	default: // AdminOU, ClientOU
		nodeDir = c.Users
	}
	return NewMspTree(filepath.Join(nodeDir, party, name))
}

// SubNodeFromSpec returns a sub MSP tree of a specific node.
func (c *OrgCryptoTree) SubNodeFromSpec(s *NodeSpec) *MspTree {
	return c.SubNode(s.Party, s.CommonName, s.OrganizationalUnit)
}

func caFromSpec(orgName string, s *NodeSpec) *CA {
	return &CA{
		Organization:       orgName,
		Name:               s.CommonName,
		Country:            s.Country,
		Province:           s.Province,
		Locality:           s.Locality,
		OrganizationalUnit: s.OrganizationalUnit,
		StreetAddress:      s.StreetAddress,
		PostalCode:         s.PostalCode,
		KeyAlgorithm:       s.PublicKeyAlgorithm,
	}
}

// GenerateOrg generate the organization's crypto.
func (c *OrgCryptoTree) GenerateOrg() error {
	s := c.OrgSpec
	orgName := s.Domain

	// generate signing CA
	signCA := caFromSpec(orgName, &s.CA)
	err := BuildCA(c.CA, signCA)
	if err != nil {
		return err
	}
	// generate TLS CA
	tlsCA := caFromSpec(orgName, &s.CA)
	tlsCA.Name = TLSCaPrefix + s.CA.CommonName
	err = BuildCA(c.TLSCa, tlsCA)
	if err != nil {
		return err
	}

	p := NodeParameters{
		SignCa:    signCA,
		TLSCa:     tlsCA,
		EnableOUs: s.EnableNodeOUs,
		KeyAlg:    s.CA.PublicKeyAlgorithm,
	}
	err = c.GenerateVerifyingMSP(p)
	if err != nil {
		return err
	}

	err = c.generateNodes(s.Specs, p)
	if err != nil {
		return err
	}

	// generate users with the admin user.
	orgAdminUser := adminUser(orgName)
	users := append(c.generateUsers(), orgAdminUser)
	err = c.generateNodes(users, p)
	if err != nil {
		return err
	}

	// copy the admin cert to the org's MSP admincerts.
	if !s.EnableNodeOUs {
		err = c.copyAdminCert(c.AdminCerts, orgAdminUser.CommonName)
		if err != nil {
			return err
		}
		err = c.copyAllAdminCert(orgAdminUser.CommonName)
		if err != nil {
			return err
		}
	}

	return nil
}

// ExtendOrg extends the organization's crypto.
func (c *OrgCryptoTree) ExtendOrg() error {
	if !c.IsExist() {
		return c.GenerateOrg()
	}

	s := c.OrgSpec
	orgName := s.Domain
	signCA, err := getCA(c.CA, s, s.CA.CommonName)
	if err != nil {
		return err
	}
	tlsCA, err := getCA(c.TLSCa, s, TLSCaPrefix+s.CA.CommonName)
	if err != nil {
		return err
	}

	p := NodeParameters{
		SignCa:    signCA,
		TLSCa:     tlsCA,
		EnableOUs: s.EnableNodeOUs,
	}
	err = c.generateNodes(s.Specs, p)
	if err != nil {
		return err
	}

	if !c.OrgSpec.EnableNodeOUs {
		err = c.copyAllAdminCert(adminUser(orgName).CommonName)
		if err != nil {
			return err
		}
	}

	return c.generateNodes(c.generateUsers(), p)
}

func (c *OrgCryptoTree) generateUsers() []NodeSpec {
	s := c.OrgSpec
	orgName := s.Domain
	users := make([]NodeSpec, 0, len(s.Users.Specs)+s.Users.Count)
	publicKeyAlg := getPublicKeyAlg(s.Users.PublicKeyAlgorithm)
	for _, spec := range s.Users.Specs {
		users = append(users, NodeSpec{
			CommonName:         fmt.Sprintf("%s@%s", spec.Name, orgName),
			PublicKeyAlgorithm: publicKeyAlg,
			OrganizationalUnit: ClientOU,
		})
	}
	for j := range s.Users.Count {
		users = append(users, NodeSpec{
			CommonName:         fmt.Sprintf("%s%d@%s", userBaseName, j+1, orgName),
			PublicKeyAlgorithm: publicKeyAlg,
			OrganizationalUnit: ClientOU,
		})
	}
	return users
}

// copyAllAdminCert copy the admin cert to each of the org's MSP admincerts.
func (c *OrgCryptoTree) copyAllAdminCert(orgAdminUserName string) error {
	for _, spec := range c.OrgSpec.Specs {
		err := c.copyAdminCert(c.SubNodeFromSpec(&spec).AdminCerts, orgAdminUserName)
		if err != nil {
			return err
		}
	}
	return nil
}

func (c *OrgCryptoTree) copyAdminCert(adminCertsDir, adminUserName string) error {
	adminCertPath := filepath.Join(adminCertsDir, adminUserName+"-cert.pem")
	if _, err := os.Stat(adminCertPath); !os.IsNotExist(err) {
		return nil
	}
	// delete the contents of admincerts
	err := os.RemoveAll(adminCertsDir)
	if err != nil {
		return errors.Wrapf(err, "error removing admin cert directory %s", adminCertsDir)
	}
	// recreate the admincerts directory
	err = os.MkdirAll(adminCertsDir, 0o750)
	if err != nil {
		return errors.Wrapf(err, "error creating admin cert directory %s", adminCertsDir)
	}
	src := filepath.Join(c.SubUser(adminUserName).SignCerts, adminUserName+"-cert.pem")
	return copyFile(src, adminCertPath)
}

func (c *OrgCryptoTree) generateNodes(nodes []NodeSpec, p NodeParameters) error {
	for i := range nodes {
		node := &nodes[i]
		tree := c.SubNodeFromSpec(node)
		if tree.IsExist() {
			continue
		}
		curParams := p
		curParams.OU = node.OrganizationalUnit
		if node.OrganizationalUnit == AdminOU && !p.EnableOUs {
			curParams.OU = ClientOU
		}
		curParams.Name = node.CommonName
		curParams.TLSSans = node.SANS
		curParams.KeyAlg = node.PublicKeyAlgorithm
		err := tree.GenerateLocalMSP(curParams)
		if err != nil {
			return err
		}
	}
	return nil
}

func copyFile(src, dst string) error {
	content, err := os.ReadFile(src)
	if err != nil {
		return errors.Wrapf(err, "error reading source file %s", src)
	}
	err = os.WriteFile(dst, content, 0o650)
	if err != nil {
		return errors.Wrapf(err, "error writing destination file %s", dst)
	}
	return nil
}

func adminUser(orgName string) NodeSpec {
	return NodeSpec{
		CommonName:         adminUserName(orgName),
		PublicKeyAlgorithm: ECDSA,
		OrganizationalUnit: AdminOU,
	}
}

func adminUserName(orgName string) string {
	return fmt.Sprintf("%s@%s", adminBaseName, orgName)
}

func getCA(caDir string, spec *OrgSpec, name string) (*CA, error) {
	privateKey, err := LoadPrivateKey(caDir)
	if err != nil {
		return nil, err
	}
	cert, err := LoadCertificate(caDir)
	if err != nil {
		return nil, err
	}
	return &CA{
		Name:               name,
		Signer:             NewSignerFromPrivateKey(privateKey),
		SignCert:           cert,
		Country:            spec.CA.Country,
		Province:           spec.CA.Province,
		Locality:           spec.CA.Locality,
		OrganizationalUnit: spec.CA.OrganizationalUnit,
		StreetAddress:      spec.CA.StreetAddress,
		PostalCode:         spec.CA.PostalCode,
	}, nil
}

func getPublicKeyAlg(pubAlgFromConfig string) (publicKeyAlg string) {
	if pubAlgFromConfig == "" {
		return ECDSA
	}
	return pubAlgFromConfig
}
