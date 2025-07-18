/*
Copyright IBM Corp. All Rights Reserved.

SPDX-License-Identifier: Apache-2.0
*/

package common

import (
	"os"

	"github.com/pkg/errors"
	"gopkg.in/yaml.v2"

	"github.com/hyperledger/fabric-x-common/cmd/common/comm"
	"github.com/hyperledger/fabric-x-common/cmd/common/signer"
)

// Config aggregates configuration of TLS and signing
type Config struct {
	Version      int
	TLSConfig    comm.Config
	SignerConfig signer.Config
}

// ConfigFromFile loads the given file and converts it to a Config
func ConfigFromFile(file string) (Config, error) {
	configData, err := os.ReadFile(file)
	if err != nil {
		return Config{}, errors.WithStack(err)
	}
	config := Config{}

	if err := yaml.Unmarshal(configData, &config); err != nil {
		return Config{}, errors.Errorf("error unmarshalling YAML file %s: %s", file, err)
	}

	return config, validateConfig(config)
}

// ToFile writes the config into a file
func (c Config) ToFile(file string) error {
	if err := validateConfig(c); err != nil {
		return errors.Wrap(err, "config isn't valid")
	}
	b, err := yaml.Marshal(c)
	if err != nil {
		return errors.Wrap(err, "failed to marshal config")
	}
	if err := os.WriteFile(file, b, 0o600); err != nil {
		return errors.Errorf("failed writing file %s: %v", file, err)
	}
	return nil
}

func validateConfig(conf Config) error {
	nonEmptyElems := map[string]string{
		"MSPID":        conf.SignerConfig.MSPID,
		"IdentityPath": conf.SignerConfig.IdentityPath,
		"KeyPath":      conf.SignerConfig.KeyPath,
	}

	for key, value := range nonEmptyElems {
		if value == "" {
			return errors.Errorf("%s is mandatory and cannot be empty", key)
		}
	}

	return nil
}
