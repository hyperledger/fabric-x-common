/*
Copyright IBM Corp. All Rights Reserved.

SPDX-License-Identifier: Apache-2.0
*/

package deliver_test

import (
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/hyperledger/fabric-x-common/common/deliver"
)

//go:generate counterfeiter -o mock/filtered_response_sender.go -fake-name FilteredResponseSender . filteredResponseSender

type filteredResponseSender interface {
	deliver.ResponseSender
	deliver.Filtered
}

//go:generate counterfeiter -o mock/private_data_response_sender.go -fake-name PrivateDataResponseSender . privateDataResponseSender

type privateDataResponseSender interface {
	deliver.ResponseSender
}

func TestDeliver(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Deliver Suite")
}
