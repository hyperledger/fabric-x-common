/*
Copyright IBM Corp. All Rights Reserved.

SPDX-License-Identifier: Apache-2.0
*/
package main

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"text/template"

	"github.com/hyperledger/fabric-x-common/internaltools/cryptogen/ca"
	"github.com/hyperledger/fabric-x-common/internaltools/cryptogen/csp"
	"github.com/hyperledger/fabric-x-common/internaltools/cryptogen/metadata"
	"github.com/hyperledger/fabric-x-common/internaltools/cryptogen/msp"

	"gopkg.in/alecthomas/kingpin.v2"
	"gopkg.in/yaml.v2"
)

const (
	userBaseName            = "User"
	adminBaseName           = "Admin"
	defaultHostnameTemplate = "{{.Prefix}}{{.Index}}"
	defaultCNTemplate       = "{{.Hostname}}.{{.Domain}}"
	ECDSA                   = "ecdsa"
	ED25519                 = "ed25519"
)

type HostnameData struct {
	Prefix string
	Index  int
	Domain string
}

type SpecData struct {
	Hostname   string
	Domain     string
	CommonName string
}

type NodeTemplate struct {
	Count              int      `yaml:"Count"`
	Start              int      `yaml:"Start"`
	Hostname           string   `yaml:"Hostname"`
	SANS               []string `yaml:"SANS"`
	PublicKeyAlgorithm string   `yaml:"PublicKeyAlgorithm"`
}

type NodeSpec struct {
	isAdmin            bool
	Hostname           string   `yaml:"Hostname"`
	CommonName         string   `yaml:"CommonName"`
	Country            string   `yaml:"Country"`
	Province           string   `yaml:"Province"`
	Locality           string   `yaml:"Locality"`
	OrganizationalUnit string   `yaml:"OrganizationalUnit"`
	StreetAddress      string   `yaml:"StreetAddress"`
	PostalCode         string   `yaml:"PostalCode"`
	SANS               []string `yaml:"SANS"`
	PublicKeyAlgorithm string   `yaml:"PublicKeyAlgorithm"`
}

// UserSpec Contains User specifications needed to customize the crypto material generation.
type UserSpec struct {
	Name string `yaml:"Name"`
}

type UsersSpec struct {
	Count              int        `yaml:"Count"`
	PublicKeyAlgorithm string     `yaml:"PublicKeyAlgorithm"`
	Specs              []UserSpec `yaml:"Specs"`
}

type OrgSpec struct {
	Name          string       `yaml:"Name"`
	Domain        string       `yaml:"Domain"`
	EnableNodeOUs bool         `yaml:"EnableNodeOUs"`
	CA            NodeSpec     `yaml:"CA"`
	Template      NodeTemplate `yaml:"Template"`
	Specs         []NodeSpec   `yaml:"Specs"`
	Users         UsersSpec    `yaml:"Users"`
}

type Config struct {
	OrdererOrgs []OrgSpec `yaml:"OrdererOrgs"`
	PeerOrgs    []OrgSpec `yaml:"PeerOrgs"`
}

var defaultConfig = `
# ---------------------------------------------------------------------------
# "OrdererOrgs" - Definition of organizations managing orderer nodes
# ---------------------------------------------------------------------------
OrdererOrgs:
  # ---------------------------------------------------------------------------
  # Orderer
  # ---------------------------------------------------------------------------
  - Name: Orderer
    Domain: example.com
    EnableNodeOUs: false

    # ---------------------------------------------------------------------------
    # "Specs" - See PeerOrgs below for complete description
    # ---------------------------------------------------------------------------
    Specs:
      - Hostname: orderer

# ---------------------------------------------------------------------------
# "PeerOrgs" - Definition of organizations managing peer nodes
# ---------------------------------------------------------------------------
PeerOrgs:
  # ---------------------------------------------------------------------------
  # Org1
  # ---------------------------------------------------------------------------
  - Name: Org1
    Domain: org1.example.com
    EnableNodeOUs: false

    # ---------------------------------------------------------------------------
    # "CA"
    # ---------------------------------------------------------------------------
    # Uncomment this section to enable the explicit definition of the CA for this
    # organization.  This entry is a Spec.  See "Specs" section below for details.
    # ---------------------------------------------------------------------------
    # CA:
    #    Hostname: ca # implicitly ca.org1.example.com
    #    Country: US
    #    Province: California
    #    Locality: San Francisco
    #    OrganizationalUnit: Hyperledger Fabric
    #    StreetAddress: address for org # default nil
    #    PostalCode: postalCode for org # default nil
    #    PublicKeyAlgorithm: ecdsa # CA's key algorithm ("ecdsa" or "ed25519")

    # ---------------------------------------------------------------------------
    # "Specs"
    # ---------------------------------------------------------------------------
    # Uncomment this section to enable the explicit definition of hosts in your
    # configuration.  Most users will want to use Template, below
    #
    # Specs is an array of Spec entries.  Each Spec entry consists of two fields:
    #   - Hostname:   (Required) The desired hostname, sans the domain.
    #   - CommonName: (Optional) Specifies the template or explicit override for
    #                 the CN.  By default, this is the template:
    #
    #                              "{{.Hostname}}.{{.Domain}}"
    #
    #                 which obtains its values from the Spec.Hostname and
    #                 Org.Domain, respectively.
    #   - SANS:       (Optional) Specifies one or more Subject Alternative Names
    #                 to be set in the resulting x509. Accepts template
    #                 variables {{.Hostname}}, {{.Domain}}, {{.CommonName}}. IP
    #                 addresses provided here will be properly recognized. Other
    #                 values will be taken as DNS names.
    #                 NOTE: Two implicit entries are created for you:
    #                     - {{ .CommonName }}
    #                     - {{ .Hostname }}
    #   PublicKeyAlgorithm: Nodes' key algorithm ("ecdsa" or "ed25519")
    # ---------------------------------------------------------------------------
    # Specs:
    #   - Hostname: foo # implicitly "foo.org1.example.com"
    #     CommonName: foo27.org5.example.com # overrides Hostname-based FQDN set above
    #     SANS:
    #       - "bar.{{.Domain}}"
    #       - "altfoo.{{.Domain}}"
    #       - "{{.Hostname}}.org6.net"
    #       - 172.16.10.31
    #     PublicKeyAlgorithm: ecdsa
    #   - Hostname: bar
    #   - Hostname: baz

    # ---------------------------------------------------------------------------
    # "Template"
    # ---------------------------------------------------------------------------
    # Allows for the definition of 1 or more hosts that are created sequentially
    # from a template. By default, this looks like "peer%d" from 0 to Count-1.
    # You may override the number of nodes (Count), the starting index (Start)
    # or the template used to construct the name (Hostname).
    #
    # PublicKeyAlgorithm: Hosts' key algorithm ("ecdsa" or "ed25519")
    #
    # Note: Template and Specs are not mutually exclusive.  You may define both
    # sections and the aggregate nodes will be created for you.  Take care with
    # name collisions
    # ---------------------------------------------------------------------------
    Template:
      Count: 1
      # Start: 5
      # Hostname: {{.Prefix}}{{.Index}} # default
      # SANS:
      #   - "{{.Hostname}}.alt.{{.Domain}}"
      # PublicKeyAlgorithm: "ecdsa"

    # ---------------------------------------------------------------------------
    # "Users"
    # ---------------------------------------------------------------------------
    # Count: The number of user accounts _in addition_ to Admin
    # PublicKeyAlgorithm: Users' key algorithm ("ecdsa" or "ed25519")
    # ---------------------------------------------------------------------------
    Users:
      Count: 1
      PublicKeyAlgorithm: "ecdsa"

  # ---------------------------------------------------------------------------
  # Org2: See "Org1" for full specification
  # ---------------------------------------------------------------------------
  - Name: Org2
    Domain: org2.example.com
    EnableNodeOUs: false
    Template:
      Count: 1
    Users:
      Count: 1
      Specs:
        - Name: testuser
`

// command line flags
var (
	app = kingpin.New("cryptogen", "Utility for generating Hyperledger Fabric key material")

	gen           = app.Command("generate", "Generate key material")
	outputDir     = gen.Flag("output", "The output directory in which to place artifacts").Default("crypto-config").String()
	genConfigFile = gen.Flag("config", "The configuration template to use").File()
	showtemplate  = app.Command("showtemplate", "Show the default configuration template")

	version       = app.Command("version", "Show version information")
	ext           = app.Command("extend", "Extend existing network")
	inputDir      = ext.Flag("input", "The input directory in which existing network place").Default("crypto-config").String()
	extConfigFile = ext.Flag("config", "The configuration template to use").File()
)

func main() {
	kingpin.Version("0.0.1")
	switch kingpin.MustParse(app.Parse(os.Args[1:])) {

	// "generate" command
	case gen.FullCommand():
		generate()

	case ext.FullCommand():
		extend()

		// "showtemplate" command
	case showtemplate.FullCommand():
		fmt.Print(defaultConfig)
		os.Exit(0)

		// "version" command
	case version.FullCommand():
		printVersion()
	}
}

func getConfig() (*Config, error) {
	var configData string

	if *genConfigFile != nil {
		data, err := io.ReadAll(*genConfigFile)
		if err != nil {
			return nil, fmt.Errorf("Error reading configuration: %s", err)
		}

		configData = string(data)
	} else if *extConfigFile != nil {
		data, err := io.ReadAll(*extConfigFile)
		if err != nil {
			return nil, fmt.Errorf("Error reading configuration: %s", err)
		}

		configData = string(data)
	} else {
		configData = defaultConfig
	}

	config := &Config{}
	err := yaml.Unmarshal([]byte(configData), &config)
	if err != nil {
		return nil, fmt.Errorf("Error Unmarshalling YAML: %s", err)
	}

	return config, nil
}

func extend() {
	config, err := getConfig()
	if err != nil {
		fmt.Printf("Error reading config: %s", err)
		os.Exit(-1)
	}

	for _, orgSpec := range config.PeerOrgs {
		err = renderOrgSpec(&orgSpec, "peer")
		if err != nil {
			fmt.Printf("Error processing peer configuration: %s", err)
			os.Exit(-1)
		}
		extendPeerOrg(orgSpec)
	}

	for _, orgSpec := range config.OrdererOrgs {
		err = renderOrgSpec(&orgSpec, "orderer")
		if err != nil {
			fmt.Printf("Error processing orderer configuration: %s", err)
			os.Exit(-1)
		}
		extendOrdererOrg(orgSpec)
	}
}

func extendPeerOrg(orgSpec OrgSpec) {
	orgName := orgSpec.Domain
	orgDir := filepath.Join(*inputDir, "peerOrganizations", orgName)
	if _, err := os.Stat(orgDir); os.IsNotExist(err) {
		generatePeerOrg(*inputDir, orgSpec)
		return
	}

	peersDir := filepath.Join(orgDir, "peers")
	usersDir := filepath.Join(orgDir, "users")
	caDir := filepath.Join(orgDir, "ca")
	tlscaDir := filepath.Join(orgDir, "tlsca")

	signCA := getCA(caDir, orgSpec, orgSpec.CA.CommonName)
	tlsCA := getCA(tlscaDir, orgSpec, "tls"+orgSpec.CA.CommonName)

	generateNodes(peersDir, orgSpec.Specs, signCA, tlsCA, msp.PEER, orgSpec.EnableNodeOUs)

	adminUser := NodeSpec{
		isAdmin:            true,
		CommonName:         fmt.Sprintf("%s@%s", adminBaseName, orgName),
		PublicKeyAlgorithm: ECDSA,
	}
	// copy the admin cert to each of the org's peer's MSP admincerts
	for _, spec := range orgSpec.Specs {
		if orgSpec.EnableNodeOUs {
			continue
		}
		err := copyAdminCert(usersDir,
			filepath.Join(peersDir, spec.CommonName, "msp", "admincerts"), adminUser.CommonName)
		if err != nil {
			fmt.Printf("Error copying admin cert for org %s peer %s:\n%v\n",
				orgName, spec.CommonName, err)
			os.Exit(1)
		}
	}

	publicKeyAlg := getPublicKeyAlg(orgSpec.Users.PublicKeyAlgorithm)
	// TODO: add ability to specify usernames
	users := []NodeSpec{}
	for j := 1; j <= orgSpec.Users.Count; j++ {
		user := NodeSpec{
			CommonName:         fmt.Sprintf("%s%d@%s", userBaseName, j, orgName),
			PublicKeyAlgorithm: publicKeyAlg,
		}

		users = append(users, user)
	}

	generateNodes(usersDir, users, signCA, tlsCA, msp.CLIENT, orgSpec.EnableNodeOUs)
}

func extendOrdererOrg(orgSpec OrgSpec) {
	orgName := orgSpec.Domain

	orgDir := filepath.Join(*inputDir, "ordererOrganizations", orgName)
	caDir := filepath.Join(orgDir, "ca")
	usersDir := filepath.Join(orgDir, "users")
	tlscaDir := filepath.Join(orgDir, "tlsca")
	orderersDir := filepath.Join(orgDir, "orderers")
	if _, err := os.Stat(orgDir); os.IsNotExist(err) {
		generateOrdererOrg(*inputDir, orgSpec)
		return
	}

	signCA := getCA(caDir, orgSpec, orgSpec.CA.CommonName)
	tlsCA := getCA(tlscaDir, orgSpec, "tls"+orgSpec.CA.CommonName)

	generateNodes(orderersDir, orgSpec.Specs, signCA, tlsCA, msp.ORDERER, orgSpec.EnableNodeOUs)

	adminUser := NodeSpec{
		isAdmin:            true,
		CommonName:         fmt.Sprintf("%s@%s", adminBaseName, orgName),
		PublicKeyAlgorithm: ECDSA,
	}

	for _, spec := range orgSpec.Specs {
		if orgSpec.EnableNodeOUs {
			continue
		}
		err := copyAdminCert(usersDir,
			filepath.Join(orderersDir, spec.CommonName, "msp", "admincerts"), adminUser.CommonName)
		if err != nil {
			fmt.Printf("Error copying admin cert for org %s orderer %s:\n%v\n",
				orgName, spec.CommonName, err)
			os.Exit(1)
		}
	}
}

func generate() {
	config, err := getConfig()
	if err != nil {
		fmt.Printf("Error reading config: %s", err)
		os.Exit(-1)
	}

	for _, orgSpec := range config.PeerOrgs {
		err = renderOrgSpec(&orgSpec, "peer")
		if err != nil {
			fmt.Printf("Error processing peer configuration: %s", err)
			os.Exit(-1)
		}
		generatePeerOrg(*outputDir, orgSpec)
	}

	for _, orgSpec := range config.OrdererOrgs {
		err = renderOrgSpec(&orgSpec, "orderer")
		if err != nil {
			fmt.Printf("Error processing orderer configuration: %s", err)
			os.Exit(-1)
		}
		generateOrdererOrg(*outputDir, orgSpec)
	}
}

func parseTemplate(input string, data interface{}) (string, error) {
	t, err := template.New("parse").Parse(input)
	if err != nil {
		return "", fmt.Errorf("Error parsing template: %s", err)
	}

	output := new(bytes.Buffer)
	err = t.Execute(output, data)
	if err != nil {
		return "", fmt.Errorf("Error executing template: %s", err)
	}

	return output.String(), nil
}

func parseTemplateWithDefault(input, defaultInput string, data interface{}) (string, error) {
	// Use the default if the input is an empty string
	if len(input) == 0 {
		input = defaultInput
	}

	return parseTemplate(input, data)
}

func renderNodeSpec(domain string, spec *NodeSpec) error {
	data := SpecData{
		Hostname: spec.Hostname,
		Domain:   domain,
	}

	// Process our CommonName
	cn, err := parseTemplateWithDefault(spec.CommonName, defaultCNTemplate, data)
	if err != nil {
		return err
	}

	spec.CommonName = cn
	data.CommonName = cn

	if spec.PublicKeyAlgorithm == "" {
		spec.PublicKeyAlgorithm = ECDSA
	}

	// Save off our original, unprocessed SANS entries
	origSANS := spec.SANS

	// Set our implicit SANS entries for CN/Hostname
	spec.SANS = []string{cn, spec.Hostname}

	// Finally, process any remaining SANS entries
	for _, _san := range origSANS {
		san, err := parseTemplate(_san, data)
		if err != nil {
			return err
		}

		spec.SANS = append(spec.SANS, san)
	}

	return nil
}

func renderOrgSpec(orgSpec *OrgSpec, prefix string) error {
	publickKeyAlg := getPublicKeyAlg(orgSpec.Template.PublicKeyAlgorithm)
	// First process all of our templated nodes
	for i := 0; i < orgSpec.Template.Count; i++ {
		data := HostnameData{
			Prefix: prefix,
			Index:  i + orgSpec.Template.Start,
			Domain: orgSpec.Domain,
		}

		hostname, err := parseTemplateWithDefault(orgSpec.Template.Hostname, defaultHostnameTemplate, data)
		if err != nil {
			return err
		}

		spec := NodeSpec{
			Hostname:           hostname,
			SANS:               orgSpec.Template.SANS,
			PublicKeyAlgorithm: publickKeyAlg,
		}
		orgSpec.Specs = append(orgSpec.Specs, spec)
	}

	// Touch up all general node-specs to add the domain
	for idx, spec := range orgSpec.Specs {
		err := renderNodeSpec(orgSpec.Domain, &spec)
		if err != nil {
			return err
		}

		orgSpec.Specs[idx] = spec
	}

	// Process the CA node-spec in the same manner
	if len(orgSpec.CA.Hostname) == 0 {
		orgSpec.CA.Hostname = "ca"
	}
	err := renderNodeSpec(orgSpec.Domain, &orgSpec.CA)
	if err != nil {
		return err
	}

	return nil
}

func generatePeerOrg(baseDir string, orgSpec OrgSpec) {
	orgName := orgSpec.Domain

	fmt.Println(orgName)
	// generate CAs
	orgDir := filepath.Join(baseDir, "peerOrganizations", orgName)
	caDir := filepath.Join(orgDir, "ca")
	tlsCADir := filepath.Join(orgDir, "tlsca")
	mspDir := filepath.Join(orgDir, "msp")
	peersDir := filepath.Join(orgDir, "peers")
	usersDir := filepath.Join(orgDir, "users")
	adminCertsDir := filepath.Join(mspDir, "admincerts")
	// generate signing CA
	signCA, err := ca.NewCA(caDir, orgName, orgSpec.CA.CommonName, orgSpec.CA.Country, orgSpec.CA.Province, orgSpec.CA.Locality, orgSpec.CA.OrganizationalUnit, orgSpec.CA.StreetAddress, orgSpec.CA.PostalCode, orgSpec.CA.PublicKeyAlgorithm)
	if err != nil {
		fmt.Printf("Error generating signCA for org %s:\n%v\n", orgName, err)
		os.Exit(1)
	}
	// generate TLS CA
	tlsCA, err := ca.NewCA(tlsCADir, orgName, "tls"+orgSpec.CA.CommonName, orgSpec.CA.Country, orgSpec.CA.Province, orgSpec.CA.Locality, orgSpec.CA.OrganizationalUnit, orgSpec.CA.StreetAddress, orgSpec.CA.PostalCode, orgSpec.CA.PublicKeyAlgorithm)
	if err != nil {
		fmt.Printf("Error generating tlsCA for org %s:\n%v\n", orgName, err)
		os.Exit(1)
	}

	err = msp.GenerateVerifyingMSP(mspDir, signCA, tlsCA, orgSpec.EnableNodeOUs, orgSpec.CA.PublicKeyAlgorithm)
	if err != nil {
		fmt.Printf("Error generating MSP for org %s:\n%v\n", orgName, err)
		os.Exit(1)
	}

	generateNodes(peersDir, orgSpec.Specs, signCA, tlsCA, msp.PEER, orgSpec.EnableNodeOUs)

	publicKeyAlg := getPublicKeyAlg(orgSpec.Users.PublicKeyAlgorithm)
	users := make([]NodeSpec, 0, len(orgSpec.Users.Specs)+orgSpec.Users.Count)
	for _, s := range orgSpec.Users.Specs {
		users = append(users, NodeSpec{
			CommonName:         fmt.Sprintf("%s@%s", s.Name, orgName),
			PublicKeyAlgorithm: publicKeyAlg,
		})
	}
	for j := range orgSpec.Users.Count {
		users = append(users, NodeSpec{
			CommonName:         fmt.Sprintf("%s%d@%s", userBaseName, j+1, orgName),
			PublicKeyAlgorithm: publicKeyAlg,
		})
	}

	// add an admin user
	adminUser := NodeSpec{
		isAdmin:            true,
		CommonName:         fmt.Sprintf("%s@%s", adminBaseName, orgName),
		PublicKeyAlgorithm: ECDSA,
	}

	users = append(users, adminUser)
	generateNodes(usersDir, users, signCA, tlsCA, msp.CLIENT, orgSpec.EnableNodeOUs)

	// copy the admin cert to the org's MSP admincerts
	if !orgSpec.EnableNodeOUs {
		err = copyAdminCert(usersDir, adminCertsDir, adminUser.CommonName)
		if err != nil {
			fmt.Printf("Error copying admin cert for org %s:\n%v\n",
				orgName, err)
			os.Exit(1)
		}
	}

	// copy the admin cert to each of the org's peer's MSP admincerts
	for _, spec := range orgSpec.Specs {
		if orgSpec.EnableNodeOUs {
			continue
		}
		err = copyAdminCert(usersDir,
			filepath.Join(peersDir, spec.CommonName, "msp", "admincerts"), adminUser.CommonName)
		if err != nil {
			fmt.Printf("Error copying admin cert for org %s peer %s:\n%v\n",
				orgName, spec.CommonName, err)
			os.Exit(1)
		}
	}
}

func copyAdminCert(usersDir, adminCertsDir, adminUserName string) error {
	if _, err := os.Stat(filepath.Join(adminCertsDir,
		adminUserName+"-cert.pem")); err == nil {
		return nil
	}
	// delete the contents of admincerts
	err := os.RemoveAll(adminCertsDir)
	if err != nil {
		return err
	}
	// recreate the admincerts directory
	err = os.MkdirAll(adminCertsDir, 0o755)
	if err != nil {
		return err
	}
	err = copyFile(filepath.Join(usersDir, adminUserName, "msp", "signcerts",
		adminUserName+"-cert.pem"), filepath.Join(adminCertsDir,
		adminUserName+"-cert.pem"))
	if err != nil {
		return err
	}
	return nil
}

func generateNodes(baseDir string, nodes []NodeSpec, signCA *ca.CA, tlsCA *ca.CA, nodeType int, nodeOUs bool) {
	for _, node := range nodes {
		nodeDir := filepath.Join(baseDir, node.CommonName)
		if _, err := os.Stat(nodeDir); os.IsNotExist(err) {
			currentNodeType := nodeType
			if node.isAdmin && nodeOUs {
				currentNodeType = msp.ADMIN
			}
			err := msp.GenerateLocalMSP(nodeDir, node.CommonName, node.SANS, signCA, tlsCA, currentNodeType, nodeOUs, node.PublicKeyAlgorithm)
			if err != nil {
				fmt.Printf("Error generating local MSP for %v:\n%v\n", node, err)
				os.Exit(1)
			}
		}
	}
}

func generateOrdererOrg(baseDir string, orgSpec OrgSpec) {
	orgName := orgSpec.Domain

	// generate CAs
	orgDir := filepath.Join(baseDir, "ordererOrganizations", orgName)
	caDir := filepath.Join(orgDir, "ca")
	tlsCADir := filepath.Join(orgDir, "tlsca")
	mspDir := filepath.Join(orgDir, "msp")
	orderersDir := filepath.Join(orgDir, "orderers")
	usersDir := filepath.Join(orgDir, "users")
	adminCertsDir := filepath.Join(mspDir, "admincerts")
	// generate signing CA
	signCA, err := ca.NewCA(caDir, orgName, orgSpec.CA.CommonName, orgSpec.CA.Country, orgSpec.CA.Province, orgSpec.CA.Locality, orgSpec.CA.OrganizationalUnit, orgSpec.CA.StreetAddress, orgSpec.CA.PostalCode, orgSpec.CA.PublicKeyAlgorithm)
	if err != nil {
		fmt.Printf("Error generating signCA for org %s:\n%v\n", orgName, err)
		os.Exit(1)
	}
	// generate TLS CA
	tlsCA, err := ca.NewCA(tlsCADir, orgName, "tls"+orgSpec.CA.CommonName, orgSpec.CA.Country, orgSpec.CA.Province, orgSpec.CA.Locality, orgSpec.CA.OrganizationalUnit, orgSpec.CA.StreetAddress, orgSpec.CA.PostalCode, orgSpec.CA.PublicKeyAlgorithm)
	if err != nil {
		fmt.Printf("Error generating tlsCA for org %s:\n%v\n", orgName, err)
		os.Exit(1)
	}

	err = msp.GenerateVerifyingMSP(mspDir, signCA, tlsCA, orgSpec.EnableNodeOUs, orgSpec.CA.PublicKeyAlgorithm)
	if err != nil {
		fmt.Printf("Error generating MSP for org %s:\n%v\n", orgName, err)
		os.Exit(1)
	}

	generateNodes(orderersDir, orgSpec.Specs, signCA, tlsCA, msp.ORDERER, orgSpec.EnableNodeOUs)

	adminUser := NodeSpec{
		isAdmin:            true,
		CommonName:         fmt.Sprintf("%s@%s", adminBaseName, orgName),
		PublicKeyAlgorithm: ECDSA,
	}

	// generate an admin for the orderer org
	users := []NodeSpec{}
	// add an admin user
	users = append(users, adminUser)
	generateNodes(usersDir, users, signCA, tlsCA, msp.CLIENT, orgSpec.EnableNodeOUs)

	// copy the admin cert to the org's MSP admincerts
	if !orgSpec.EnableNodeOUs {
		err = copyAdminCert(usersDir, adminCertsDir, adminUser.CommonName)
		if err != nil {
			fmt.Printf("Error copying admin cert for org %s:\n%v\n",
				orgName, err)
			os.Exit(1)
		}
	}

	// copy the admin cert to each of the org's orderers's MSP admincerts
	for _, spec := range orgSpec.Specs {
		if orgSpec.EnableNodeOUs {
			continue
		}
		err = copyAdminCert(usersDir,
			filepath.Join(orderersDir, spec.CommonName, "msp", "admincerts"), adminUser.CommonName)
		if err != nil {
			fmt.Printf("Error copying admin cert for org %s orderer %s:\n%v\n",
				orgName, spec.CommonName, err)
			os.Exit(1)
		}
	}
}

func copyFile(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()
	out, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer out.Close()
	_, err = io.Copy(out, in)
	cerr := out.Close()
	if err != nil {
		return err
	}
	return cerr
}

func printVersion() {
	fmt.Println(metadata.GetVersionInfo())
}

func getCA(caDir string, spec OrgSpec, name string) *ca.CA {
	priv, _ := csp.LoadPrivateKey(caDir)
	cert, _ := ca.LoadCertificate(caDir)

	return &ca.CA{
		Name:               name,
		Signer:             ca.GetSignerFromPrivateKey(priv),
		SignCert:           cert,
		Country:            spec.CA.Country,
		Province:           spec.CA.Province,
		Locality:           spec.CA.Locality,
		OrganizationalUnit: spec.CA.OrganizationalUnit,
		StreetAddress:      spec.CA.StreetAddress,
		PostalCode:         spec.CA.PostalCode,
	}
}

func getPublicKeyAlg(pubAlgFromConfig string) (publicKeyAlg string) {
	if pubAlgFromConfig == "" {
		publicKeyAlg = ECDSA
	} else {
		publicKeyAlg = pubAlgFromConfig
	}
	return
}
