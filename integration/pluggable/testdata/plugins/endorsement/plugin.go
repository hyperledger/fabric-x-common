/*
Copyright IBM Corp. All Rights Reserved.

SPDX-License-Identifier: Apache-2.0
*/

package main

import (
	endorsement "github.ibm.com/decentralized-trust-research/fabricx-config/core/handlers/endorsement/api"
	"github.ibm.com/decentralized-trust-research/fabricx-config/core/handlers/endorsement/builtin"
	"github.ibm.com/decentralized-trust-research/fabricx-config/integration/pluggable"
)

// go build -buildmode=plugin -o plugin.so

// NewPluginFactory is the function ran by the plugin infrastructure to create an endorsement plugin factory.
func NewPluginFactory() endorsement.PluginFactory {
	pluggable.PublishEndorsementPluginActivation()
	return &builtin.DefaultEndorsementFactory{}
}
