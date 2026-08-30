/*
Copyright IBM Corp. All Rights Reserved.

SPDX-License-Identifier: Apache-2.0
*/

// Package tx implements the `fxadmin tx` command family: endorsing, merging,
// preparing, submitting, and sending configuration update transactions.
package tx

import (
	"bytes"
	"os"

	"github.com/cockroachdb/errors"
	"github.com/hyperledger/fabric-lib-go/bccsp"
	"github.com/hyperledger/fabric-lib-go/bccsp/factory"
	"github.com/hyperledger/fabric-lib-go/common/flogging"
	cb "github.com/hyperledger/fabric-protos-go-apiv2/common"
	"google.golang.org/protobuf/proto"

	"github.com/hyperledger/fabric-x-common/common/util"
	"github.com/hyperledger/fabric-x-common/msp"
	"github.com/hyperledger/fabric-x-common/protoutil"
	"github.com/hyperledger/fabric-x-common/tools/fxadmin/core/client"
	"github.com/hyperledger/fabric-x-common/tools/fxadmin/core/signer"
	"github.com/hyperledger/fabric-x-common/tools/fxadmin/core/user"
)

var logger = flogging.MustGetLogger("fxadmin.tx")

// Handler executes the tx subcommands. It carries the BCCSP used to build the
// channel config bundle when reading router endpoints from the config block.
type Handler struct {
	csp bccsp.BCCSP
}

// New returns a tx command handler.
func New() *Handler {
	return &Handler{csp: factory.GetDefault()}
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

// Submit implements `fxadmin tx submit`. It reads the prepared configuration
// transaction (a common.Envelope) at inputPath and broadcasts it to every
// router of the network described by the config block at currentBlockPath,
// using the client identity in the configuration YAML at configPath to sign the
// connection. It logs each router's outcome and a summary.
//
// Submit succeeds only when at least 2f+1 of the routers acknowledged the transaction,
// where f is the number of faulty parties the network tolerates, derived from
// the number of parties in the config block. Otherwise, it returns an error,
// so a partial or failed delivery is reported to the caller.
func (h *Handler) Submit(inputPath, configPath, currentBlockPath string) error {
	logger.Debugf("tx submit: input=%s config=%s current-block=%s", inputPath, configPath, currentBlockPath)

	envelope, err := readEnvelope(inputPath)
	if err != nil {
		return err
	}

	cl, err := client.LoadFromFiles(configPath, currentBlockPath, h.csp)
	if err != nil {
		return err
	}

	statuses, err := cl.BroadcastToAllRouters(envelope)
	if err != nil {
		return errors.Wrap(err, "failed to broadcast configuration transaction")
	}

	return reportBroadcast(statuses, 2*cl.FaultThreshold()+1)
}

// readEnvelope reads and unmarshals a prepared configuration transaction
// (common.Envelope) from path.
func readEnvelope(path string) (*cb.Envelope, error) {
	content, err := os.ReadFile(path)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to read configuration transaction %q", path)
	}
	envelope, err := protoutil.UnmarshalEnvelope(content)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to unmarshal configuration transaction %q", path)
	}
	return envelope, nil
}

// reportBroadcast logs each router's outcome, then a summary of how
// many routers acknowledged the transaction against the required quorum. It
// returns an error when fewer than quorum routers acknowledged, so submission
// fails on a partial or failed delivery.
func reportBroadcast(statuses []client.RouterStatus, quorum int) error {
	for _, status := range statuses {
		if status.Err != nil {
			logger.Warnf("router %s did not acknowledge the configuration transaction: %v",
				status.Endpoint, status.Err)
		} else {
			logger.Infof("router %s acknowledged the configuration transaction", status.Endpoint)
		}
	}

	acked := countAcks(statuses)
	if acked < quorum {
		return errors.Newf("configuration transaction acknowledged by %d of %d routers, below the quorum of %d",
			acked, len(statuses), quorum)
	}
	logger.Infof("configuration transaction acknowledged by %d of %d routers, quorum of %d reached",
		acked, len(statuses), quorum)
	return nil
}

// countAcks returns the number of routers that acknowledged the transaction,
// those whose status carries no error.
func countAcks(statuses []client.RouterStatus) int {
	acked := 0
	for _, status := range statuses {
		if status.Err == nil {
			acked++
		}
	}
	return acked
}

// Send implements `fxadmin tx send`, equivalent to `tx prepare` followed by
// `tx submit` in one step: it prepares the configuration transaction from the
// endorsed common.ConfigUpdateEnvelope at inputPath, writing it to outputPath,
// then submits that transaction to the routers of the network described by the
// config block at currentBlockPath, using the client identity in the
// configuration YAML at configPath for both steps.
// The transaction is written by Prepare before Submit broadcasts it, so the
// record is kept even if the broadcast fails.
func (h *Handler) Send(inputPath, configPath, currentBlockPath, outputPath string) error {
	logger.Debugf("tx send: input=%s config=%s current-block=%s output=%s",
		inputPath, configPath, currentBlockPath, outputPath)

	if err := h.Prepare(inputPath, configPath, outputPath); err != nil {
		return err
	}
	return h.Submit(outputPath, configPath, currentBlockPath)
}
