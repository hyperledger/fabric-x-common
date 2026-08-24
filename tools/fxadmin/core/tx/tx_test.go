/*
Copyright IBM Corp. All Rights Reserved.

SPDX-License-Identifier: Apache-2.0
*/

package tx_test

import (
	"bytes"
	"errors"
	"io"
	"net"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	cb "github.com/hyperledger/fabric-protos-go-apiv2/common"
	ab "github.com/hyperledger/fabric-protos-go-apiv2/orderer"
	"github.com/stretchr/testify/require"
	"go.yaml.in/yaml/v3"
	"google.golang.org/grpc"
	"google.golang.org/protobuf/proto"

	"github.com/hyperledger/fabric-x-common/api/ordererpb"
	"github.com/hyperledger/fabric-x-common/api/types"
	"github.com/hyperledger/fabric-x-common/common/util"
	"github.com/hyperledger/fabric-x-common/tools/configtxgen"
	"github.com/hyperledger/fabric-x-common/tools/cryptogen"
	"github.com/hyperledger/fabric-x-common/tools/fxadmin/core/signer"
	"github.com/hyperledger/fabric-x-common/tools/fxadmin/core/tx"
	"github.com/hyperledger/fabric-x-common/tools/fxadmin/core/user"
	"github.com/hyperledger/fabric-x-common/utils/testcrypto"
)

const (
	notImplemented   = "not implemented"
	adminYAML        = "admin.yaml"
	missingInputCase = "missing input file"

	missingConfigCase   = "missing config file"
	notEnvelopeCase     = "input is not an envelope"
	readUserConfigError = "failed to read user configuration"
)

// TestHandlerNotImplemented asserts every not-yet-implemented tx subcommand is
// a skeleton that reports "not implemented" without panicking. Replace each
// subtest with a behavioral test as the command is implemented.
func TestHandlerNotImplemented(t *testing.T) {
	t.Parallel()
	h := tx.New()

	t.Run("send", func(t *testing.T) {
		t.Parallel()
		require.ErrorContains(t, h.Send("endorsed.pb", adminYAML, "current.pb"), notImplemented)
	})
}

// TestSubmitErrors asserts that `tx submit` reports readable errors, before any
// network access, for a missing transaction file, a transaction that is not a
// well-formed envelope, a missing configuration file, and an unreadable config
// block.
func TestSubmitErrors(t *testing.T) {
	t.Parallel()

	configPath, _, _ := newAdminConfig(t)
	validTx := writeFile(t, marshalConfigTx(t, "test-channel"))
	invalidEnvelope := writeFile(t, []byte("not an envelope"))
	invalidBlock := writeFile(t, []byte("not a block"))

	for _, tc := range []struct {
		name    string
		input   string
		config  string
		block   string
		wantErr string
	}{
		{
			name:    missingInputCase,
			input:   filepath.Join(t.TempDir(), "absent.pb"),
			config:  configPath,
			block:   invalidBlock,
			wantErr: "failed to read configuration transaction",
		},
		{
			name:    notEnvelopeCase,
			input:   invalidEnvelope,
			config:  configPath,
			block:   invalidBlock,
			wantErr: "failed to unmarshal configuration transaction",
		},
		{
			name:    missingConfigCase,
			input:   validTx,
			config:  filepath.Join(t.TempDir(), "absent.yaml"),
			block:   invalidBlock,
			wantErr: readUserConfigError,
		},
		{
			name:    "unreadable config block",
			input:   validTx,
			config:  configPath,
			block:   filepath.Join(t.TempDir(), "absent.pb"),
			wantErr: "failed to read config block",
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			require.ErrorContains(t, tx.New().Submit(tc.input, tc.config, tc.block), tc.wantErr)
		})
	}
}

// TestSubmit asserts that `tx submit` reads the prepared configuration
// transaction, connects to the network described by the config block, and
// broadcasts the transaction to every router without error, including when some
// routers reject it or are unreachable (the command reports per-router outcomes
// but only fails when no routers are configured).
func TestSubmit(t *testing.T) {
	t.Parallel()

	// Three routers: one acknowledges, one rejects, one is unreachable. Submit
	// still returns nil, because it broadcasts to every router and reports the
	// per-router outcomes rather than failing on individual rejections.
	acking := startBroadcastServer(t, cb.Status_SUCCESS)
	rejecting := startBroadcastServer(t, cb.Status_BAD_REQUEST)
	unreachable := freeAddress(t)

	blockPath, configPath := newSubmitFixture(t, "test-channel", []string{acking, rejecting, unreachable})
	txPath := writeFile(t, marshalConfigTx(t, "test-channel"))

	require.NoError(t, tx.New().Submit(txPath, configPath, blockPath))
}

// TestSubmitNoRouters asserts that `tx submit` fails when the config block
// carries no router endpoints to broadcast to. The failure surfaces while
// loading the client from the block, before any broadcast is attempted.
func TestSubmitNoRouters(t *testing.T) {
	t.Parallel()

	blockPath, configPath := newSubmitFixture(t, "test-channel", nil)
	txPath := writeFile(t, marshalConfigTx(t, "test-channel"))

	err := tx.New().Submit(txPath, configPath, blockPath)
	require.ErrorContains(t, err, "no router endpoints")
}

// TestEndorse asserts that `tx endorse` wraps the input ConfigUpdate in a
// ConfigUpdateEnvelope carrying exactly one admin ConfigSignature, that the
// embedded ConfigUpdate bytes are unchanged (so every endorser signs the same
// bytes), and that the signature verifies against the admin identity over
// SignatureHeader||ConfigUpdate.
func TestEndorse(t *testing.T) {
	t.Parallel()

	adminConfigPath, mspDir, mspID := newAdminConfig(t)
	rawConfigUpdate := marshalConfigUpdate(t, "test-channel")
	rawConfigUpdatePath := writeFile(t, rawConfigUpdate)
	outputPath := filepath.Join(t.TempDir(), "endorsement.pb")

	require.NoError(t, tx.New().Endorse(rawConfigUpdatePath, adminConfigPath, outputPath))

	out, err := os.ReadFile(outputPath)
	require.NoError(t, err)
	env := &cb.ConfigUpdateEnvelope{}
	require.NoError(t, proto.Unmarshal(out, env))

	require.Equal(t, rawConfigUpdate, env.GetConfigUpdate())
	require.Len(t, env.GetSignatures(), 1)
	sig := env.GetSignatures()[0]
	require.NotEmpty(t, sig.GetSignatureHeader())
	require.NotEmpty(t, sig.GetSignature())

	admin, err := signer.New(mspID, mspDir)
	require.NoError(t, err)
	signed := util.ConcatenateBytes(sig.GetSignatureHeader(), env.GetConfigUpdate())
	require.NoError(t, admin.Verify(signed, sig.GetSignature()))

	sh := &cb.SignatureHeader{}
	require.NoError(t, proto.Unmarshal(sig.GetSignatureHeader(), sh))
	creator, err := admin.Serialize()
	require.NoError(t, err)
	require.Equal(t, creator, sh.GetCreator())
	require.NotEmpty(t, sh.GetNonce())
}

// TestEndorseErrors asserts that `tx endorse` reports readable errors for a
// missing input file, a missing admin configuration, and an input that is not
// a valid ConfigUpdate.
func TestEndorseErrors(t *testing.T) {
	t.Parallel()

	adminConfigPath, _, _ := newAdminConfig(t)
	validConfigUpdate := writeFile(t, marshalConfigUpdate(t, "test-channel"))
	invalidConfigUpdate := writeFile(t, []byte("not a valid config update"))

	for _, tc := range []struct {
		name    string
		input   string
		config  string
		wantErr string
	}{
		{
			name:    missingInputCase,
			input:   filepath.Join(t.TempDir(), "absent.pb"),
			config:  adminConfigPath,
			wantErr: "failed to read config update",
		},
		{
			name:    missingConfigCase,
			input:   validConfigUpdate,
			config:  filepath.Join(t.TempDir(), "absent.yaml"),
			wantErr: readUserConfigError,
		},
		{
			name:    "input is not a config update",
			input:   invalidConfigUpdate,
			config:  adminConfigPath,
			wantErr: "failed to unmarshal config update",
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			outputPath := filepath.Join(t.TempDir(), "endorsement.pb")
			require.ErrorContains(t, tx.New().Endorse(tc.input, tc.config, outputPath), tc.wantErr)
		})
	}
}

// TestMerge asserts that `tx merge` combines the signatures of several
// endorsements over the same ConfigUpdate into a single envelope that carries
// the shared ConfigUpdate bytes and every distinct signer's ConfigSignature,
// each still verifying against its own admin identity.
func TestMerge(t *testing.T) {
	t.Parallel()

	admins := newAdminConfigs(t, 2)
	rawConfigUpdate := marshalConfigUpdate(t, "test-channel")

	endorsement1 := endorse(t, admins[0], rawConfigUpdate)
	endorsement2 := endorse(t, admins[1], rawConfigUpdate)
	outputPath := filepath.Join(t.TempDir(), "merged.pb")

	require.NoError(t, tx.New().Merge([]string{endorsement1, endorsement2}, outputPath))

	merged := readConfigUpdateEnvelope(t, outputPath)
	require.Equal(t, rawConfigUpdate, merged.GetConfigUpdate())
	require.Len(t, merged.GetSignatures(), 2)
	requireSignatureFrom(t, merged, admins[0])
	requireSignatureFrom(t, merged, admins[1])
}

// TestMergeDeduplicatesSigners asserts that merging endorsements from the same
// signer keeps a single ConfigSignature for that signer.
func TestMergeDeduplicatesSigners(t *testing.T) {
	t.Parallel()

	admins := newAdminConfigs(t, 1)
	rawConfigUpdate := marshalConfigUpdate(t, "test-channel")

	endorsement1 := endorse(t, admins[0], rawConfigUpdate)
	endorsement2 := endorse(t, admins[0], rawConfigUpdate)
	outputPath := filepath.Join(t.TempDir(), "merged.pb")

	require.NoError(t, tx.New().Merge([]string{endorsement1, endorsement2}, outputPath))

	merged := readConfigUpdateEnvelope(t, outputPath)
	require.Equal(t, rawConfigUpdate, merged.GetConfigUpdate())
	require.Len(t, merged.GetSignatures(), 1)
	requireSignatureFrom(t, merged, admins[0])
}

// TestMergeErrors asserts that `tx merge` reports readable errors for an
// unreadable input, an input that is not a ConfigUpdateEnvelope, and
// endorsements whose ConfigUpdate bytes disagree.
func TestMergeErrors(t *testing.T) {
	t.Parallel()

	admins := newAdminConfigs(t, 2)
	validEndorsement := endorse(t, admins[0], marshalConfigUpdate(t, "test-channel"))
	otherEndorsement := endorse(t, admins[1], marshalConfigUpdate(t, "other-channel"))
	// In a case where the first input's ConfigUpdate is empty, it must still be the
	// reference a later non-empty input is checked against.
	endorsementOfEmptyUpdate := endorse(t, admins[0], marshalConfigUpdate(t, ""))
	notConfigUpdateEnvelope := writeFile(t, []byte("not a config update envelope"))

	for _, tc := range []struct {
		name    string
		inputs  []string
		wantErr string
	}{
		{
			name:    missingInputCase,
			inputs:  []string{validEndorsement, filepath.Join(t.TempDir(), "absent.pb")},
			wantErr: "failed to read endorsement",
		},
		{
			name:    notEnvelopeCase,
			inputs:  []string{validEndorsement, notConfigUpdateEnvelope},
			wantErr: "failed to unmarshal endorsement",
		},
		{
			name:    "config updates disagree",
			inputs:  []string{validEndorsement, otherEndorsement},
			wantErr: "config update mismatch",
		},
		{
			name:    "empty first config update still anchors the comparison",
			inputs:  []string{endorsementOfEmptyUpdate, validEndorsement},
			wantErr: "config update mismatch",
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			outputPath := filepath.Join(t.TempDir(), "merged.pb")
			require.ErrorContains(t, tx.New().Merge(tc.inputs, outputPath), tc.wantErr)
		})
	}
}

// TestPrepare asserts that `tx prepare` wraps an endorsed ConfigUpdateEnvelope
// in a common.Envelope whose channel header is of type CONFIG_UPDATE and whose
// channel ID matches the one in the endorsed update, that the wrapped envelope
// wraps the original endorsed update (endorsement signatures
// preserved), and that the payload is signed by the submitting client identity.
func TestPrepare(t *testing.T) {
	t.Parallel()

	client := newAdminConfigs(t, 1)[0]
	rawConfigUpdate := marshalConfigUpdate(t, "test-channel")
	endorsedPath := endorse(t, client, rawConfigUpdate)
	endorsed := readConfigUpdateEnvelope(t, endorsedPath)
	outputPath := filepath.Join(t.TempDir(), "config_tx.pb")

	require.NoError(t, tx.New().Prepare(endorsedPath, client.configPath, outputPath))

	out, err := os.ReadFile(outputPath)
	require.NoError(t, err)
	env := &cb.Envelope{}
	require.NoError(t, proto.Unmarshal(out, env))

	payload := &cb.Payload{}
	require.NoError(t, proto.Unmarshal(env.GetPayload(), payload))

	channelHeader := &cb.ChannelHeader{}
	require.NoError(t, proto.Unmarshal(payload.GetHeader().GetChannelHeader(), channelHeader))
	require.Equal(t, int32(cb.HeaderType_CONFIG_UPDATE), channelHeader.GetType())
	require.Equal(t, "test-channel", channelHeader.GetChannelId())

	wrapped := &cb.ConfigUpdateEnvelope{}
	require.NoError(t, proto.Unmarshal(payload.GetData(), wrapped))
	require.Equal(t, rawConfigUpdate, wrapped.GetConfigUpdate())
	require.Len(t, wrapped.GetSignatures(), 1)
	requireSignatureFrom(t, wrapped, client)

	identity, err := signer.New(client.mspID, client.mspDir)
	require.NoError(t, err)
	require.NoError(t, identity.Verify(env.GetPayload(), env.GetSignature()))

	signatureHeader := &cb.SignatureHeader{}
	require.NoError(t, proto.Unmarshal(payload.GetHeader().GetSignatureHeader(), signatureHeader))
	creator, err := identity.Serialize()
	require.NoError(t, err)
	require.Equal(t, creator, signatureHeader.GetCreator())

	require.Equal(t, endorsed.GetConfigUpdate(), wrapped.GetConfigUpdate())
}

// TestPrepareMergedEndorsement asserts that `tx prepare` preserves every
// signature of a multi-org merged endorsement while wrapping it, and that the
// outer envelope is signed by the single submitting client (one of the admins).
func TestPrepareMergedEndorsement(t *testing.T) {
	t.Parallel()

	admins := newAdminConfigs(t, 2)
	rawConfigUpdate := marshalConfigUpdate(t, "test-channel")
	mergedPath := filepath.Join(t.TempDir(), "merged.pb")
	require.NoError(t, tx.New().Merge(
		[]string{endorse(t, admins[0], rawConfigUpdate), endorse(t, admins[1], rawConfigUpdate)},
		mergedPath,
	))

	client := admins[0] // the submitting client is one of the endorsing admins.
	outputPath := filepath.Join(t.TempDir(), "config_tx.pb")
	require.NoError(t, tx.New().Prepare(mergedPath, client.configPath, outputPath))

	out, err := os.ReadFile(outputPath)
	require.NoError(t, err)
	env := &cb.Envelope{}
	require.NoError(t, proto.Unmarshal(out, env))
	payload := &cb.Payload{}
	require.NoError(t, proto.Unmarshal(env.GetPayload(), payload))

	wrapped := &cb.ConfigUpdateEnvelope{}
	require.NoError(t, proto.Unmarshal(payload.GetData(), wrapped))
	require.Equal(t, rawConfigUpdate, wrapped.GetConfigUpdate())
	require.Len(t, wrapped.GetSignatures(), 2)
	requireSignatureFrom(t, wrapped, admins[0])
	requireSignatureFrom(t, wrapped, admins[1])

	identity, err := signer.New(client.mspID, client.mspDir)
	require.NoError(t, err)
	require.NoError(t, identity.Verify(env.GetPayload(), env.GetSignature()))
}

// TestPrepareErrors asserts that `tx prepare` reports readable errors for a
// missing input file, a missing client configuration, and an input that is not
// a ConfigUpdateEnvelope.
func TestPrepareErrors(t *testing.T) {
	t.Parallel()

	client := newAdminConfigs(t, 1)[0]
	validEndorsement := endorse(t, client, marshalConfigUpdate(t, "test-channel"))
	notConfigUpdateEnvelope := writeFile(t, []byte("not a config update envelope"))

	for _, tc := range []struct {
		name    string
		input   string
		config  string
		wantErr string
	}{
		{
			name:    missingInputCase,
			input:   filepath.Join(t.TempDir(), "absent.pb"),
			config:  client.configPath,
			wantErr: "failed to read endorsement",
		},
		{
			name:    missingConfigCase,
			input:   validEndorsement,
			config:  filepath.Join(t.TempDir(), "absent.yaml"),
			wantErr: readUserConfigError,
		},
		{
			name:    notEnvelopeCase,
			input:   notConfigUpdateEnvelope,
			config:  client.configPath,
			wantErr: "failed to unmarshal endorsement",
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			outputPath := filepath.Join(t.TempDir(), "config_tx.pb")
			require.ErrorContains(t, tx.New().Prepare(tc.input, tc.config, outputPath), tc.wantErr)
		})
	}
}

// adminConfig is a generated admin identity: the path to its admin
// configuration YAML plus the MSP directory and ID for independent verification.
type adminConfig struct {
	configPath, mspDir, mspID string
}

// newAdminConfig generates a single orderer org and returns its admin user's
// configuration YAML, MSP directory, and MSP ID.
func newAdminConfig(t *testing.T) (configPath, mspDir, mspID string) {
	t.Helper()
	admin := newAdminConfigs(t, 1)[0]
	return admin.configPath, admin.mspDir, admin.mspID
}

// newAdminConfigs generates count orderer organizations from one crypto tree,
// each with a broadcast and a deliver endpoint, and writes an admin
// configuration YAML for each org's admin user (users/Admin@<org>/msp).
func newAdminConfigs(t *testing.T, count int) []adminConfig {
	t.Helper()
	targetPath := t.TempDir()
	endpoints := make([]*types.OrdererEndpoint, 0, 2*count)
	for i := range count {
		partyID := uint32(i + 1)
		endpoints = append(endpoints,
			&types.OrdererEndpoint{ID: partyID, Host: "localhost", Port: 7050 + 2*i, API: []string{types.Broadcast}},
			&types.OrdererEndpoint{ID: partyID, Host: "localhost", Port: 7051 + 2*i, API: []string{types.Deliver}},
		)
	}
	_, err := testcrypto.CreateOrExtendConfigBlockWithCrypto(targetPath, &testcrypto.ConfigBlock{
		ChannelID:        "test-channel",
		OrdererEndpoints: endpoints,
	})
	require.NoError(t, err)

	orgsPath := filepath.Join(targetPath, cryptogen.OrdererOrganizationsDir)
	orgDirs, err := os.ReadDir(orgsPath)
	require.NoError(t, err)
	require.Len(t, orgDirs, count)

	admins := make([]adminConfig, count)
	for i, org := range orgDirs {
		orgName := org.Name()
		mspID := strings.TrimSuffix(orgName, ".com")
		mspDir := filepath.Join(orgsPath, orgName, "users", "Admin@"+orgName, "msp")

		configPath := filepath.Join(t.TempDir(), adminYAML)
		content, err := yaml.Marshal(user.Config{
			MSP: user.MSPConfig{LocalMspID: mspID, LocalMspDir: mspDir},
		})
		require.NoError(t, err)
		require.NoError(t, os.WriteFile(configPath, content, 0o600))
		admins[i] = adminConfig{configPath: configPath, mspDir: mspDir, mspID: mspID}
	}
	return admins
}

// endorse runs `tx endorse` for the given admin over rawUpdate and returns the
// path to the resulting endorsement file.
func endorse(t *testing.T, admin adminConfig, rawUpdate []byte) string {
	t.Helper()
	inputPath := writeFile(t, rawUpdate)
	outputPath := filepath.Join(t.TempDir(), "endorsement.pb")
	require.NoError(t, tx.New().Endorse(inputPath, admin.configPath, outputPath))
	return outputPath
}

// readConfigUpdateEnvelope reads and unmarshals a ConfigUpdateEnvelope from path.
func readConfigUpdateEnvelope(t *testing.T, path string) *cb.ConfigUpdateEnvelope {
	t.Helper()
	content, err := os.ReadFile(path)
	require.NoError(t, err)
	env := &cb.ConfigUpdateEnvelope{}
	require.NoError(t, proto.Unmarshal(content, env))
	return env
}

// requireSignatureFrom asserts that env carries exactly one signature from the
// given admin and that it verifies over SignatureHeader||ConfigUpdate.
func requireSignatureFrom(t *testing.T, env *cb.ConfigUpdateEnvelope, admin adminConfig) {
	t.Helper()
	identity, err := signer.New(admin.mspID, admin.mspDir)
	require.NoError(t, err)
	creator, err := identity.Serialize()
	require.NoError(t, err)

	var found int
	for _, sig := range env.GetSignatures() {
		sh := &cb.SignatureHeader{}
		require.NoError(t, proto.Unmarshal(sig.GetSignatureHeader(), sh))
		if !bytes.Equal(sh.GetCreator(), creator) {
			continue
		}
		found++
		signed := util.ConcatenateBytes(sig.GetSignatureHeader(), env.GetConfigUpdate())
		require.NoError(t, identity.Verify(signed, sig.GetSignature()))
	}
	require.Equal(t, 1, found)
}

// marshalConfigUpdate returns the marshaled bytes of a minimal ConfigUpdate for
// the given channel.
func marshalConfigUpdate(t *testing.T, channelID string) []byte {
	t.Helper()
	raw, err := proto.Marshal(&cb.ConfigUpdate{ChannelId: channelID})
	require.NoError(t, err)
	return raw
}

// marshalConfigTx returns the marshaled bytes of a minimal, well-formed
// common.Envelope standing in for a prepared configuration transaction. Its
// contents are not validated by `tx submit`, which only unmarshals the envelope
// before broadcasting it.
func marshalConfigTx(t *testing.T, channelID string) []byte {
	t.Helper()
	channelHeader, err := proto.Marshal(&cb.ChannelHeader{
		Type:      int32(cb.HeaderType_CONFIG_UPDATE),
		ChannelId: channelID,
	})
	require.NoError(t, err)
	payload, err := proto.Marshal(&cb.Payload{
		Header: &cb.Header{ChannelHeader: channelHeader},
	})
	require.NoError(t, err)
	raw, err := proto.Marshal(&cb.Envelope{Payload: payload})
	require.NoError(t, err)
	return raw
}

// writeFile writes content to a fresh temp file and returns its path.
func writeFile(t *testing.T, content []byte) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "config_update.pb")
	require.NoError(t, os.WriteFile(path, content, 0o600))
	return path
}

// newSubmitFixture builds a real ARMA config block whose parties' router
// endpoints are the given addresses (one party per address), writes it to disk,
// and writes an admin configuration YAML for a loadable consenter identity of
// the same crypto tree. It returns the paths to the block and the admin
// configuration, ready to be passed to `tx submit`.
func newSubmitFixture(t *testing.T, channelID string, routerEndpoints []string) (blockPath, configPath string) {
	t.Helper()

	shared := &ordererpb.SharedConfig{}
	for i, endpoint := range routerEndpoints {
		host, port := splitHostPort(t, endpoint)
		shared.PartiesConfig = append(shared.PartiesConfig, &ordererpb.PartyConfig{
			PartyID:      uint32(i + 1),
			RouterConfig: &ordererpb.RouterNodeConfig{Host: host, Port: port},
			// An assembler endpoint is required for the config block to be valid;
			// `tx submit` only dials the routers.
			AssemblerConfig: &ordererpb.AssemblerNodeConfig{Host: host, Port: port},
		})
	}
	meta, err := proto.Marshal(shared)
	require.NoError(t, err)

	targetPath := t.TempDir()
	block, err := cryptogen.CreateOrExtendConfigBlockWithCrypto(cryptogen.ConfigBlockParameters{
		TargetPath:  targetPath,
		BaseProfile: configtxgen.SampleFabricX,
		ChannelID:   channelID,
		Organizations: []cryptogen.OrganizationParameters{{
			Name:             "orderer-org-1",
			Domain:           "orderer-org-1.com",
			OrdererEndpoints: []*types.OrdererEndpoint{{ID: 1, Host: "localhost", Port: 7050}},
			ConsenterNodes:   []cryptogen.Node{{CommonName: "consenter", Hostname: "consenter"}},
			OrdererNodes:     []cryptogen.Node{{CommonName: "orderer-node", Hostname: "orderer-node"}},
		}},
		ArmaMetaBytes: meta,
	})
	require.NoError(t, err)

	blockPath = filepath.Join(targetPath, "current.pb")
	raw, err := proto.Marshal(block)
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(blockPath, raw, 0o600))

	mspDirs := testcrypto.GetConsenterMspDirs(targetPath)
	require.NotEmpty(t, mspDirs)
	configPath = filepath.Join(t.TempDir(), adminYAML)
	content, err := yaml.Marshal(user.Config{
		MSP: user.MSPConfig{LocalMspID: mspDirs[0].MspName, LocalMspDir: mspDirs[0].MspDir},
	})
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(configPath, content, 0o600))
	return blockPath, configPath
}

// splitHostPort splits a "host:port" address into its host and numeric port.
func splitHostPort(t *testing.T, address string) (host string, port uint32) {
	t.Helper()
	host, portStr, err := net.SplitHostPort(address)
	require.NoError(t, err)
	p, err := strconv.ParseUint(portStr, 10, 32)
	require.NoError(t, err)
	return host, uint32(p)
}

// broadcastStub is an in-process AtomicBroadcast server that replies to every
// received envelope with a fixed status.
type broadcastStub struct {
	ab.UnimplementedAtomicBroadcastServer
	status cb.Status
}

// Broadcast replies with the stub's fixed status for each envelope received,
// returning when the client closes the stream.
func (s *broadcastStub) Broadcast(stream ab.AtomicBroadcast_BroadcastServer) error {
	for {
		_, err := stream.Recv()
		if errors.Is(err, io.EOF) {
			return nil // client closed the stream
		}
		if err != nil {
			return err
		}
		if err := stream.Send(&ab.BroadcastResponse{Status: s.status}); err != nil {
			return err
		}
	}
}

// startBroadcastServer starts an in-process AtomicBroadcast server that answers
// every broadcast with status, and returns its "host:port" address. The server
// is stopped when the test ends.
func startBroadcastServer(t *testing.T, status cb.Status) string {
	t.Helper()
	lis, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)

	srv := grpc.NewServer()
	ab.RegisterAtomicBroadcastServer(srv, &broadcastStub{status: status})
	go func() { _ = srv.Serve(lis) }()
	t.Cleanup(srv.Stop)

	return lis.Addr().String()
}

// freeAddress returns a "host:port" that no server is listening on, so a dial
// to it fails. The listener is closed before returning.
func freeAddress(t *testing.T) string {
	t.Helper()
	lis, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	addr := lis.Addr().String()
	require.NoError(t, lis.Close())
	return addr
}
