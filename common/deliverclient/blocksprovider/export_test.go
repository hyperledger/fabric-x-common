/*
Copyright IBM Corp. All Rights Reserved.

SPDX-License-Identifier: Apache-2.0
*/

package blocksprovider

// LaunchHeaderReceivers runs a single pass of the monitor's header receiver launch loop.
func LaunchHeaderReceivers(m *BFTCensorshipMonitor) error {
	return m.launchHeaderReceivers()
}
