/*
Copyright IBM Corp. All Rights Reserved.

SPDX-License-Identifier: Apache-2.0
*/

package modify_test

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	cb "github.com/hyperledger/fabric-protos-go-apiv2/common"
	"github.com/stretchr/testify/require"
	"go.yaml.in/yaml/v3"

	"github.com/hyperledger/fabric-x-common/common/channelconfig"
	"github.com/hyperledger/fabric-x-common/protolator"
	"github.com/hyperledger/fabric-x-common/protolator/protoext/peerext"
	"github.com/hyperledger/fabric-x-common/protoutil"
	"github.com/hyperledger/fabric-x-common/tools/configtxgen"
	"github.com/hyperledger/fabric-x-common/tools/cryptogen"
	"github.com/hyperledger/fabric-x-common/tools/fxadmin/core/helpers"
	"github.com/hyperledger/fabric-x-common/tools/fxadmin/core/modify"
	"github.com/hyperledger/fabric-x-common/utils/test"
	"github.com/hyperledger/fabric-x-common/utils/testcrypto"
)

const (
	// newOrg is the name and MSP ID of the org the tests add.
	newOrg = "peer2"

	// Formats an organization definition is written in.
	formatJSON = "json"
	formatYAML = "yaml"

	blockFileName = "next.pb"
	blockFileMode = 0o640

	// existingOrgCount is the number of application orgs in the block the tests edit.
	existingOrgCount = 2

	errNoName = "has no Name"
)

// existingOrgs are the existingOrgCount application orgs of the block the tests
// edit, as testcrypto names them.
var existingOrgs = []string{"peer-org-0", "peer-org-1"}

// TestAppAdd adds an application org from its definition and checks the edited
// block: the org's group is exactly the group of the definition, the existing
// orgs are untouched, the block still loads as a channel config, and the block
// file was replaced in place.
func TestAppAdd(t *testing.T) {
	t.Parallel()

	baseBlock := newConfigBlock(t)
	mspDir := newOrgMSPDir(t)

	for _, tc := range []struct {
		name        string
		format      string
		anchorPeers []*configtxgen.AnchorPeer
	}{
		{
			name:   "JSON definition, as configtxgen -printOrg prints the group",
			format: formatJSON,
		},
		{
			name:   "YAML definition",
			format: formatYAML,
		},
		{
			name:   "org with anchor peers",
			format: formatYAML,
			anchorPeers: []*configtxgen.AnchorPeer{
				{Host: "peer0.peer2.example.com", Port: 7051},
				{Host: "peer1.peer2.example.com", Port: 8051},
			},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			orgGroup := newOrgGroup(t, mspDir, tc.anchorPeers)
			blockPath := writeBlockFile(t, baseBlock)
			orgPath := writeOrgDefinition(t, renderOrgDefinition(t, newOrg, orgGroup, tc.format))

			require.NoError(t, modify.New().AppAdd(orgPath, blockPath))

			// The org is added exactly as defined, and the existing orgs are untouched.
			application := readApplicationGroup(t, blockPath)
			test.RequireProtoEqual(t, orgGroup, application.GetGroups()[newOrg])
			baseApplication := applicationGroupOf(t, baseBlock)
			require.Len(t, application.GetGroups(), len(existingOrgs)+1)
			for _, org := range existingOrgs {
				test.RequireProtoEqual(t, baseApplication.GetGroups()[org], application.GetGroups()[org])
			}

			// The edited block loads as a channel config that includes the new org.
			material, err := channelconfig.LoadConfigBlockMaterialFromFile(blockPath)
			require.NoError(t, err)
			require.ElementsMatch(t, append([]string{newOrg}, existingOrgs...), mspIDsOf(material))

			// The block was replaced in place: same mode, no temporary file left.
			info, err := os.Stat(blockPath)
			require.NoError(t, err)
			require.Equal(t, os.FileMode(blockFileMode), info.Mode().Perm())
			requireOnlyFile(t, filepath.Dir(blockPath), blockFileName)
		})
	}
}

// TestAppAddRejects covers definitions and conflicts that must fail, and
// asserts that a failed add leaves the block file unchanged.
func TestAppAddRejects(t *testing.T) {
	t.Parallel()

	baseBlock := newConfigBlock(t)
	orgGroup := newOrgGroup(t, newOrgMSPDir(t), nil)

	// A valid org group under a name the block rejects.
	for _, tc := range []struct {
		name    string
		orgName string
		wantErr string
	}{
		{
			name:    "org name already in the Application group",
			orgName: "peer-org-0",
			wantErr: `application org "peer-org-0" already exists`,
		},
		{
			name:    "missing Name",
			orgName: "",
			wantErr: errNoName,
		},
		{
			name:    "org name that is not a valid config key",
			orgName: "peer 2",
			wantErr: "the edited config is not a valid channel config",
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			blockPath := writeBlockFile(t, baseBlock)
			orgPath := writeOrgDefinition(t, renderOrgDefinition(t, tc.orgName, orgGroup, formatYAML))

			err := modify.New().AppAdd(orgPath, blockPath)
			require.ErrorContains(t, err, tc.wantErr)
			requireBlockUnchanged(t, blockPath, baseBlock)
		})
	}

	// A malformed definition.
	for _, tc := range []struct {
		name       string
		definition string
		wantErr    string
	}{
		{
			name:       "empty definition",
			definition: "",
			wantErr:    errNoName,
		},
		{
			name:       "Name that is not a string",
			definition: "Name: 5\n",
			wantErr:    errNoName,
		},
		{
			name:       "not YAML",
			definition: "Name: [peer2\n",
			wantErr:    "failed to parse organization definition",
		},
		{
			name:       "key that is not a config group field",
			definition: "Name: peer2\npolcies: {}\n",
			wantErr:    "failed to decode the config group",
		},
		{
			name:       "org group without an MSP",
			definition: "Name: peer2\nmod_policy: Admins\n",
			wantErr:    "the edited config is not a valid channel config",
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			blockPath := writeBlockFile(t, baseBlock)
			orgPath := writeOrgDefinition(t, tc.definition)

			err := modify.New().AppAdd(orgPath, blockPath)
			require.ErrorContains(t, err, tc.wantErr)
			requireBlockUnchanged(t, blockPath, baseBlock)
		})
	}
}

// TestAppAddRejectsNonConfigBlock verifies that a block file that is not a
// config block is rejected without being rewritten.
func TestAppAddRejectsNonConfigBlock(t *testing.T) {
	t.Parallel()

	notABlock := []byte("not a block")
	blockPath := writeBlockFile(t, notABlock)
	orgGroup := newOrgGroup(t, newOrgMSPDir(t), nil)
	orgPath := writeOrgDefinition(t, renderOrgDefinition(t, newOrg, orgGroup, formatYAML))

	err := modify.New().AppAdd(orgPath, blockPath)
	require.ErrorContains(t, err, "failed to decode config block")
	requireBlockUnchanged(t, blockPath, notABlock)
}

// newConfigBlock returns a marshaled config block whose Application group holds
// the existingOrgs.
func newConfigBlock(t *testing.T) []byte {
	t.Helper()
	block, err := testcrypto.CreateOrExtendConfigBlockWithCrypto(t.TempDir(), &testcrypto.ConfigBlock{
		ChannelID:             "mychannel",
		PeerOrganizationCount: existingOrgCount,
	})
	require.NoError(t, err)
	return protoutil.MarshalOrPanic(block)
}

// newOrgMSPDir generates the crypto material of a peer org that is not in the
// block and returns its org-level MSP directory.
func newOrgMSPDir(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	_, err := testcrypto.CreateOrExtendConfigBlockWithCrypto(dir, &testcrypto.ConfigBlock{PeerOrganizationCount: 1})
	require.NoError(t, err)
	return filepath.Join(dir, cryptogen.PeerOrganizationsDir, "peer-org-0.com", cryptogen.MSPDir)
}

// newOrgGroup returns the config group of the newOrg application org with the
// MSP at mspDir, as configtxgen builds it, for a test to write as a definition.
func newOrgGroup(t *testing.T, mspDir string, anchorPeers []*configtxgen.AnchorPeer) *cb.ConfigGroup {
	t.Helper()
	member := &configtxgen.Policy{Type: configtxgen.SignaturePolicyType, Rule: "OR('" + newOrg + ".member')"}
	admin := &configtxgen.Policy{Type: configtxgen.SignaturePolicyType, Rule: "OR('" + newOrg + ".admin')"}
	group, err := configtxgen.NewApplicationOrgGroup(&configtxgen.Organization{
		Name:    newOrg,
		ID:      newOrg,
		MSPDir:  mspDir,
		MSPType: "bccsp",
		Policies: map[string]*configtxgen.Policy{
			channelconfig.ReadersPolicyKey: member,
			channelconfig.WritersPolicyKey: member,
			channelconfig.AdminsPolicyKey:  admin,
			"Endorsement":                  member,
		},
		AnchorPeers: anchorPeers,
	})
	require.NoError(t, err)
	return group
}

// renderOrgDefinition renders an organization definition.
func renderOrgDefinition(t *testing.T, name string, group *cb.ConfigGroup, format string) string {
	t.Helper()
	var groupJSON bytes.Buffer
	require.NoError(t, protolator.DeepMarshalJSON(&groupJSON, &peerext.DynamicApplicationOrgGroup{ConfigGroup: group}))
	definition := map[string]any{}
	require.NoError(t, json.Unmarshal(groupJSON.Bytes(), &definition))
	if name != "" {
		definition["Name"] = name
	}

	var content []byte
	var err error
	if format == formatYAML {
		content, err = yaml.Marshal(definition)
	} else {
		content, err = json.Marshal(definition)
	}
	require.NoError(t, err)
	return string(content)
}

// writeBlockFile writes content as the block file of a fresh directory and
// returns its path.
func writeBlockFile(t *testing.T, content []byte) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), blockFileName)
	require.NoError(t, os.WriteFile(path, content, blockFileMode))
	return path
}

// writeOrgDefinition writes the definition as org.yaml in a temp directory and
// returns its path.
func writeOrgDefinition(t *testing.T, definition string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "org.yaml")
	require.NoError(t, os.WriteFile(path, []byte(definition), 0o600))
	return path
}

// readApplicationGroup returns the Application group of the config block file
// at path.
func readApplicationGroup(t *testing.T, path string) *cb.ConfigGroup {
	t.Helper()
	content, err := os.ReadFile(path)
	require.NoError(t, err)
	return applicationGroupOf(t, content)
}

// applicationGroupOf returns the Application group of a marshaled config block.
func applicationGroupOf(t *testing.T, marshaledBlock []byte) *cb.ConfigGroup {
	t.Helper()
	block, err := helpers.UnmarshalConfigBlock(marshaledBlock)
	require.NoError(t, err)
	application := block.Config().GetChannelGroup().GetGroups()[channelconfig.ApplicationGroupKey]
	require.NotNil(t, application)
	return application
}

// mspIDsOf returns the MSP IDs of the application orgs in loaded config material.
func mspIDsOf(material *channelconfig.ConfigBlockMaterial) []string {
	ids := make([]string, 0, len(material.ApplicationOrganizations))
	for _, org := range material.ApplicationOrganizations {
		ids = append(ids, org.MspID)
	}
	return ids
}

// requireBlockUnchanged asserts that the block file at path still holds want
// and that no temporary file was left next to it.
func requireBlockUnchanged(t *testing.T, path string, want []byte) {
	t.Helper()
	content, err := os.ReadFile(path)
	require.NoError(t, err)
	require.Equal(t, want, content, "a failed add must leave the block unchanged")
	requireOnlyFile(t, filepath.Dir(path), blockFileName)
}

// requireOnlyFile asserts that dir holds exactly one entry, the named file.
func requireOnlyFile(t *testing.T, dir, name string) {
	t.Helper()
	entries, err := os.ReadDir(dir)
	require.NoError(t, err)
	names := make([]string, 0, len(entries))
	for _, entry := range entries {
		names = append(names, entry.Name())
	}
	require.Equal(t, []string{name}, names)
}
