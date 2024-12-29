/*
Copyright IBM Corp. All Rights Reserved.

SPDX-License-Identifier: Apache-2.0
*/

package extcc_test

import (
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.ibm.com/decentralized-trust-research/fabricx-config/core/chaincode/extcc"
)

func TestExtcc(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Chaincode Suite")
}

//go:generate counterfeiter -o mock/ccstreamhandler.go --fake-name StreamHandler . StreamHandler
type chaincodeStreamHandler interface {
	extcc.StreamHandler
}
