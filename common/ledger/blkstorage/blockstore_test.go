/*
Copyright IBM Corp. 2016 All Rights Reserved.

SPDX-License-Identifier: Apache-2.0
*/

package blkstorage

import (
	"crypto/rand"
	"fmt"
	"testing"

	"github.com/hyperledger/fabric-lib-go/common/flogging"
	"github.com/hyperledger/fabric-protos-go-apiv2/common"
	"github.com/hyperledger/fabric-protos-go-apiv2/peer"
	"github.com/stretchr/testify/require"

	"github.com/hyperledger/fabric-x-common/common/configtx/test"
	"github.com/hyperledger/fabric-x-common/common/ledger/testutil"
	"github.com/hyperledger/fabric-x-common/internaltools/pkg/txflags"
	"github.com/hyperledger/fabric-x-common/protoutil"
)

func TestWrongBlockNumber(t *testing.T) {
	env := newTestEnv(t, NewConf(t.TempDir(), 0))
	defer env.Cleanup()

	provider := env.provider
	store, _ := provider.Open("testLedger")
	defer store.Shutdown()

	blocks := testutil.ConstructTestBlocks(t, 5)
	for i := 0; i < 3; i++ {
		err := store.AddBlock(blocks[i])
		require.NoError(t, err)
	}
	err := store.AddBlock(blocks[4])
	require.Error(t, err, "Error shold have been thrown when adding block number 4 while block number 3 is expected")
}

func TestTxIDIndexErrorPropagations(t *testing.T) {
	env := newTestEnv(t, NewConf(t.TempDir(), 0))
	defer env.Cleanup()

	provider := env.provider
	store, _ := provider.Open("testLedger")
	defer store.Shutdown()
	blocks := testutil.ConstructTestBlocks(t, 3)
	for i := 0; i < 3; i++ {
		err := store.AddBlock(blocks[i])
		require.NoError(t, err)
	}

	index := store.fileMgr.db

	txIDBasedFunctions := []func() error{
		func() error {
			_, err := store.RetrieveTxByID("junkTxID")
			return err
		},
		func() error {
			_, err := store.RetrieveBlockByTxID("junkTxID")
			return err
		},
		func() error {
			_, _, err := store.RetrieveTxValidationCodeByTxID("junkTxID")
			return err
		},
	}

	index.Put(
		constructTxIDKey("junkTxID", 5, 4),
		[]byte("junkValue"),
		false,
	)
	expectedErrMsg := fmt.Sprintf("unexpected error while unmarshalling bytes [%#v] into TxIDIndexValProto:", []byte("junkValue"))
	for _, f := range txIDBasedFunctions {
		err := f()
		require.Error(t, err)
		require.Contains(t, err.Error(), expectedErrMsg)
	}

	env.provider.leveldbProvider.Close()
	expectedErrMsg = "error while trying to retrieve transaction info by TXID [junkTxID]:"
	for _, f := range txIDBasedFunctions {
		err := f()
		require.Error(t, err)
		require.Contains(t, err.Error(), expectedErrMsg)
	}
}

func BenchmarkAddBlock(b *testing.B) { //nolint:gocognit
	flogging.ActivateSpec("error")
	defer flogging.ActivateSpec("blkstorage=debug")

	numTx := 500
	txSize := 300
	flushInterval := 100

	cases := []struct {
		name string
		sync bool
	}{
		{name: "Sync", sync: true},
		{name: "NoSync", sync: false},
	}

	for _, bc := range cases {
		b.Run(bc.name, func(b *testing.B) {
			blocks := constructBenchmarkBlocks(b, b.N+1, numTx, txSize)

			env := newTestEnv(b, NewConf(b.TempDir(), 0))
			defer env.Cleanup()
			store, err := env.provider.Open("benchLedger")
			require.NoError(b, err)
			defer store.Shutdown()

			require.NoError(b, store.AddBlock(blocks[0]))

			b.ResetTimer()

			switch bc.sync {
			case true:
				for i := 1; i <= b.N; i++ {
					if err := store.AddBlock(blocks[i]); err != nil {
						b.Fatal(err)
					}
				}
			default:
				for i := 1; i <= b.N; i++ {
					if err := store.AddBlockNoSync(blocks[i]); err != nil {
						b.Fatal(err)
					}
					if i%flushInterval == 0 {
						if err := store.Flush(); err != nil {
							b.Fatal(err)
						}
					}
				}
				if err := store.Flush(); err != nil {
					b.Fatal(err)
				}
			}

			b.StopTimer()
		})
	}
}

func constructBenchmarkBlocks(b *testing.B, n, numTx, txSize int) []*common.Block {
	b.Helper()
	blocks := make([]*common.Block, 0, n)

	gb, err := test.MakeGenesisBlock("benchmarkchannel")
	require.NoError(b, err)
	gb.Metadata.Metadata[common.BlockMetadataIndex_TRANSACTIONS_FILTER] = txflags.NewWithValues(
		len(gb.Data.Data), peer.TxValidationCode_VALID)
	blocks = append(blocks, gb)

	prevHash := protoutil.BlockHeaderHash(gb.Header)
	for blockNum := 1; blockNum < n; blockNum++ {
		simulationResults := make([][]byte, numTx)
		for i := range simulationResults {
			simulationResults[i] = make([]byte, txSize)
			_, err := rand.Read(simulationResults[i])
			require.NoError(b, err)
		}

		envs := make([]*common.Envelope, numTx)
		for i, sr := range simulationResults {
			env, _, err := testutil.ConstructTransactionFromTxDetails(
				&testutil.TxDetails{
					ChaincodeName:     "bench",
					ChaincodeVersion:  "v1",
					SimulationResults: sr,
					Type:              common.HeaderType_ENDORSER_TRANSACTION,
				},
				false,
			)
			require.NoError(b, err)
			envs[i] = env
		}

		block := testutil.NewBlock(envs, uint64(blockNum), prevHash) //nolint:gosec // int -> uint64
		blocks = append(blocks, block)
		prevHash = protoutil.BlockHeaderHash(block.Header)
	}
	return blocks
}
