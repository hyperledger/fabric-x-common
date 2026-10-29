/*
Copyright IBM Corp. All Rights Reserved.

SPDX-License-Identifier: Apache-2.0
*/

package helpers

import (
	"github.com/cockroachdb/errors"
	"google.golang.org/protobuf/proto"

	"github.com/hyperledger/fabric-x-common/api/ordererpb"
	"github.com/hyperledger/fabric-x-common/tools/configtxgen"
)

// UnmarshalArmaSharedConfig unmarshals the ARMA shared config that an orderer
// carries as its consensus metadata. It errors if the orderer's consensus type
// is not ARMA.
func UnmarshalArmaSharedConfig(consensusType string, metadata []byte) (*ordererpb.SharedConfig, error) {
	if consensusType != configtxgen.Arma {
		return nil, errors.Newf("unsupported consensus type %q: expected %q", consensusType, configtxgen.Arma)
	}
	sharedConfig := &ordererpb.SharedConfig{}
	if err := proto.Unmarshal(metadata, sharedConfig); err != nil {
		return nil, errors.Wrap(err, "failed to unmarshal ARMA shared config from consensus metadata")
	}
	return sharedConfig, nil
}
