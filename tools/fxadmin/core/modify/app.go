/*
Copyright IBM Corp. All Rights Reserved.

SPDX-License-Identifier: Apache-2.0
*/

package modify

import (
	"bytes"
	"encoding/json"
	"os"

	"github.com/cockroachdb/errors"
	cb "github.com/hyperledger/fabric-protos-go-apiv2/common"
	"go.yaml.in/yaml/v3"

	"github.com/hyperledger/fabric-x-common/common/channelconfig"
	"github.com/hyperledger/fabric-x-common/protolator"
	"github.com/hyperledger/fabric-x-common/protolator/protoext/peerext"
)

// orgNameKey is the key of an organization definition that holds the org's
// name: the key of its group under the Application group. Every other key of
// the definition belongs to the group itself.
const orgNameKey = "Name"

// AppAdd implements `fxadmin modify app add`. The organization definition at
// orgPath is the org's config group exactly as the block stores it, plus its
// Name; the group is added as is, under that name, to the Application group of
// the config block at blockPath, which is rewritten in place. An org whose
// name is already in the Application group is rejected.
func (*Handler) AppAdd(orgPath, blockPath string) error {
	logger.Debugf("modify app add: org=%s block=%s", orgPath, blockPath)

	name, orgGroup, err := loadOrgGroup(orgPath)
	if err != nil {
		return err
	}
	block, err := readConfigBlock(blockPath)
	if err != nil {
		return err
	}
	application, err := applicationGroup(block.Config())
	if err != nil {
		return err
	}
	if _, exists := application.Groups[name]; exists {
		return errors.Newf("application org %q already exists in %q", name, blockPath)
	}
	application.Groups[name] = orgGroup

	if err := writeConfigBlock(block, blockPath); err != nil {
		return err
	}
	logger.Infof("added application org %s to %s", name, blockPath)
	return nil
}

// applicationGroup returns the Application group of config, which holds one
// group per application org.
func applicationGroup(config *cb.Config) (*cb.ConfigGroup, error) {
	application := config.GetChannelGroup().GetGroups()[channelconfig.ApplicationGroupKey]
	if application == nil {
		return nil, errors.Newf("the config has no %s group", channelconfig.ApplicationGroupKey)
	}
	if application.Groups == nil {
		application.Groups = make(map[string]*cb.ConfigGroup)
	}
	return application, nil
}

// loadOrgGroup reads the organization definition YAML at path and returns the
// org's name and config group.
func loadOrgGroup(path string) (string, *cb.ConfigGroup, error) {
	content, err := os.ReadFile(path)
	if err != nil {
		return "", nil, errors.Wrapf(err, "failed to read organization definition %q", path)
	}
	definition := map[string]any{}
	if err = yaml.Unmarshal(content, &definition); err != nil {
		return "", nil, errors.Wrapf(err, "failed to parse organization definition %q", path)
	}
	name, ok := definition[orgNameKey].(string)
	if !ok || name == "" {
		return "", nil, errors.Newf("organization definition %q has no %s", path, orgNameKey)
	}
	delete(definition, orgNameKey)

	// protolator reads JSON, so the rest of the definition is re-encoded as JSON.
	groupJSON, err := json.Marshal(definition)
	if err != nil {
		return "", nil, errors.Wrapf(err, "failed to encode organization definition %q as JSON", path)
	}
	group := &cb.ConfigGroup{}
	orgGroup := &peerext.DynamicApplicationOrgGroup{ConfigGroup: group}
	if err := protolator.DeepUnmarshalJSON(bytes.NewReader(groupJSON), orgGroup); err != nil {
		return "", nil, errors.Wrapf(err, "failed to decode the config group of organization definition %q", path)
	}
	return name, group, nil
}
