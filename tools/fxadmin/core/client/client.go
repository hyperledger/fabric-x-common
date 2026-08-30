/*
Copyright IBM Corp. All Rights Reserved.

SPDX-License-Identifier: Apache-2.0
*/

// Package client connects to the Fabric-X orderer. It combines the user
// identity and client-TLS material (user.Config) with the orderer
// connection information read from a config block (ordererconn.Info)
// to sign and send seek and broadcast envelopes to assemblers and routers.
package client

import (
	"context"
	"encoding/pem"
	"fmt"
	"os"
	"time"

	"github.com/cockroachdb/errors"
	"github.com/hyperledger/fabric-lib-go/bccsp"
	"github.com/hyperledger/fabric-lib-go/common/flogging"
	cb "github.com/hyperledger/fabric-protos-go-apiv2/common"
	ab "github.com/hyperledger/fabric-protos-go-apiv2/orderer"
	"google.golang.org/grpc"
	"google.golang.org/protobuf/proto"

	"github.com/hyperledger/fabric-x-common/common/util"
	"github.com/hyperledger/fabric-x-common/msp"
	"github.com/hyperledger/fabric-x-common/protoutil"
	"github.com/hyperledger/fabric-x-common/tools/fxadmin/core/ordererconn"
	"github.com/hyperledger/fabric-x-common/tools/fxadmin/core/seek"
	"github.com/hyperledger/fabric-x-common/tools/fxadmin/core/signer"
	"github.com/hyperledger/fabric-x-common/tools/fxadmin/core/user"
	"github.com/hyperledger/fabric-x-common/tools/pkg/comm"
)

const rpcTimeout = 30 * time.Second

var logger = flogging.MustGetLogger("fxadmin.client")

// Client sends signed seek and broadcast envelopes to the orderer
// using the user identity for signing and TLS.
type Client struct {
	ordererConnInfo *ordererconn.Info
	clientConfig    comm.ClientConfig
	signer          msp.SigningIdentity
	tlsCertHash     []byte
}

// load builds a Client from the user configuration and the config block.
func load(config *user.Config, block *cb.Block, csp bccsp.BCCSP) (*Client, error) {
	ordererConnInfo, err := ordererconn.Load(block, csp)
	if err != nil {
		return nil, err
	}
	return LoadWithConnInfo(config, ordererConnInfo)
}

// LoadWithConnInfo builds a Client that dials the orderer described by info,
// signing with the identity in config.
func LoadWithConnInfo(config *user.Config, info *ordererconn.Info) (*Client, error) {
	clientConfig, err := newClientConfig(config, info)
	if err != nil {
		return nil, err
	}
	signingIdentity, err := signer.New(config.MSP.LocalMspID, config.MSP.LocalMspDir)
	if err != nil {
		return nil, err
	}
	certHash, err := tlsCertHash(clientConfig)
	if err != nil {
		return nil, err
	}
	return &Client{
		ordererConnInfo: info,
		clientConfig:    clientConfig,
		signer:          signingIdentity,
		tlsCertHash:     certHash,
	}, nil
}

// LoadFromFiles builds a Client from the user configuration YAML at configPath
// and the config block at blockPath, using csp to build the channel config
// bundle.
func LoadFromFiles(configPath, blockPath string, csp bccsp.BCCSP) (*Client, error) {
	config, err := user.LoadConfig(configPath)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to load user configuration %q", configPath)
	}
	block, err := ReadConfigBlock(blockPath)
	if err != nil {
		return nil, err
	}
	return load(config, block, csp)
}

// ReadConfigBlock reads and unmarshals a protobuf config block from path. It is
// shared by the commands that take a --current-block file.
func ReadConfigBlock(path string) (*cb.Block, error) {
	content, err := os.ReadFile(path)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to read config block %q", path)
	}
	block, err := protoutil.UnmarshalBlock(content)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to unmarshal config block %q", path)
	}
	return block, nil
}

// WriteBlock marshals block and writes it to path.
func WriteBlock(block *cb.Block, path string) error {
	content, err := proto.Marshal(block)
	if err != nil {
		return errors.Wrap(err, "failed to marshal block")
	}
	if err := os.WriteFile(path, content, 0o600); err != nil {
		return errors.Wrapf(err, "failed to write block to %q", path)
	}
	return nil
}

// SequenceFromBlock returns the config sequence recorded in a config block's
// configuration envelope. It reads the sequence field directly rather than
// building a channel config bundle, so it needs no BCCSP.
func SequenceFromBlock(block *cb.Block) (uint64, error) {
	envelope, err := protoutil.ExtractEnvelope(block, 0)
	if err != nil {
		return 0, errors.Wrap(err, "failed to extract envelope from config block")
	}
	payload, err := protoutil.UnmarshalPayload(envelope.GetPayload())
	if err != nil {
		return 0, errors.Wrap(err, "failed to unmarshal config block payload")
	}
	configEnvelope := &cb.ConfigEnvelope{}
	if err := proto.Unmarshal(payload.GetData(), configEnvelope); err != nil {
		return 0, errors.Wrap(err, "failed to unmarshal config envelope")
	}
	if configEnvelope.GetConfig() == nil {
		return 0, errors.New("config block carries no config")
	}
	return configEnvelope.GetConfig().GetSequence(), nil
}

// ChannelID returns the channel the client targets.
func (c *Client) ChannelID() string {
	return c.ordererConnInfo.ChannelID
}

// NumParties returns the number of parties in the network the client targets,
// as read from the config block's ARMA shared config.
func (c *Client) NumParties() int {
	return c.ordererConnInfo.NumParties
}

// AssemblerEndpoints returns the assembler endpoints the client targets, in
// config-block order.
func (c *Client) AssemblerEndpoints() []string {
	return c.ordererConnInfo.AssemblerEndpoints
}

// FetchBlock seeks the block described by seek from the first reachable
// assembler, failing over to the next assembler on error.
func (c *Client) FetchBlock(seek *ab.SeekInfo) (*cb.Block, error) {
	if len(c.ordererConnInfo.AssemblerEndpoints) == 0 {
		return nil, errors.New("no assembler endpoints configured")
	}

	envelope, err := c.createSignedDeliverSeekEnvelope(seek)
	if err != nil {
		return nil, err
	}

	var errs []error
	for _, endpoint := range c.ordererConnInfo.AssemblerEndpoints {
		block, err := c.fetchBlockFromEndpoint(context.Background(), endpoint, envelope)
		if err == nil {
			return block, nil
		}
		logger.Debugf("failed to fetch block from assembler %s: %v", endpoint, err)
		errs = append(errs, fmt.Errorf("assembler %s: %w", endpoint, err))
	}
	return nil, errors.Wrap(errors.Join(errs...), "failed to fetch block from all assemblers")
}

// LedgerStatus summarizes a single assembler's ledger: the number of its newest
// block (its height indicator), the config sequence of its last config block,
// and the last config block itself.
type LedgerStatus struct {
	LastBlockNumber    uint64
	LastConfigSequence uint64
	ConfigBlock        *cb.Block
}

// FetchLedgerStatus returns the ledger status of a single assembler. It seeks the
// assembler's newest block (whose number is the ledger's last block), follows its
// last-config index to the config block, and reads that block's sequence. Unlike
// FetchBlock, it targets one endpoint and does not fail over, so callers (e.g.
// `fxadmin follow`) can track each assembler's ledger independently. ctx bounds
// the whole operation, including both underlying block fetches.
func (c *Client) FetchLedgerStatus(ctx context.Context, endpoint string) (LedgerStatus, error) {
	newestEnvelope, err := c.createSignedDeliverSeekEnvelope(seek.Newest())
	if err != nil {
		return LedgerStatus{}, err
	}
	newest, err := c.fetchBlockFromEndpoint(ctx, endpoint, newestEnvelope)
	if err != nil {
		return LedgerStatus{}, err
	}
	lastBlock := newest.GetHeader().GetNumber()

	lastConfigIndex, err := protoutil.GetLastConfigIndexFromBlock(newest)
	if err != nil {
		return LedgerStatus{}, errors.Wrapf(err, "failed to read last config index from block %d", lastBlock)
	}

	configBlock := newest
	if lastBlock != lastConfigIndex {
		var configEnvelope *cb.Envelope
		configEnvelope, err = c.createSignedDeliverSeekEnvelope(seek.ByNumber(lastConfigIndex))
		if err != nil {
			return LedgerStatus{}, err
		}
		if configBlock, err = c.fetchBlockFromEndpoint(ctx, endpoint, configEnvelope); err != nil {
			return LedgerStatus{}, err
		}
	}

	sequence, err := SequenceFromBlock(configBlock)
	if err != nil {
		return LedgerStatus{}, err
	}
	return LedgerStatus{LastBlockNumber: lastBlock, LastConfigSequence: sequence, ConfigBlock: configBlock}, nil
}

// RouterStatus is the outcome of broadcasting an envelope to a single router.
// Err is nil when the router acknowledged the envelope with status SUCCESS, and
// otherwise describes the rejection or the reason the router was unreachable.
type RouterStatus struct {
	Endpoint string
	Err      error
}

// BroadcastToAllRouters sends a prepared envelope to every router and returns
// one RouterStatus per router, in endpoint order, reporting whether each router
// acknowledged the envelope. The envelope must already be built and signed by
// the caller (e.g. a prepared configuration transaction). It returns an error
// only when no routers are configured; individual router rejections and
// unreachable routers are carried in the returned statuses.
func (c *Client) BroadcastToAllRouters(envelope *cb.Envelope) ([]RouterStatus, error) {
	if len(c.ordererConnInfo.RouterEndpoints) == 0 {
		return nil, errors.New("no router endpoints configured")
	}

	statuses := make([]RouterStatus, 0, len(c.ordererConnInfo.RouterEndpoints))
	for _, endpoint := range c.ordererConnInfo.RouterEndpoints {
		statuses = append(statuses, RouterStatus{
			Endpoint: endpoint,
			Err:      c.broadcastToRouter(endpoint, envelope),
		})
	}
	return statuses, nil
}

// signedSeekEnvelope wraps seek in a signed DELIVER_SEEK_INFO envelope for the
// client's channel, binding the client TLS certificate hash when mutual TLS is
// in use.
func (c *Client) createSignedDeliverSeekEnvelope(seek *ab.SeekInfo) (*cb.Envelope, error) {
	envelope, err := protoutil.CreateSignedEnvelopeWithTLSBinding(
		cb.HeaderType_DELIVER_SEEK_INFO,
		c.ChannelID(),
		c.signer,
		seek,
		0,
		0,
		c.tlsCertHash,
	)
	if err != nil {
		return nil, errors.Wrap(err, "failed to create signed seek envelope")
	}
	return envelope, nil
}

// dial opens a gRPC connection to endpoint, bounded by both ctx and the
// configured DialTimeout, so a caller's deadline is honored while a single dial
// is still capped. It mirrors comm.ClientConfig.Dial but derives its context
// from ctx instead of context.Background().
func (c *Client) dial(ctx context.Context, endpoint string) (*grpc.ClientConn, error) {
	dialOpts, err := c.clientConfig.DialOptions()
	if err != nil {
		return nil, err
	}
	dialCtx, cancel := context.WithTimeout(ctx, c.clientConfig.DialTimeout)
	defer cancel()
	//nolint:staticcheck // grpc.DialContext mirrors comm.ClientConfig.Dial.
	conn, err := grpc.DialContext(dialCtx, endpoint, dialOpts...)
	if err != nil {
		return nil, errors.Wrap(err, "failed to create new connection")
	}
	return conn, nil
}

// fetchBlockFromEndpoint opens a Deliver stream to endpoint, sends the seek envelope, and
// receives exactly one block. ctx bounds both the dial and the RPC (capped by
// rpcTimeout).
func (c *Client) fetchBlockFromEndpoint(
	ctx context.Context, endpoint string, envelope *cb.Envelope,
) (*cb.Block, error) {
	conn, err := c.dial(ctx, endpoint)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to dial assembler %s", endpoint)
	}
	defer func() { _ = conn.Close() }()

	ctx, cancel := context.WithTimeout(ctx, rpcTimeout)
	defer cancel()

	stream, err := ab.NewAtomicBroadcastClient(conn).Deliver(ctx)
	if err != nil {
		return nil, errors.Wrap(err, "failed to open deliver stream")
	}
	defer func() { _ = stream.CloseSend() }()

	if err = stream.Send(envelope); err != nil {
		return nil, errors.Wrap(err, "failed to send seek envelope")
	}

	resp, err := stream.Recv()
	if err != nil {
		return nil, errors.Wrap(err, "failed to receive deliver response")
	}

	switch t := resp.GetType().(type) {
	case *ab.DeliverResponse_Block:
		block := t.Block
		if block.GetData() == nil || len(block.GetData().GetData()) == 0 {
			return nil, errors.New("received an empty block")
		}
		return block, nil
	case *ab.DeliverResponse_Status:
		return nil, errors.Newf("assembler rejected seek request with status %s", t.Status)
	default:
		return nil, errors.Newf("received an unexpected deliver response type: %T", resp.GetType())
	}
}

// broadcastToRouter opens a Broadcast stream to endpoint, sends the envelope, and
// checks the response status.
func (c *Client) broadcastToRouter(endpoint string, envelope *cb.Envelope) error {
	conn, err := c.clientConfig.Dial(endpoint)
	if err != nil {
		return errors.Wrapf(err, "failed to dial router %s", endpoint)
	}
	defer func() { _ = conn.Close() }()

	ctx, cancel := context.WithTimeout(context.Background(), rpcTimeout)
	defer cancel()

	stream, err := ab.NewAtomicBroadcastClient(conn).Broadcast(ctx)
	if err != nil {
		return errors.Wrap(err, "failed to open broadcast stream")
	}
	defer func() { _ = stream.CloseSend() }()

	if err = stream.Send(envelope); err != nil {
		return errors.Wrap(err, "failed to send broadcast envelope")
	}

	resp, err := stream.Recv()
	if err != nil {
		return errors.Wrap(err, "failed to receive broadcast response")
	}
	if resp.GetStatus() != cb.Status_SUCCESS {
		return errors.Newf("router returned status %s: %s", resp.GetStatus(), resp.GetInfo())
	}
	return nil
}

// newClientConfig builds the gRPC client configuration from the user TLS
// material and the TLS CA certificates carried by the config block.
func newClientConfig(config *user.Config, info *ordererconn.Info) (comm.ClientConfig, error) {
	cc := comm.ClientConfig{DialTimeout: comm.DefaultConnectionTimeout}

	if !config.TLS.IsEnabled() {
		return cc, nil
	}

	if len(info.TLSCACerts) == 0 {
		return cc, errors.New("TLS is enabled but the config block carries no TLS CA certificates")
	}

	cc.SecOpts = comm.SecureOptions{
		UseTLS:        true,
		ServerRootCAs: info.TLSCACerts,
	}

	if config.TLS.ClientCert == "" {
		return cc, nil
	}

	cert, err := readFile(config.TLS.ClientCert)
	if err != nil {
		return cc, err
	}
	key, err := readFile(config.TLS.ClientKey)
	if err != nil {
		return cc, err
	}
	cc.SecOpts.RequireClientCert = true
	cc.SecOpts.Certificate = cert
	cc.SecOpts.Key = key
	return cc, nil
}

// tlsCertHash returns the SHA-256 of the client TLS certificate for binding
// into the signed envelope, or nil when mutual TLS is not in use.
func tlsCertHash(cc comm.ClientConfig) ([]byte, error) {
	if !cc.SecOpts.UseTLS || !cc.SecOpts.RequireClientCert {
		return nil, nil
	}
	block, _ := pem.Decode(cc.SecOpts.Certificate)
	if block == nil || block.Type != "CERTIFICATE" {
		return nil, errors.New("failed to decode client TLS certificate PEM")
	}
	return util.ComputeSHA256(block.Bytes), nil
}

// readFile reads a file, wrapping the error with its path.
func readFile(path string) ([]byte, error) {
	content, err := os.ReadFile(path)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to read file %q", path)
	}
	return content, nil
}
