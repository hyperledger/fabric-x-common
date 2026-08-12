/*
Copyright IBM Corp. All Rights Reserved.

SPDX-License-Identifier: Apache-2.0
*/

package user

import (
	"fmt"
	"os"

	"github.com/cockroachdb/errors"
	"go.yaml.in/yaml/v3"
)

// Config is the user identity and client-TLS material read from the user
// configuration YAML. For reconfiguration the identity must be an
// admin recognized by the orderer's channel policies.
type Config struct {
	MSP MSPConfig `yaml:"msp"`
	TLS TLSConfig `yaml:"tls"`
}

// MSPConfig identifies the local MSP used to sign the seek requests.
type MSPConfig struct {
	LocalMspID  string `yaml:"localMspID"`
	LocalMspDir string `yaml:"localMspDir"`
}

// TLSConfig holds the client-side TLS material. When Enabled is true the client
// dials the router and assembler over TLS; ClientCert and ClientKey enable mutual TLS.
type TLSConfig struct {
	Enabled    *bool  `yaml:"enabled"`
	ClientCert string `yaml:"clientCert"`
	ClientKey  string `yaml:"clientKey"`
}

// IsEnabled reports whether TLS is enabled. A nil TLSConfig or an unset Enabled
// flag defaults to false.
func (c *TLSConfig) IsEnabled() bool {
	if c == nil || c.Enabled == nil {
		return false
	}
	return *c.Enabled
}

// LoadConfig reads and validates the user configuration YAML at path.
func LoadConfig(path string) (*Config, error) {
	content, err := os.ReadFile(path)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to read user configuration file %q", path)
	}

	var config Config
	if err := yaml.Unmarshal(content, &config); err != nil {
		return nil, errors.Wrapf(err, "failed to unmarshal user configuration file %q", path)
	}

	if err := config.validate(); err != nil {
		return nil, fmt.Errorf("invalid user configuration %q: %w", path, err)
	}
	return &config, nil
}

// validate reports the first missing required field.
func (c *Config) validate() error {
	if c.MSP.LocalMspID == "" {
		return errors.New("msp.localMspID is required")
	}
	if c.MSP.LocalMspDir == "" {
		return errors.New("msp.localMspDir is required")
	}
	if c.TLS.IsEnabled() && (c.TLS.ClientCert == "") != (c.TLS.ClientKey == "") {
		return errors.New("tls.clientCert and tls.clientKey must be set together")
	}
	return nil
}
