/*
Copyright IBM Corp. All Rights Reserved.

SPDX-License-Identifier: Apache-2.0
*/

// Package tx implements the `fxadmin tx` command family: endorsing, merging,
// preparing, submitting, and sending configuration update transactions.
package tx

import (
	"bytes"
	"fmt"
	"os"

	"github.com/cockroachdb/errors"
	"github.com/hyperledger/fabric-lib-go/common/flogging"
	cb "github.com/hyperledger/fabric-protos-go-apiv2/common"
	"google.golang.org/protobuf/proto"

	"github.com/hyperledger/fabric-x-common/common/util"
	"github.com/hyperledger/fabric-x-common/msp"
	"github.com/hyperledger/fabric-x-common/protoutil"
	"github.com/hyperledger/fabric-x-common/tools/fxadmin/core/signer"
	"github.com/hyperledger/fabric-x-common/tools/fxadmin/core/user"
)

var logger = flogging.MustGetLogger("fxadmin.tx")

var errNotImplemented = errors.New("not implemented")

// Handler executes the tx subcommands. Its dependencies (the signer, the
// router Broadcast client) will be added as constructor arguments and struct
// fields when the commands are implemented.
type Handler struct{}

// New returns a tx command handler.
func New() *Handler {
	return &Handler{}
}

// Endorse implements `fxadmin tx endorse`. It reads the marshaled
// common.ConfigUpdate at inputPath, signs it with the admin identity described
// by the configuration YAML at configPath, and writes a marshaled
// common.ConfigUpdateEnvelope carrying that admin's ConfigSignature to
// outputPath.
func (*Handler) Endorse(inputPath, configPath, outputPath string) error {
	logger.Debugf("tx endorse: input=%s config=%s output=%s", inputPath, configPath, outputPath)

	configUpdate, err := os.ReadFile(inputPath)
	if err != nil {
		return errors.Wrapf(err, "failed to read config update %q", inputPath)
	}
	if err = proto.Unmarshal(configUpdate, &cb.ConfigUpdate{}); err != nil {
		return errors.Wrapf(err, "failed to unmarshal config update %q", inputPath)
	}

	config, err := user.LoadConfig(configPath)
	if err != nil {
		return errors.Wrapf(err, "failed to load user configuration %q", configPath)
	}
	admin, err := signer.New(config.MSP.LocalMspID, config.MSP.LocalMspDir)
	if err != nil {
		return errors.Wrapf(err, "failed to load admin signing identity for MSP %q", config.MSP.LocalMspID)
	}

	signature, err := signConfigUpdate(admin, configUpdate)
	if err != nil {
		return errors.Wrapf(err, "failed to endorse config update %q", inputPath)
	}
	env := &cb.ConfigUpdateEnvelope{
		ConfigUpdate: configUpdate,
		Signatures:   []*cb.ConfigSignature{signature},
	}

	out, err := proto.Marshal(env)
	if err != nil {
		return errors.Wrap(err, "failed to marshal endorsed config update")
	}
	if err := os.WriteFile(outputPath, out, 0o600); err != nil {
		return errors.Wrapf(err, "failed to write output %q", outputPath)
	}

	logger.Infof("endorsed config update %s with %s to %s", inputPath, config.MSP.LocalMspID, outputPath)
	return nil
}

// signConfigUpdate produces the admin's ConfigSignature over the config update.
// The signature covers SignatureHeader||ConfigUpdate.
func signConfigUpdate(admin msp.SigningIdentity, configUpdate []byte) (*cb.ConfigSignature, error) {
	sigHeader, err := protoutil.NewSignatureHeader(admin)
	if err != nil {
		return nil, errors.Wrap(err, "failed to create signature header")
	}
	signatureHeader, err := proto.Marshal(sigHeader)
	if err != nil {
		return nil, errors.Wrap(err, "failed to marshal signature header")
	}
	signature, err := admin.Sign(util.ConcatenateBytes(signatureHeader, configUpdate))
	if err != nil {
		return nil, errors.Wrap(err, "failed to sign config update")
	}
	return &cb.ConfigSignature{
		SignatureHeader: signatureHeader,
		Signature:       signature,
	}, nil
}

// Merge implements `fxadmin tx merge`. It reads several marshaled
// common.ConfigUpdateEnvelope files, each endorsing the same ConfigUpdate, and
// writes a single envelope carrying the shared ConfigUpdate bytes and the union
// of their signatures. All inputs must endorse identical ConfigUpdate bytes;
// duplicate signers are de-duplicated, keeping the first occurrence.
// Signatures are not verified here; their validity is checked against
// channel policy at submission.
func (*Handler) Merge(inputPaths []string, outputPath string) error {
	logger.Debugf("tx merge: inputs=%v output=%s", inputPaths, outputPath)

	merged := &cb.ConfigUpdateEnvelope{}
	seen := make(map[string]struct{})
	for i, path := range inputPaths {
		env, err := readConfigUpdateEnvelope(path)
		if err != nil {
			return err
		}
		if i == 0 {
			merged.ConfigUpdate = env.GetConfigUpdate()
		} else if !bytes.Equal(merged.GetConfigUpdate(), env.GetConfigUpdate()) {
			return errors.Newf("config update mismatch: %q endorses a different config update", path)
		}
		if err := appendSignatures(merged, env, seen); err != nil {
			return errors.Wrapf(err, "invalid endorsement %q", path)
		}
	}

	out, err := proto.Marshal(merged)
	if err != nil {
		return errors.Wrap(err, "failed to marshal merged config update")
	}
	if err := os.WriteFile(outputPath, out, 0o600); err != nil {
		return errors.Wrapf(err, "failed to write output %q", outputPath)
	}

	logger.Infof("merged %d endorsements into %s (%d signatures)",
		len(inputPaths), outputPath, len(merged.GetSignatures()))
	return nil
}

// readConfigUpdateEnvelope reads and unmarshals a common.ConfigUpdateEnvelope from path.
func readConfigUpdateEnvelope(path string) (*cb.ConfigUpdateEnvelope, error) {
	content, err := os.ReadFile(path)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to read endorsement %q", path)
	}
	env := &cb.ConfigUpdateEnvelope{}
	if err := proto.Unmarshal(content, env); err != nil {
		return nil, errors.Wrapf(err, "failed to unmarshal endorsement %q", path)
	}
	return env, nil
}

// appendSignatures adds env's signatures to merged, skipping signers already in
// seen. A signer is identified by its SignatureHeader.Creator bytes.
func appendSignatures(merged, env *cb.ConfigUpdateEnvelope, seen map[string]struct{}) error {
	for _, sig := range env.GetSignatures() {
		sigHeader := &cb.SignatureHeader{}
		if err := proto.Unmarshal(sig.GetSignatureHeader(), sigHeader); err != nil {
			return errors.Wrap(err, "failed to unmarshal signature header")
		}
		key := string(sigHeader.GetCreator())
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		merged.Signatures = append(merged.Signatures, sig)
	}
	return nil
}

// Prepare implements `fxadmin tx prepare`. It reads the endorsed
// common.ConfigUpdateEnvelope at inputPath and wraps it in a common.Envelope
// whose channel header is of type CONFIG_UPDATE, signed by the submitting
// client identity described by the configuration YAML at configPath (a channel
// writer, which may be one of the admins). The marshaled envelope is written to
// outputPath, ready for submission to the routers.
func (*Handler) Prepare(inputPath, configPath, outputPath string) error {
	logger.Debugf("tx prepare: input=%s config=%s output=%s", inputPath, configPath, outputPath)

	env, err := readConfigUpdateEnvelope(inputPath)
	if err != nil {
		return err
	}
	configUpdate := &cb.ConfigUpdate{}
	if err = proto.Unmarshal(env.GetConfigUpdate(), configUpdate); err != nil {
		return errors.Wrapf(err, "failed to unmarshal config update in endorsement %q", inputPath)
	}

	config, err := user.LoadConfig(configPath)
	if err != nil {
		return errors.Wrapf(err, "failed to load user configuration %q", configPath)
	}
	client, err := signer.New(config.MSP.LocalMspID, config.MSP.LocalMspDir)
	if err != nil {
		return errors.Wrapf(err, "failed to load submitting client signing identity for MSP %q", config.MSP.LocalMspID)
	}

	tx, err := protoutil.CreateSignedEnvelope(
		cb.HeaderType_CONFIG_UPDATE, configUpdate.GetChannelId(), client, env, 0, 0)
	if err != nil {
		return errors.Wrap(err, "failed to create configuration transaction")
	}

	out, err := proto.Marshal(tx)
	if err != nil {
		return errors.Wrap(err, "failed to marshal configuration transaction")
	}
	if err := os.WriteFile(outputPath, out, 0o600); err != nil {
		return errors.Wrapf(err, "failed to write output %q", outputPath)
	}

	logger.Infof("prepared configuration transaction from %s signed by %s to %s",
		inputPath, config.MSP.LocalMspID, outputPath)
	return nil
}

// Submit implements `fxadmin tx submit`.
func (*Handler) Submit(inputPath, configPath, currentBlockPath string) error {
	logger.Debugf("tx submit: input=%s config=%s current-block=%s", inputPath, configPath, currentBlockPath)
	return fmt.Errorf("tx submit: %w", errNotImplemented)
}

// Send implements `fxadmin tx send`.
func (*Handler) Send(inputPath, configPath, currentBlockPath string) error {
	logger.Debugf("tx send: input=%s config=%s current-block=%s", inputPath, configPath, currentBlockPath)
	return fmt.Errorf("tx send: %w", errNotImplemented)
}
