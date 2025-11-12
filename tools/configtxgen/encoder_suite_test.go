/*
Copyright IBM Corp. All Rights Reserved.

SPDX-License-Identifier: Apache-2.0
*/

package configtxgen_test

import (
	"testing"

	"github.com/hyperledger/fabric-lib-go/bccsp/factory"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/stretchr/testify/require"
)

func TestEncoder(t *testing.T) {
	err := factory.InitFactories(nil)
	require.NoError(t, err)

	RegisterFailHandler(Fail)
	RunSpecs(t, "Encoder Suite")
}
