/*
Copyright IBM Corp. All Rights Reserved.

SPDX-License-Identifier: Apache-2.0
*/

// Package change defines the `fxadmin modify party` requests.
package change

// Node is a `modify party node` request. It selects one node of a party
// (Role, plus Shard for a batcher) and carries the fields to change; an empty
// field is left unchanged.
type Node struct {
	Party     uint32
	Role      string
	Shard     uint32
	Host      string
	Port      uint32
	TLSCert   string
	SignCert  string
	BlockPath string
}

// CA is a `modify party ca add|remove|set` request over a party's CA
// (SignCerts) and TLS-CA (TLSCerts) certificate lists. Op is the sub-verb.
// An empty field is left unchanged.
type CA struct {
	Op        string
	Party     uint32
	SignCerts []string
	TLSCerts  []string
	BlockPath string
}
