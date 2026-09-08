/*
Copyright IBM Corp. All Rights Reserved.

SPDX-License-Identifier: Apache-2.0
*/

package applicationpb

import (
	"encoding/binary"
	"math/bits"
)

// ASN.1 tags used by asn1_tx_schema.asn.
const (
	tagInteger     = 0x02
	tagOctetString = 0x04
	tagSequence    = 0x30
	tagUTF8String  = 0x0c
)

// QuickASN1Marshal marshals a transaction for a given namespace.
// It uses the schema described in asn1_tx_schema.asn.
//
// It sizes the whole encoding before writing any of it, so the result is one allocation and every
// payload byte is copied exactly once. Every size function therefore has to agree with the appender
// that writes what it measures, so each is grouped with that appender below.
//
// Against encoding/asn1, medians of BenchmarkASN1Marshal over -count=5:
//
//	two read-writes, 32-byte keys and values     7,195 ns/61 allocs   -> 237 ns/1 alloc
//	512 read-writes, 32-byte keys and values   887,108 ns/8477 allocs -> 26,460 ns/1 alloc
//
// With megabyte keys and values the two are a wash -- both are dominated by copying the four
// megabytes, and that case's run-to-run spread of 20-26% is wider than the difference between them
// -- so what is left there is 61 allocations against one.
func (ns *TxNamespace) QuickASN1Marshal(txID string, metadata [][]byte) ([]byte, error) {
	metadataSize := 0
	for _, m := range metadata {
		metadataSize += tlvSize(len(m))
	}

	readsOnlySize := 0
	for _, r := range ns.ReadsOnly {
		readsOnlySize += tlvSize(readSize(r))
	}
	readWritesSize := 0
	for _, rw := range ns.ReadWrites {
		readWritesSize += tlvSize(readWriteSize(rw))
	}
	blindWritesSize := 0
	for _, w := range ns.BlindWrites {
		blindWritesSize += tlvSize(writeSize(w))
	}

	txSize := tlvSize(len(txID)) +
		tlvSize(len(ns.NsId)) +
		tlvSize(positiveIntegerSize(ns.NsVersion)) +
		tlvSize(readsOnlySize) + tlvSize(readWritesSize) + tlvSize(blindWritesSize)
	// A nil metadata is absent; a non-nil empty one is an empty SEQUENCE. That is what
	// encoding/asn1 does for an `optional` field with no default -- it omits the field only when it
	// equals the type's zero value, which for a slice is nil -- and the two encodings have to agree.
	if metadata != nil {
		txSize += tlvSize(metadataSize)
	}

	buf := make([]byte, 0, tlvSize(txSize))
	buf = appendTLVHeader(buf, tagSequence, txSize)
	buf = appendTLVString(buf, txID)
	if metadata != nil {
		buf = appendTLVHeader(buf, tagSequence, metadataSize)
		for _, m := range metadata {
			buf = appendTLVBytes(buf, m)
		}
	}
	buf = appendTLVString(buf, ns.NsId)
	buf = appendTLVPositiveInteger(buf, ns.NsVersion)

	buf = appendTLVHeader(buf, tagSequence, readsOnlySize)
	for _, r := range ns.ReadsOnly {
		buf = appendTLVHeader(buf, tagSequence, readSize(r))
		buf = appendTLVBytes(buf, r.Key)
		if r.Version != nil {
			buf = appendTLVPositiveInteger(buf, *r.Version)
		}
	}

	buf = appendTLVHeader(buf, tagSequence, readWritesSize)
	for _, rw := range ns.ReadWrites {
		buf = appendTLVHeader(buf, tagSequence, readWriteSize(rw))
		buf = appendTLVBytes(buf, rw.Key)
		buf = appendTLVBytes(buf, rw.Value)
		if rw.Version != nil {
			buf = appendTLVPositiveInteger(buf, *rw.Version)
		}
	}

	buf = appendTLVHeader(buf, tagSequence, blindWritesSize)
	for _, w := range ns.BlindWrites {
		buf = appendTLVHeader(buf, tagSequence, writeSize(w))
		buf = appendTLVBytes(buf, w.Key)
		buf = appendTLVBytes(buf, w.Value)
	}

	return buf, nil
}

// readSize returns the content size of a Read's sequence: a key and an optional version.
// A nil version is the schema's default and is omitted.
func readSize(r *Read) int {
	size := tlvSize(len(r.Key))
	if r.Version != nil {
		size += tlvSize(positiveIntegerSize(*r.Version))
	}
	return size
}

// readWriteSize returns the content size of a ReadWrite's sequence.
func readWriteSize(rw *ReadWrite) int {
	size := tlvSize(len(rw.Key)) + tlvSize(len(rw.Value))
	if rw.Version != nil {
		size += tlvSize(positiveIntegerSize(*rw.Version))
	}
	return size
}

// writeSize returns the content size of a Write's sequence.
func writeSize(w *Write) int {
	return tlvSize(len(w.Key)) + tlvSize(len(w.Value))
}

// tlvSize returns the encoded size of a TLV -- a tag-length-value triplet -- whose value occupies
// valueSize bytes. The length's own size has to agree with what appendTLVHeader emits below: one
// byte in the short format, and one plus the length's own bytes in the long one.
func tlvSize(valueSize int) int {
	lengthSize := 1
	if valueSize >= 128 {
		lengthSize += byteLen(valueSize)
	}
	return 1 + lengthSize + valueSize
}

// appendTLVBytes appends a TLV with an OCTET STRING tag.
func appendTLVBytes(buf, data []byte) []byte {
	return append(appendTLVHeader(buf, tagOctetString, len(data)), data...)
}

// appendTLVString appends a TLV with a UTF8String tag. The schema forces UTF8 rather than letting
// the encoder pick PrintableString for simple strings.
func appendTLVString(buf []byte, value string) []byte {
	return append(appendTLVHeader(buf, tagUTF8String, len(value)), value...)
}

// appendTLVPositiveInteger appends a TLV with an INTEGER tag, big-endian, positive values only.
func appendTLVPositiveInteger(buf []byte, value uint64) []byte {
	size := positiveIntegerSize(value)
	buf = appendTLVHeader(buf, tagInteger, size)
	for shift := (size - 1) * 8; shift >= 0; shift -= 8 {
		// The leading byte is zero whenever positiveIntegerSize added one for the sign.
		buf = append(buf, byte(value>>uint(shift))) //nolint:gosec // shift is bounded by size.
	}
	return buf
}

// appendTLVHeader appends a tag and a length, per the ASN.1 TLV schema:
//   - tag (byte).
//   - length (variable size length of the value).
//   - value (the value's bytes, appended by the caller).
//
// The length is a variable size number, whose MSB indicates whether the short or long format is
// used.
//   - [0] short  Single byte where the remaining bits encode the length (max 127).
//   - [1] long   The remaining bits of the first byte encode the size of the encoded length,
//     followed by the said number of bytes encoding the length in big-endian.
func appendTLVHeader(buf []byte, tag byte, valueSize int) []byte {
	buf = append(buf, tag)
	if valueSize < 128 {
		// [0] short - MSB is already zero.
		return append(buf, byte(valueSize)) //nolint:gosec // bounded by the branch.
	}

	var tmp [8]byte
	binary.BigEndian.PutUint64(tmp[:], uint64(valueSize))
	size := byteLen(valueSize)
	// [1] long - We add the encoding size and mark the MSB to indicate we use the long format.
	//nolint:gosec // size is at most 8, so the tag byte cannot overflow.
	return append(append(buf, 0x80|byte(size)), tmp[len(tmp)-size:]...)
}

// byteLen returns the number of bytes the big-endian encoding of a non-negative l occupies.
func byteLen(l int) int {
	return (bits.Len(uint(l)) + 7) / 8
}

// positiveIntegerSize returns the number of bytes a positive INTEGER's value occupies: the value's
// own bytes, plus a leading zero byte whenever its top bit is set, which keeps it positive. Dividing
// the bit length by eight yields both -- a value whose bit length is a multiple of eight is exactly
// the one that fills its top byte -- and zero, which ASN.1 encodes as a single zero byte.
func positiveIntegerSize(value uint64) int {
	return bits.Len64(value)/8 + 1
}
