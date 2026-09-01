/*
Copyright IBM Corp. All Rights Reserved.

SPDX-License-Identifier: Apache-2.0
*/

package follow_test

import (
	"io"
	"net"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"go.yaml.in/yaml/v3"
	"google.golang.org/protobuf/proto"

	"github.com/hyperledger/fabric-x-common/api/ordererpb"
	"github.com/hyperledger/fabric-x-common/api/types"
	"github.com/hyperledger/fabric-x-common/tools/configtxgen"
	"github.com/hyperledger/fabric-x-common/tools/cryptogen"
	"github.com/hyperledger/fabric-x-common/tools/fxadmin/core/client"
	clienttest "github.com/hyperledger/fabric-x-common/tools/fxadmin/core/client/test"
	"github.com/hyperledger/fabric-x-common/tools/fxadmin/core/follow"
	"github.com/hyperledger/fabric-x-common/tools/fxadmin/core/user"
)

// TestRunAllAssemblersCommitted asserts that when every assembler has committed
// the next config block, follow reports each as committed. The generated current
// block is at genesis sequence 0, so follow waits for sequence 1, which the
// in-process assembler stubs serve.
//
//nolint:paralleltest
func TestRunAllAssemblersCommitted(t *testing.T) {
	endpoints := []string{
		serveAssembler(t, 1), serveAssembler(t, 1), serveAssembler(t, 1), serveAssembler(t, 1),
	}
	configPath, blockPath := newFollowInputs(t, "test-channel", endpoints)
	outputPath := filepath.Join(t.TempDir(), "next_config.pb")

	out := captureStdout(t, func() {
		require.NoError(t, follow.New().Run(configPath, blockPath, outputPath, 30*time.Second))
	})

	require.Contains(t, out, "LAST BLOCK")
	require.Equal(t, 4, strings.Count(out, "committed"))
	committedRows := regexp.MustCompile(`\s10\s+1\s+committed`).FindAllString(out, -1)
	require.Len(t, committedRows, 4)
	require.NotContains(t, out, "behind")
	require.NotContains(t, out, "unreachable")

	// The agreed next config block is written and carries the expected sequence.
	require.Equal(t, uint64(1), sequenceOfWrittenBlock(t, outputPath))
}

// TestRunAgreesDespiteDifferentSignatureMetadata asserts that assemblers which
// committed the same next config block but carry different signature
// metadata - as real assemblers do, still agree, and the block is written.
// This guards the header+data (metadata-excluding) comparison.
//
//nolint:paralleltest
func TestRunAgreesDespiteDifferentSignatureMetadata(t *testing.T) {
	// Four assemblers all committed config sequence 1, but each serves the block
	// with a distinct SIGNATURES metadata payload.
	endpoints := []string{
		serveAssemblerWithSigs(t, "sigs-1"), serveAssemblerWithSigs(t, "sigs-2"),
		serveAssemblerWithSigs(t, "sigs-3"), serveAssemblerWithSigs(t, "sigs-4"),
	}
	configPath, blockPath := newFollowInputs(t, "test-channel", endpoints)
	outputPath := filepath.Join(t.TempDir(), "next_config.pb")

	require.NoError(t, follow.New().Run(configPath, blockPath, outputPath, 30*time.Second))
	require.Equal(t, uint64(1), sequenceOfWrittenBlock(t, outputPath))
}

// TestRunTimeoutSomeBehind asserts that when the timeout elapses before every
// assembler committed the next config block, follow reports the assemblers still
// at the old sequence as behind and the rest as committed.
//
//nolint:paralleltest
func TestRunTimeoutSomeBehind(t *testing.T) {
	// Three of four assemblers already serve the new config sequence 1; the fourth
	// is still at the current sequence 0, so it never reaches the expected sequence.
	endpoints := []string{
		serveAssembler(t, 1), serveAssembler(t, 1), serveAssembler(t, 1), serveAssembler(t, 0),
	}
	configPath, blockPath := newFollowInputs(t, "test-channel", endpoints)
	outputPath := filepath.Join(t.TempDir(), "next_config.pb")

	// With 4 parties the quorum is f+1 = 2, so the three committed assemblers still
	// agree on the next config block; it is written despite the fourth being behind.
	out := captureStdout(t, func() {
		require.NoError(t, follow.New().Run(configPath, blockPath, outputPath, 500*time.Millisecond))
	})

	require.Equal(t, 3, strings.Count(out, "committed"))
	require.Equal(t, 1, strings.Count(out, "behind"))
	require.Equal(t, uint64(1), sequenceOfWrittenBlock(t, outputPath))
}

// TestRunAssemblerUnreachable asserts that an assembler that never answers is
// reported as unreachable, alongside a reachable assembler that committed.
//
//nolint:paralleltest
func TestRunAssemblerUnreachable(t *testing.T) {
	// Two of four assemblers serve the new config sequence 1; the other two have no
	// server listening, so they can never be reached.
	endpoints := []string{
		serveAssembler(t, 1), serveAssembler(t, 1), clienttest.FreeAddress(t), clienttest.FreeAddress(t),
	}
	configPath, blockPath := newFollowInputs(t, "test-channel", endpoints)
	outputPath := filepath.Join(t.TempDir(), "next_config.pb")

	// With 4 parties the quorum is f+1 = 2, so the two reachable committed
	// assemblers are exactly enough to agree on and write the next config block.
	out := captureStdout(t, func() {
		require.NoError(t, follow.New().Run(configPath, blockPath, outputPath, 500*time.Millisecond))
	})

	require.Equal(t, 2, strings.Count(out, "committed"))
	require.Equal(t, 2, strings.Count(out, "unreachable"))
	require.Equal(t, uint64(1), sequenceOfWrittenBlock(t, outputPath))
}

// TestRunWritesBlockOnQuorum asserts that with 4 parties (quorum f+1 = 2), the
// next config block is written once at least two assemblers agree on it, even
// though a third is still behind.
//
//nolint:paralleltest
func TestRunWritesBlockOnQuorum(t *testing.T) {
	// Three of four assemblers committed sequence 1 (agreeing on the same block);
	// the fourth is unreachable. Two agreeing assemblers already meet the quorum.
	endpoints := []string{
		serveAssembler(t, 1), serveAssembler(t, 1), serveAssembler(t, 1), clienttest.FreeAddress(t),
	}
	configPath, blockPath := newFollowInputs(t, "test-channel", endpoints)
	outputPath := filepath.Join(t.TempDir(), "next_config.pb")

	require.NoError(t, follow.New().Run(configPath, blockPath, outputPath, 500*time.Millisecond))
	require.Equal(t, uint64(1), sequenceOfWrittenBlock(t, outputPath))
}

// TestRunNoQuorumNoBlock asserts that when fewer than a quorum of assemblers
// agree on the next config block, follow fails and writes no output. With 4
// parties the quorum is 2, so a single committed assembler is not enough.
//
//nolint:paralleltest
func TestRunNoQuorumNoBlock(t *testing.T) {
	// Only one assembler committed sequence 1; the other three are still behind,
	// so the quorum of 2 is never reached.
	endpoints := []string{
		serveAssembler(t, 1), serveAssembler(t, 0), serveAssembler(t, 0), serveAssembler(t, 0),
	}
	configPath, blockPath := newFollowInputs(t, "test-channel", endpoints)
	outputPath := filepath.Join(t.TempDir(), "next_config.pb")

	err := follow.New().Run(configPath, blockPath, outputPath, 500*time.Millisecond)
	require.ErrorContains(t, err, "no config block at last config sequence 1 was agreed by a quorum")
	require.NoFileExists(t, outputPath)
}

// serveAssembler binds a listener and serves an assembler ledger whose last
// config block is at configSequence, returning the "host:port" address.
// the stub reports its newest block as number 10 and records that
// its last config block is number 5.
func serveAssembler(t *testing.T, configSequence uint64) string {
	t.Helper()
	lis, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	clienttest.ServeConfigDeliver(t, lis, clienttest.ConfigLedger{
		NewestNumber: 10, ConfigIndex: 5, ConfigSequence: configSequence,
	})
	return lis.Addr().String()
}

// serveAssemblerWithSigs is serveAssembler at config sequence 1 but sets the
// served config block's SIGNATURES metadata to sigs.
func serveAssemblerWithSigs(t *testing.T, sigs string) string {
	t.Helper()
	lis, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	clienttest.ServeConfigDeliver(t, lis, clienttest.ConfigLedger{
		NewestNumber: 10, ConfigIndex: 5, ConfigSequence: 1,
		ConfigSignatureMetadata: []byte(sigs),
	})
	return lis.Addr().String()
}

// captureStdout redirects os.Stdout for the duration of fn and returns what fn
// wrote to it.
func captureStdout(t *testing.T, fn func()) string {
	t.Helper()
	orig := os.Stdout
	r, w, err := os.Pipe()
	require.NoError(t, err)
	os.Stdout = w
	defer func() { os.Stdout = orig }()

	fn()

	require.NoError(t, w.Close())
	out, err := io.ReadAll(r)
	require.NoError(t, err)
	return string(out)
}

// TestRunErrors asserts that `follow` reports readable errors, before it begins
// polling, for a missing config block and a malformed config block.
func TestRunErrors(t *testing.T) {
	t.Parallel()

	configPath := filepath.Join(t.TempDir(), "admin.yaml")
	require.NoError(t, os.WriteFile(configPath,
		[]byte("msp:\n  localMspID: org1\n  localMspDir: /tmp\n"), 0o600))
	malformedBlock := filepath.Join(t.TempDir(), "current.pb")
	require.NoError(t, os.WriteFile(malformedBlock, []byte("not a block"), 0o600))

	outputPath := filepath.Join(t.TempDir(), "next_config.pb")

	t.Run("missing config block", func(t *testing.T) {
		t.Parallel()
		err := follow.New().Run(configPath, filepath.Join(t.TempDir(), "absent.pb"), outputPath, 30*time.Second)
		require.ErrorContains(t, err, "failed to read config block")
	})

	t.Run("malformed config block", func(t *testing.T) {
		t.Parallel()
		err := follow.New().Run(configPath, malformedBlock, outputPath, 30*time.Second)
		require.ErrorContains(t, err, "failed to unmarshal config block")
	})
}

// newFollowInputs builds a config block whose assembler endpoints are
// assemblerEndpoints (one party each), writes it and an admin configuration YAML
// for the orderer org's admin user, and returns their paths.
func newFollowInputs(t *testing.T, channelID string, assemblerEndpoints []string) (configPath, blockPath string) {
	t.Helper()

	shared := &ordererpb.SharedConfig{}
	for i, endpoint := range assemblerEndpoints {
		host, portStr, err := net.SplitHostPort(endpoint)
		require.NoError(t, err)
		port, err := strconv.ParseUint(portStr, 10, 32)
		require.NoError(t, err)
		shared.PartiesConfig = append(shared.PartiesConfig, &ordererpb.PartyConfig{
			PartyID:         uint32(i + 1),
			RouterConfig:    &ordererpb.RouterNodeConfig{Host: host, Port: uint32(port)},
			AssemblerConfig: &ordererpb.AssemblerNodeConfig{Host: host, Port: uint32(port)},
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

	orgsPath := filepath.Join(targetPath, cryptogen.OrdererOrganizationsDir)
	orgDirs, err := os.ReadDir(orgsPath)
	require.NoError(t, err)
	require.NotEmpty(t, orgDirs)
	orgName := orgDirs[0].Name()
	mspID := strings.TrimSuffix(orgName, ".com")
	mspDir := filepath.Join(orgsPath, orgName, "users", "Admin@"+orgName, "msp")

	configPath = filepath.Join(t.TempDir(), "admin.yaml")
	content, err := yaml.Marshal(user.Config{
		MSP: user.MSPConfig{LocalMspID: mspID, LocalMspDir: mspDir},
	})
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(configPath, content, 0o600))
	return configPath, blockPath
}

// sequenceOfWrittenBlock reads the config block from path and returns
// its config sequence.
func sequenceOfWrittenBlock(t *testing.T, path string) uint64 {
	t.Helper()
	block, err := client.ReadConfigBlock(path)
	require.NoError(t, err)
	sequence, err := client.SequenceFromBlock(block)
	require.NoError(t, err)
	return sequence
}
