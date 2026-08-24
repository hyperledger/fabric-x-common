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

	"github.com/hyperledger/fabric-x-common/common/util"
	"github.com/hyperledger/fabric-x-common/msp"
	"github.com/hyperledger/fabric-x-common/protoutil"
	"github.com/hyperledger/fabric-x-common/tools/fxadmin/core/ordererconn"
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
	clientConfig, err := newClientConfig(config, ordererConnInfo)
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
		ordererConnInfo: ordererConnInfo,
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
	block, err := readBlock(blockPath)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to load config block %q", blockPath)
	}
	return load(config, block, csp)
}

// readBlock reads and unmarshals a protobuf config block from path.
func readBlock(path string) (*cb.Block, error) {
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

// ChannelID returns the channel the client targets.
func (c *Client) ChannelID() string {
	return c.ordererConnInfo.ChannelID
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
		block, err := c.fetchBlockFromEndpoint(endpoint, envelope)
		if err == nil {
			return block, nil
		}
		logger.Debugf("failed to fetch block from assembler %s: %v", endpoint, err)
		errs = append(errs, fmt.Errorf("assembler %s: %w", endpoint, err))
	}
	return nil, errors.Wrap(errors.Join(errs...), "failed to fetch block from all assemblers")
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

// fetchBlockFromEndpoint opens a Deliver stream to endpoint, sends the seek envelope, and
// receives exactly one block.
func (c *Client) fetchBlockFromEndpoint(endpoint string, envelope *cb.Envelope) (*cb.Block, error) {
	conn, err := c.clientConfig.Dial(endpoint)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to dial assembler %s", endpoint)
	}
	defer func() { _ = conn.Close() }()

	ctx, cancel := context.WithTimeout(context.Background(), rpcTimeout)
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
