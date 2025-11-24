/*
Copyright IBM Corp. All Rights Reserved.

SPDX-License-Identifier: Apache-2.0
*/

package protoutil

import "github.com/hyperledger/fabric-x-common/api/protocommon"

func NewConfigGroup() *protocommon.ConfigGroup {
	return &protocommon.ConfigGroup{
		Groups:   make(map[string]*protocommon.ConfigGroup),
		Values:   make(map[string]*protocommon.ConfigValue),
		Policies: make(map[string]*protocommon.ConfigPolicy),
	}
}
