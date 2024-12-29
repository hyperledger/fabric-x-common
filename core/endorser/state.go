/*
Copyright IBM Corp. 2018 All Rights Reserved.

SPDX-License-Identifier: Apache-2.0
*/

package endorser

import (
	"github.com/hyperledger/fabric-protos-go-apiv2/ledger/rwset"
	"github.com/pkg/errors"
	endorsement "github.ibm.com/decentralized-trust-research/fabricx-config/core/handlers/endorsement/api/state"
	"github.ibm.com/decentralized-trust-research/fabricx-config/core/ledger"
	"github.ibm.com/decentralized-trust-research/fabricx-config/core/transientstore"
)

// QueryCreator creates new QueryExecutors
//
//go:generate mockery -dir . -name QueryCreator -case underscore -output mocks/
type QueryCreator interface {
	NewQueryExecutor() (ledger.QueryExecutor, error)
}

// ChannelState defines state operations
type ChannelState struct {
	*transientstore.Store
	QueryCreator
}

// FetchState fetches state
func (cs *ChannelState) FetchState() (endorsement.State, error) {
	qe, err := cs.NewQueryExecutor()
	if err != nil {
		return nil, err
	}

	return &StateContext{
		QueryExecutor: qe,
		Store:         cs.Store,
	}, nil
}

// StateContext defines an execution context that interacts with the state
type StateContext struct {
	*transientstore.Store
	ledger.QueryExecutor
}

// GetTransientByTXID returns the private data associated with this transaction ID.
func (sc *StateContext) GetTransientByTXID(txID string) ([]*rwset.TxPvtReadWriteSet, error) {
	scanner, err := sc.Store.GetTxPvtRWSetByTxid(txID, nil)
	if err != nil {
		return nil, errors.WithStack(err)
	}
	defer scanner.Close()
	var data []*rwset.TxPvtReadWriteSet
	for {
		res, err := scanner.Next()
		if err != nil {
			return nil, errors.WithStack(err)
		}
		if res == nil {
			break
		}
		if res.PvtSimulationResultsWithConfig == nil {
			continue
		}
		data = append(data, res.PvtSimulationResultsWithConfig.PvtRwset)
	}
	return data, nil
}
