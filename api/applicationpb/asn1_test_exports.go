/*
Copyright IBM Corp. All Rights Reserved.

SPDX-License-Identifier: Apache-2.0
*/

package applicationpb

import (
	"encoding/asn1"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/hyperledger/fabric-x-common/utils/test"
)

// TestTx represents a test transaction for ASN1 marshalling tests.
type TestTx struct {
	ID         string
	Metadata   [][]byte
	Namespaces []*TxNamespace
}

// CommonTestAsnMarshal tests ASN1 marshalling and unmarshalling of the given TXs.
// It can be used by other test packages (e.g., loadgen) to ensure consistency with the implementation.
func CommonTestAsnMarshal(t *testing.T, txs []*TestTx) {
	t.Helper()
	for _, tx := range txs {
		t.Run(tx.ID, func(t *testing.T) {
			t.Parallel()
			for _, ns := range tx.Namespaces {
				t.Run(ns.NsId, func(t *testing.T) {
					t.Parallel()
					requireASN1Marshal(t, tx.ID, tx.Metadata, ns)
				})
			}

			// Test that we can reconstruct the TX from the marshalled version.
			// This ensures we didn't lose any data in the translation.
			t.Run("verify reconstruction of the TX", func(t *testing.T) {
				t.Parallel()
				translatedNamespace := make([][]byte, len(tx.Namespaces))
				for i, ns := range tx.Namespaces {
					var err error
					translatedNamespace[i], err = ns.ASN1Marshal(tx.ID, tx.Metadata)
					require.NoError(t, err)
				}
				txID, txMetadata, actualTxNs := reconstructTX(t, translatedNamespace)
				require.Equal(t, tx.ID, txID)
				require.Equal(t, tx.Metadata, txMetadata)
				test.RequireProtoElementsMatch(t, tx.Namespaces, actualTxNs)
			})
		})
	}
}

func requireASN1Marshal(t *testing.T, txID string, metadata [][]byte, ns *TxNamespace) []byte {
	t.Helper()
	translated := ns.translate(txID, metadata)

	// The marshalled object cannot distinguish between empty and nil slice.
	// So, we convert all nils to empty slices to allow comparison with the unmarshalled object.
	for i := range translated.ReadWrites {
		rw := &translated.ReadWrites[i]
		if rw.Value == nil {
			rw.Value = make([]byte, 0)
		}
	}
	for i := range translated.BlindWrites {
		w := &translated.BlindWrites[i]
		if w.Value == nil {
			w.Value = make([]byte, 0)
		}
	}

	derBytes, err := ns.ASN1Marshal(txID, metadata)
	require.NoError(t, err)

	derBytesQuick, err := ns.QuickASN1Marshal(txID, metadata)
	require.NoError(t, err)

	require.Equal(t, derBytes, derBytesQuick)

	actual := &asn1Namespace{}
	_, err = asn1.Unmarshal(derBytes, actual)
	require.NoError(t, err)
	require.Equal(t, translated, actual)

	actualQuick := &asn1Namespace{}
	_, err = asn1.Unmarshal(derBytesQuick, actualQuick)
	require.NoError(t, err)
	require.Equal(t, translated, actualQuick)
	return derBytes
}

// reconstructTX unmarshal the given namespaces and reconstruct a TX.
// Any change to [*applicationpb.Tx] requires a change to this method.
func reconstructTX(t *testing.T, namespaces [][]byte) (txID string, txMetadata [][]byte, txNs []*TxNamespace) {
	t.Helper()
	txNs = make([]*TxNamespace, len(namespaces))

	for i, nsDer := range namespaces {
		translated := &asn1Namespace{}
		_, err := asn1.Unmarshal(nsDer, translated)
		require.NoError(t, err)

		if i == 0 {
			txID = translated.TxID
			txMetadata = translated.Metadata
		} else {
			require.Equal(t, txID, translated.TxID)
			require.Equal(t, txMetadata, translated.Metadata)
		}

		nsVer := asnToProtoVersion(translated.NamespaceVersion)
		require.NotNil(t, nsVer)
		ns := &TxNamespace{
			NsId:        translated.NamespaceID,
			NsVersion:   *nsVer,
			ReadsOnly:   make([]*Read, len(translated.ReadsOnly)),
			ReadWrites:  make([]*ReadWrite, len(translated.ReadWrites)),
			BlindWrites: make([]*Write, len(translated.BlindWrites)),
		}
		txNs[i] = ns

		for j, read := range translated.ReadsOnly {
			ns.ReadsOnly[j] = &Read{
				Key:     read.Key,
				Version: asnToProtoVersion(read.Version),
			}
		}
		for j, rw := range translated.ReadWrites {
			ns.ReadWrites[j] = &ReadWrite{
				Key:     rw.Key,
				Value:   rw.Value,
				Version: asnToProtoVersion(rw.Version),
			}
		}
		for j, w := range translated.BlindWrites {
			ns.BlindWrites[j] = &Write{
				Key:   w.Key,
				Value: w.Value,
			}
		}
	}

	return txID, txMetadata, txNs
}
