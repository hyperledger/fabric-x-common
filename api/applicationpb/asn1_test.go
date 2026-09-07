/*
Copyright IBM Corp. All Rights Reserved.

SPDX-License-Identifier: Apache-2.0
*/

package applicationpb

import (
	"math/rand"
	"testing"
	"unicode/utf8"

	"github.com/stretchr/testify/require"

	"github.com/hyperledger/fabric-x-common/utils/test"
)

func TestAsnMarshal(t *testing.T) {
	t.Parallel()
	CommonTestAsnMarshal(t, []*TestTx{{
		ID:         "some-tx-id",
		Namespaces: txTestCases,
	}})
}

func FuzzASN1MarshalTxNamespace(f *testing.F) {
	i := uint32(1024)
	for _, ns := range txTestCases {
		//nolint:gosec // false positive; safe integer conversion.
		for _, r := range ns.ReadsOnly {
			f.Add(
				"some-tx-id", []byte("some-metadata-1"), []byte("some-metadata-2"),
				ns.NsId, ns.NsVersion, protoToAsnVersion(r.Version),
				uint32(len(ns.ReadsOnly)), uint32(len(ns.ReadWrites)), uint32(len(ns.BlindWrites)),
				i, int64(1234+i),
			)
			i++
		}
		//nolint:gosec // false positive; safe integer conversion.
		for _, r := range ns.ReadWrites {
			f.Add(
				"another-tx-id", []byte("another-metadata-1"), []byte("another-metadata-2"),
				ns.NsId, ns.NsVersion, protoToAsnVersion(r.Version),
				uint32(len(ns.ReadsOnly)), uint32(len(ns.ReadWrites)), uint32(len(ns.BlindWrites)),
				i, int64(1234+i),
			)
			i++
		}
	}
	f.Fuzz(func(
		t *testing.T,
		id string, metadata1, metadata2 []byte, nsID string, nsVersion uint64, readVersion int64,
		rCount, rwCount, wCount uint32,
		maxSize uint32, seed int64,
	) {
		txNs := generateTxNs(t, id, nsID, nsVersion, readVersion, rCount, rwCount, wCount, maxSize, seed)
		metadata := [][]byte{metadata1, metadata2}
		derBytes := requireASN1Marshal(t, id, metadata, txNs)
		actualTxID, actualTxMetadata, actualTxNs := reconstructTX(t, [][]byte{derBytes})
		require.Equal(t, id, actualTxID)
		require.Equal(t, metadata, actualTxMetadata)
		test.RequireProtoElementsMatch(t, []*TxNamespace{txNs}, actualTxNs)
	})
}

// generateTxNs creates a TX with a single namespace given the input parameters.
func generateTxNs( //nolint:revive // required parameters.
	t *testing.T,
	id, nsID string, nsVersion uint64, readVersion int64,
	rCount, rwCount, wCount uint32,
	maxSize uint32, seed int64,
) *TxNamespace {
	t.Helper()
	if !utf8.ValidString(id) || !utf8.ValidString(nsID) {
		t.Skip("invalid UTF8")
	}
	tx := &TxNamespace{
		NsId:        nsID,
		NsVersion:   nsVersion,
		ReadsOnly:   make([]*Read, rCount),
		ReadWrites:  make([]*ReadWrite, rwCount),
		BlindWrites: make([]*Write, wCount),
	}
	rnd := rand.New(rand.NewSource(seed))
	for i := range tx.ReadsOnly {
		tx.ReadsOnly[i] = &Read{
			Key:     mustRead(t, rnd, maxSize),
			Version: asnToProtoVersion(readVersion),
		}
	}
	for i := range tx.ReadWrites {
		tx.ReadWrites[i] = &ReadWrite{
			Key:     mustRead(t, rnd, maxSize),
			Value:   mustRead(t, rnd, maxSize),
			Version: asnToProtoVersion(readVersion),
		}
	}
	for i := range tx.BlindWrites {
		tx.BlindWrites[i] = &Write{
			Key:   mustRead(t, rnd, maxSize),
			Value: mustRead(t, rnd, maxSize),
		}
	}
	return tx
}

var txTestCases = []*TxNamespace{
	{
		NsId:      "empty",
		NsVersion: 1,
	},
	{
		NsId:      "only reads",
		NsVersion: 2,
		ReadsOnly: []*Read{
			{
				Key:     []byte{1},
				Version: new(uint64(2)),
			},
			{
				Key:     []byte{3, 4, 5},
				Version: new(uint64(0)),
			},
		},
	},
	{
		NsId:      "only reads with nil version",
		NsVersion: 2,
		ReadsOnly: []*Read{
			{
				Key:     []byte{1},
				Version: new(uint64(2)),
			},
			{
				Key:     []byte{3, 4, 5},
				Version: new(uint64(0)),
			},
			{
				Key:     []byte{7, 8, 9},
				Version: nil,
			},
		},
	},
	{
		NsId:      "only read-write",
		NsVersion: 3,
		ReadWrites: []*ReadWrite{
			{
				Key:     []byte{1},
				Version: new(uint64(2)),
				Value:   []byte{3},
			},
			{
				Key:     []byte{5},
				Version: new(uint64(0)),
				Value:   []byte{6},
			},
		},
	},
	{
		NsId:      "only read-write with nil value or version",
		NsVersion: 3,
		ReadWrites: []*ReadWrite{
			{
				Key:     []byte{1},
				Version: new(uint64(2)),
				Value:   []byte{3},
			},
			{
				Key:     []byte{7},
				Version: nil,
				Value:   []byte{8},
			},
			{
				Key:     []byte{9},
				Version: new(uint64(3)),
				Value:   nil,
			},
			{
				Key:     []byte{10},
				Version: nil,
				Value:   nil,
			},
		},
	},
	{
		NsId:      "only blind writes",
		NsVersion: 4,
		BlindWrites: []*Write{
			{
				Key:   []byte{5},
				Value: []byte{6, 7},
			},
			{
				Key:   []byte{10},
				Value: make([]byte, 0),
			},
		},
	},
	{
		NsId:      "only blind writes with nil value",
		NsVersion: 4,
		BlindWrites: []*Write{
			{
				Key:   []byte{5},
				Value: []byte{6, 7},
			},
			{
				Key:   []byte{10},
				Value: make([]byte, 0),
			},
			{
				Key:   []byte{11},
				Value: nil,
			},
		},
	},
	{
		NsId:      "all",
		NsVersion: 5,
		ReadsOnly: []*Read{
			{
				Key:     []byte{6},
				Version: new(uint64(7)),
			},
			{
				Key:     []byte{9, 10, 11},
				Version: new(uint64(1)),
			},
		},
		ReadWrites: []*ReadWrite{
			{
				Key:     []byte{100},
				Version: new(uint64(1)),
				Value:   []byte{2},
			},
			{
				Key:     []byte{5},
				Version: new(uint64(1)),
				Value:   []byte{13},
			},
		},
		BlindWrites: []*Write{
			{
				Key:   []byte{1, 2, 3},
				Value: []byte{100, 101, 102},
			},
			{
				Key:   []byte{5},
				Value: []byte{6},
			},
		},
	},
	{
		NsId:      "varying number of items",
		NsVersion: 6,
		ReadsOnly: []*Read{
			{
				Key:     []byte{1, 2},
				Version: new(uint64(1)),
			},
		},
		ReadWrites: []*ReadWrite{
			{
				Key:     []byte{1},
				Version: new(uint64(1)),
				Value:   []byte{3},
			},
			{
				Key:     []byte{4},
				Version: new(uint64(1)),
				Value:   []byte{6},
			},
			{
				Key:     []byte{7},
				Version: new(uint64(1)),
				Value:   []byte{9},
			},
		},
		BlindWrites: []*Write{
			{
				Key:   []byte{10},
				Value: []byte{12},
			},
			{
				Key:   []byte{13},
				Value: []byte{14},
			},
		},
	},
	{
		NsId:      "varying length",
		NsVersion: 6,
		ReadsOnly: []*Read{
			{Key: make([]byte, 127)},
			{Key: make([]byte, 128)},
			{Key: make([]byte, 129)},
			{Key: make([]byte, 255)},
			{Key: make([]byte, 256)},
			{Key: make([]byte, 257)},
			{Key: make([]byte, 0xffff)},
			{Key: make([]byte, 0x10000)},
			{Key: make([]byte, 0x10001)},
			{Key: make([]byte, 0x100000)},
		},
	},
}

// mustRead reads a byte array of the given size from the source.
// It panics if the read fails, or cannot read the requested size.
// "crypto/rand" and "math/rand" never fail and always returns the correct length.
func mustRead(t *testing.T, source *rand.Rand, maxSize uint32) []byte {
	t.Helper()
	size := source.Intn(int(maxSize))
	value := make([]byte, size)
	n, err := source.Read(value)
	require.NoError(t, err)
	require.Equal(t, size, n)
	return value
}

// TestMetadataEdgeCases checks the one input shape FuzzASN1MarshalTxNamespace cannot reach: it always
// passes a two-element metadata slice, so nil and empty are never compared.
func TestMetadataEdgeCases(t *testing.T) {
	t.Parallel()
	ns := &TxNamespace{NsId: "0", NsVersion: 1}
	for _, tc := range []struct {
		name     string
		metadata [][]byte
	}{
		{name: "nil", metadata: nil},
		{name: "empty non-nil", metadata: [][]byte{}},
		{name: "one empty element", metadata: [][]byte{{}}},
		{name: "one element", metadata: [][]byte{[]byte("m")}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			slow, err := ns.ASN1Marshal("tx", tc.metadata)
			require.NoError(t, err)
			quick, err := ns.QuickASN1Marshal("tx", tc.metadata)
			require.NoError(t, err)
			require.Equal(t, slow, quick, "encoding/asn1=%x quick=%x", slow, quick)
		})
	}
}

// BenchmarkASN1Marshal compares the two encoders at the three shapes QuickASN1Marshal's doc comment
// quotes: the shape a committer evaluation generates, the same shape with many operations, and one
// with megabyte keys and values. The large-value case has run-to-run spread of tens of percent, so
// read it from the median of several -count runs rather than from a single result.
func BenchmarkASN1Marshal(b *testing.B) {
	metadata := [][]byte{[]byte("some-metadata-1")}
	for _, bc := range []struct {
		name    string
		rwCount int
		size    int
	}{
		{name: "rw=2/size=32", rwCount: 2, size: 32},
		{name: "rw=512/size=32", rwCount: 512, size: 32},
		{name: "rw=2/size=1MB", rwCount: 2, size: 1 << 20},
	} {
		ns := benchNamespace(bc.rwCount, bc.size)
		b.Run(bc.name+"/encoding-asn1", func(b *testing.B) {
			b.ReportAllocs()
			for range b.N {
				if _, err := ns.ASN1Marshal("some-tx-id", metadata); err != nil {
					b.Fatal(err)
				}
			}
		})
		b.Run(bc.name+"/quick", func(b *testing.B) {
			b.ReportAllocs()
			for range b.N {
				if _, err := ns.QuickASN1Marshal("some-tx-id", metadata); err != nil {
					b.Fatal(err)
				}
			}
		})
	}
}

// benchNamespace builds a namespace of rwCount read-writes, each with a key and a value of size
// bytes.
func benchNamespace(rwCount, size int) *TxNamespace {
	ns := &TxNamespace{NsId: "0", NsVersion: 1, ReadWrites: make([]*ReadWrite, rwCount)}
	for i := range ns.ReadWrites {
		version := uint64(i)
		ns.ReadWrites[i] = &ReadWrite{
			Key:     make([]byte, size),
			Value:   make([]byte, size),
			Version: &version,
		}
	}
	return ns
}
