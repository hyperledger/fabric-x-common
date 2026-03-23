/*
Copyright IBM Corp. All Rights Reserved.

SPDX-License-Identifier: Apache-2.0
*/

package configtxgen

import (
	"github.com/hyperledger/fabric-lib-go/bccsp/sw"
	cb "github.com/hyperledger/fabric-protos-go-apiv2/common"
	"github.com/onsi/ginkgo/v2"
	"github.com/onsi/gomega"
	"github.com/pkg/errors"

	"github.com/hyperledger/fabric-x-common/common/channelconfig"
	"github.com/hyperledger/fabric-x-common/core/config/configtest"
)

func hasModPolicySet(groupName string, cg *cb.ConfigGroup) error {
	if cg.ModPolicy == "" {
		return errors.Errorf("group %s has empty mod_policy", groupName)
	}

	for valueName, value := range cg.Values {
		if value.ModPolicy == "" {
			return errors.Errorf("group %s has value %s with empty mod_policy", groupName, valueName)
		}
	}

	for policyName, policy := range cg.Policies {
		if policy.ModPolicy == "" {
			return errors.Errorf("group %s has policy %s with empty mod_policy", groupName, policyName)
		}
	}

	for groupName, group := range cg.Groups {
		err := hasModPolicySet(groupName, group)
		if err != nil {
			return errors.WithMessagef(err, "missing sub-mod_policy for group %s", groupName)
		}
	}

	return nil
}

var _ = ginkgo.Describe("Integration", func() {
	ginkgo.DescribeTable("successfully parses the profile",
		func(profile string) {
			config := Load(profile, configtest.GetDevConfigDir())
			config.Capabilities = map[string]bool{"V2_0": true}
			group, err := NewChannelGroup(config)
			gomega.Expect(err).NotTo(gomega.HaveOccurred())

			cryptoProvider, err := sw.NewDefaultSecurityLevelWithKeystore(sw.NewDummyKeyStore())
			gomega.Expect(err).NotTo(gomega.HaveOccurred())
			_, err = channelconfig.NewBundle("test", &cb.Config{
				ChannelGroup: group,
			}, cryptoProvider)
			gomega.Expect(err).NotTo(gomega.HaveOccurred())

			err = hasModPolicySet("Channel", group)
			gomega.Expect(err).NotTo(gomega.HaveOccurred())
		},
		ginkgo.Entry("Sample Insecure Solo Profile", SampleInsecureSoloProfile),
		ginkgo.Entry("Sample Single MSP Solo Profile", SampleSingleMSPSoloProfile),
		ginkgo.Entry("Sample DevMode Solo Profile", SampleDevModeSoloProfile),
	)
})
