/*
Copyright IBM Corp. All Rights Reserved.

SPDX-License-Identifier: Apache-2.0
*/

package cryptogen

import (
	"net"
	"os"
	"path"
	"strconv"
	"strings"

	"github.com/cockroachdb/errors"
	"github.com/hyperledger/fabric-protos-go-apiv2/common"

	"github.com/hyperledger/fabric-x-common/api/types"
	"github.com/hyperledger/fabric-x-common/common/viperutil"
	"github.com/hyperledger/fabric-x-common/sampleconfig"
	"github.com/hyperledger/fabric-x-common/tools/configtxgen"
)

// ConfigBlockParameters represents the configuration of the config block.
type ConfigBlockParameters struct {
	TargetPath                   string
	BaseProfile                  string
	ChannelID                    string
	Organizations                []OrganizationParameters
	MetaNamespaceVerificationKey []byte
	ArmaMetaBytes                []byte
}

// OrganizationParameters represents the properties of an organization.
// The Name field will also be used for MspID and organization ID.
type OrganizationParameters struct {
	Name             string
	Domain           string
	OrdererEndpoints []OrdererEndpoint
	ConsenterNodes   []Node
	OrdererNodes     []Node
	PeerNodes        []Node
}

// Node describe an organization node.
type Node struct {
	CommonName string
	Hostname   string
	Party      string
	SANS       []string
}

// OrdererEndpoint address should be in the format of <host>:<port>, not the full [types.OrdererEndpoint] format.
type OrdererEndpoint struct {
	Address string
	API     []string
}

// file names.
const (
	ConfigBlockFileName = "config-block.pb.bin"
	metaNamespaceFile   = "meta-namespace-cert.pem"
	armaDataFile        = "arma.pb.bin"
)

// LoadSampleConfig returns the orderer/application config combination that corresponds to
// a given profile.
func LoadSampleConfig(profile string) (*configtxgen.Profile, error) {
	config := viperutil.New()
	err := config.ReadConfig(strings.NewReader(sampleconfig.DefaultYaml))
	if err != nil {
		return nil, errors.Wrap(err, "failed to read config")
	}

	conf := &configtxgen.TopLevel{}
	err = config.EnhancedExactUnmarshal(conf)
	if err != nil {
		return nil, errors.Wrap(err, "error unmarshalling config into struct")
	}

	result, ok := conf.Profiles[profile]
	if !ok {
		return nil, errors.Errorf("could not find profile: %s", profile)
	}
	return result, nil
}

// CreateDefaultConfigBlockWithCrypto creates a config block with default values and a crypto material.
// It uses the first orderer organization as a template and creates the given organizations.
// It uses the same organizations for the orderer and the application.
func CreateDefaultConfigBlockWithCrypto(conf ConfigBlockParameters) (*common.Block, error) {
	if conf.BaseProfile == "" {
		conf.BaseProfile = configtxgen.SampleFabricX
	}
	if conf.ChannelID == "" {
		conf.ChannelID = "chan"
	}
	profile, err := LoadSampleConfig(conf.BaseProfile)
	if err != nil {
		return nil, err
	}

	if len(profile.Orderer.Organizations) < 1 {
		return nil, errors.Errorf("no orderer organizations in selected profile: %s", conf.BaseProfile)
	}

	sourceOrg := *profile.Orderer.Organizations[0]

	profile.Consortiums = nil
	profile.Orderer.ConsenterMapping = make([]*configtxgen.Consenter, 0, len(conf.Organizations))
	profile.Orderer.Organizations = make([]*configtxgen.Organization, 0, len(conf.Organizations))
	profile.Application.Organizations = make([]*configtxgen.Organization, 0, len(conf.Organizations))
	cryptoConf := &Config{}
	for i, o := range conf.Organizations {
		spec := createOrgSpec(&o)

		id := uint32(i) //nolint:gosec // int -> uint32.
		org, orgErr := createOrg(id, sourceOrg, &o)
		if orgErr != nil {
			return nil, orgErr
		}

		profile.Orderer.ConsenterMapping = append(profile.Orderer.ConsenterMapping, createConsenter(id, &o)...)

		switch orgOU(&o) {
		case PeerOU:
			profile.Application.Organizations = append(profile.Application.Organizations, org)
			cryptoConf.PeerOrgs = append(cryptoConf.PeerOrgs, spec)
		case OrdererOU:
			profile.Orderer.Organizations = append(profile.Orderer.Organizations, org)
			cryptoConf.OrdererOrgs = append(cryptoConf.OrdererOrgs, spec)
		default:
			profile.Application.Organizations = append(profile.Application.Organizations, org)
			profile.Orderer.Organizations = append(profile.Orderer.Organizations, org)
			cryptoConf.GenericOrgs = append(cryptoConf.GenericOrgs, spec)
		}
	}

	err = os.WriteFile(path.Join(conf.TargetPath, metaNamespaceFile), conf.MetaNamespaceVerificationKey, 0o644)
	if err != nil {
		return nil, errors.Wrap(err, "failed to write meta namespace file")
	}
	profile.Application.MetaNamespaceVerificationKeyPath = metaNamespaceFile
	err = os.WriteFile(path.Join(conf.TargetPath, armaDataFile), conf.ArmaMetaBytes, 0o644)
	if err != nil {
		return nil, errors.Wrap(err, "failed to write ARMA data file")
	}
	profile.Orderer.Arma.Path = armaDataFile

	err = Generate(conf.TargetPath, cryptoConf)
	if err != nil {
		return nil, err
	}

	profile.CompleteInitialization(conf.TargetPath)

	block, err := configtxgen.GetOutputBlock(profile, conf.ChannelID)
	if err != nil {
		return nil, errors.Wrap(err, "failed to get output block")
	}
	err = configtxgen.WriteOutputBlock(block, path.Join(conf.TargetPath, ConfigBlockFileName))
	return block, errors.Wrap(err, "failed to write block")
}

func orgOU(o *OrganizationParameters) string {
	ordererNodeCount := len(o.ConsenterNodes) + len(o.OrdererNodes)
	peerNodeCount := len(o.PeerNodes)
	switch {
	case ordererNodeCount > 0 && peerNodeCount == 0:
		return OrdererOU
	case ordererNodeCount == 0 && peerNodeCount > 0:
		return PeerOU
	default:
		return "all"
	}
}

func createOrgSpec(o *OrganizationParameters) OrgSpec {
	ordererNodeCount := len(o.ConsenterNodes) + len(o.OrdererNodes)
	peerNodeCount := len(o.PeerNodes)
	nodeSpecs := make([]NodeSpec, 0, ordererNodeCount+peerNodeCount)
	for _, n := range o.ConsenterNodes {
		nodeSpecs = append(nodeSpecs, NodeSpec{
			CommonName:         n.CommonName,
			Hostname:           n.Hostname,
			SANS:               n.SANS,
			Party:              n.Party,
			OrganizationalUnit: OrdererOU,
		})
	}
	for _, n := range o.OrdererNodes {
		nodeSpecs = append(nodeSpecs, NodeSpec{
			CommonName:         n.CommonName,
			Hostname:           n.Hostname,
			SANS:               n.SANS,
			Party:              n.Party,
			OrganizationalUnit: OrdererOU,
		})
	}
	for _, n := range o.PeerNodes {
		nodeSpecs = append(nodeSpecs, NodeSpec{
			CommonName:         n.CommonName,
			Hostname:           n.Hostname,
			SANS:               n.SANS,
			Party:              n.Party,
			OrganizationalUnit: PeerOU,
		})
	}

	return OrgSpec{
		Name:   o.Name,
		Domain: o.Domain,
		CA: NodeSpec{
			Hostname:   "ca." + o.Domain,
			CommonName: o.Name + "-CA",
		},
		Users: UsersSpec{
			Specs: []UserSpec{
				{Name: "client"},
			},
		},
		Specs: nodeSpecs,
	}
}

func createOrg(
	id uint32, sourceOrg configtxgen.Organization, o *OrganizationParameters,
) (*configtxgen.Organization, error) {
	org := sourceOrg
	org.ID = o.Name
	org.Name = o.Name
	org.MSPDir = path.Join(getOrgPath(o), MSPDir)
	org.OrdererEndpoints = make([]*types.OrdererEndpoint, len(o.OrdererEndpoints))
	for epIdx, ep := range o.OrdererEndpoints {
		var err error
		org.OrdererEndpoints[epIdx], err = newOrdererEndpoint(id, ep.Address, o.Name, ep.API)
		if err != nil {
			return nil, errors.Wrapf(err, "invalid %+v", ep)
		}
	}
	org.Policies = make(map[string]*configtxgen.Policy)
	for k, p := range sourceOrg.Policies {
		org.Policies[k] = &configtxgen.Policy{
			Type: p.Type,
			Rule: strings.ReplaceAll(p.Rule, sourceOrg.Name, o.Name),
		}
	}
	return &org, nil
}

func createConsenter(id uint32, o *OrganizationParameters) []*configtxgen.Consenter {
	consenter := make([]*configtxgen.Consenter, len(o.ConsenterNodes))
	for i, n := range o.ConsenterNodes {
		// We use the org's admin certificate as the consenter nodes.
		identity := path.Join(getOrgPath(o), OrdererNodesDir, n.Party, n.CommonName, MSPDir,
			SignCertsDir, n.CommonName+CertSuffix)
		consenter[i] = &configtxgen.Consenter{
			ID:            id,
			Host:          n.Hostname,
			Port:          8080,
			MSPID:         o.Name,
			Identity:      identity,
			ClientTLSCert: identity,
			ServerTLSCert: identity,
		}
	}
	return consenter
}

func getOrgPath(o *OrganizationParameters) string {
	switch orgOU(o) {
	case PeerOU:
		return path.Join(PeerOrganizationsDir, o.Name)
	case OrdererOU:
		return path.Join(OrdererOrganizationsDir, o.Name)
	default:
		return path.Join(GenericOrganizationsDir, o.Name)
	}
}

func newOrdererEndpoint(id uint32, address, name string, api []string) (*types.OrdererEndpoint, error) {
	host, portStr, err := net.SplitHostPort(address)
	if err != nil {
		return nil, errors.Wrapf(err, "invalid address: %s", address)
	}
	port, err := strconv.Atoi(portStr)
	if err != nil {
		return nil, errors.Wrapf(err, "invalid port: %s", portStr)
	}
	return &types.OrdererEndpoint{
		Host:  host,
		Port:  port,
		MspID: name,
		ID:    id,
		API:   api,
	}, nil
}
