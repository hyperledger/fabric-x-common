/*
Copyright IBM Corp. All Rights Reserved.

SPDX-License-Identifier: Apache-2.0
*/

package grpcmetrics_test

import (
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/hyperledger/fabric-x-common/common/grpcmetrics/testpb"
)

//nolint:revive // line length exceeds 120 charcters.
//go:generate protoc --proto_path=testpb --go_out=paths=source_relative:testpb --go-grpc_out=paths=source_relative,require_unimplemented_servers=false:testpb testpb/echo.proto

func TestGrpcmetrics(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Grpcmetrics Suite")
}

//go:generate counterfeiter -o fakes/echo_service.go --fake-name EchoServiceServer . echoServiceServer
type echoServiceServer interface {
	testpb.EchoServiceServer
}
