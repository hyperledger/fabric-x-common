/*
Copyright IBM Corp. All Rights Reserved.

SPDX-License-Identifier: Apache-2.0
*/

package cryptogen

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/cockroachdb/errors"
)

// OrgCryptoTree represents a cryptogen's organization tree structure.
type OrgCryptoTree struct {
	*MspTree
	NodeType int
	OrgSpec  *OrgSpec
	CA       string
	Users    string
	TLSCa    string
	Nodes    string
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

	PeersPrefix   = "peer"
	OrdererPrefix = "orderer"
	TLSCaPrefix   = "tls"

	DefaultCaHostname = "ca"
)

// Generate generates crypto in the given directory using the given config.
func Generate(rootDir string, config *Config) error {
	err := renderAll(config)
	if err != nil {
		return err
	}
	for _, c := range buildCases(config) {
		err = NewOrgCryptoTree(rootDir, c.spec, c.nodeType).GenerateOrg()
		if err != nil {
			return err
		}
	}
	return err
}

// Extend extends a crypto in the given directory using the given config.
func Extend(rootDir string, config *Config) error {
	err := renderAll(config)
	if err != nil {
		return err
	}
	for _, c := range buildCases(config) {
		err = NewOrgCryptoTree(rootDir, c.spec, c.nodeType).ExtendOrg()
		if err != nil {
			return err
		}
	}
	return nil
}

func renderAll(config *Config) error {
	for _, c := range buildCases(config) {
		err := renderOrgSpec(c.spec, c.nodeType)
		if err != nil {
			return err
		}
	}
	return nil
}

type build struct {
	spec     *OrgSpec
	nodeType int
}

func buildCases(config *Config) []build {
	ret := make([]build, 0, len(config.OrdererOrgs)+len(config.PeerOrgs))
	for _, orgSpec := range config.PeerOrgs {
		ret = append(ret, build{
			spec:     &orgSpec,
			nodeType: NodeTypePeer,
		})
	}
	for _, orgSpec := range config.OrdererOrgs {
		ret = append(ret, build{
			spec:     &orgSpec,
			nodeType: NodeTypeOrderer,
		})
	}
	return ret
}

// NewOrgCryptoTree creates a new organization tree.
func NewOrgCryptoTree(root string, org *OrgSpec, nodeType int) *OrgCryptoTree {
	var internalRoot string
	var nodesDir string
	switch nodeType {
	case NodeTypeOrderer:
		internalRoot = filepath.Join(root, OrdererOrganizationsDir, org.Domain)
		nodesDir = OrdererNodesDir
	default: // msp.NodeTypePeer
		internalRoot = filepath.Join(root, PeerOrganizationsDir, org.Domain)
		nodesDir = PeerNodesDir
	}
	return &OrgCryptoTree{
		MspTree:  NewMspTree(internalRoot),
		NodeType: nodeType,
		OrgSpec:  org,
		CA:       filepath.Join(internalRoot, CaDir),
		Users:    filepath.Join(internalRoot, UsersDir),
		TLSCa:    filepath.Join(internalRoot, TLSCaDir),
		Nodes:    filepath.Join(internalRoot, nodesDir),
	}
}

// SubNode returns a sub MSP tree of a specific node.
func (c *OrgCryptoTree) SubNode(name string) *MspTree {
	return NewMspTree(filepath.Join(c.Nodes, name))
}

// SubUser returns a sub MSP of a specific user.
func (c *OrgCryptoTree) SubUser(name string) *MspTree {
	return NewMspTree(filepath.Join(c.Users, name))
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
		Type:      c.NodeType,
		EnableOUs: s.EnableNodeOUs,
		KeyAlg:    s.CA.PublicKeyAlgorithm,
	}
	err = c.GenerateVerifyingMSP(p)
	if err != nil {
		return err
	}

	err = generateNodes(c.Nodes, s.Specs, p)
	if err != nil {
		return err
	}

	var users []NodeSpec
	if c.NodeType == NodeTypePeer {
		users = c.generateUsers()
	}

	// add an admin user
	orgAdminUser := adminUser(orgName)
	users = append(users, orgAdminUser)

	p.Type = NodeTypeClient
	err = generateNodes(c.Users, users, p)
	if err != nil {
		return err
	}

	// copy the admin cert to the org's MSP admincerts
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
		Type:      c.NodeType,
		EnableOUs: s.EnableNodeOUs,
	}
	err = generateNodes(c.Nodes, s.Specs, p)
	if err != nil {
		return err
	}

	if !c.OrgSpec.EnableNodeOUs {
		err = c.copyAllAdminCert(adminUser(orgName).CommonName)
		if err != nil {
			return err
		}
	}

	if c.NodeType != NodeTypePeer {
		return nil
	}

	// We generate users only for the peer.
	users := c.generateUsers()
	p.Type = NodeTypeClient
	return generateNodes(c.Users, users, p)
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
		})
	}
	for j := range s.Users.Count {
		users = append(users, NodeSpec{
			CommonName:         fmt.Sprintf("%s%d@%s", userBaseName, j+1, orgName),
			PublicKeyAlgorithm: publicKeyAlg,
		})
	}
	return users
}

// copyAllAdminCert copy the admin cert to each of the org's MSP admincerts.
func (c *OrgCryptoTree) copyAllAdminCert(orgAdminUserName string) error {
	for _, spec := range c.OrgSpec.Specs {
		err := c.copyAdminCert(c.SubNode(spec.CommonName).AdminCerts, orgAdminUserName)
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

func generateNodes(nodesDir string, nodes []NodeSpec, p NodeParameters) error {
	for _, node := range nodes {
		tree := NewMspTree(filepath.Join(nodesDir, node.CommonName))
		if tree.IsExist() {
			continue
		}
		curParams := p
		if node.isAdmin && p.EnableOUs {
			curParams.Type = NodeTypeAdmin
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
		isAdmin:            true,
		CommonName:         fmt.Sprintf("%s@%s", adminBaseName, orgName),
		PublicKeyAlgorithm: ECDSA,
	}
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
