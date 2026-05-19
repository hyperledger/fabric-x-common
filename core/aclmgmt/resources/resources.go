/*
Copyright IBM Corp. All Rights Reserved.

SPDX-License-Identifier: Apache-2.0
*/

// Package resources contains resource names used in fabric for ACL checks.
package resources

const (
	// Peer_Propose is the ACL resource for the peer Propose API.
	// The underscore naming is intentional to preserve compatibility with
	// downstream consumers (e.g., fabric-smart-client) that reference this constant.
	Peer_Propose = "peer/Propose" //nolint:revive,staticcheck
)
