/*
Copyright IBM Corp. All Rights Reserved.

SPDX-License-Identifier: Apache-2.0
*/

package msptesttools

import (
	"github.com/hyperledger/fabric-lib-go/bccsp/factory"

	"github.com/hyperledger/fabric-x-common/core/config/configtest"
	"github.com/hyperledger/fabric-x-common/msp"
	"github.com/hyperledger/fabric-x-common/msp/mgmt"
)

// LoadTestMSPSetup sets up the local MSP
// and a chain MSP for the default chain
func LoadMSPSetupForTesting() error {
	dir := configtest.GetDevMspDir()
	conf, err := msp.GetLocalMspConfig(dir, nil, "SampleOrg")
	if err != nil {
		return err
	}

	err = mgmt.GetLocalMSP(factory.GetDefault()).Setup(conf)
	if err != nil {
		return err
	}

	err = mgmt.GetManagerForChain("testchannelid").Setup([]msp.MSP{mgmt.GetLocalMSP(factory.GetDefault())})
	if err != nil {
		return err
	}

	return nil
}
