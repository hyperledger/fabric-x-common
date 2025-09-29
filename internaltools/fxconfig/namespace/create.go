/*
Copyright IBM Corp. All Rights Reserved.

SPDX-License-Identifier: Apache-2.0
*/

package namespace

import (
	"context"
	"encoding/pem"
	"errors"
	"fmt"
	"os"
	"path"

	"github.com/hyperledger/fabric-lib-go/bccsp/sw"
	cb "github.com/hyperledger/fabric-protos-go-apiv2/common"
	msppb "github.com/hyperledger/fabric-protos-go-apiv2/msp"
	ab "github.com/hyperledger/fabric-protos-go-apiv2/orderer"
	"github.com/hyperledger/fabric-x-committer/api/protoblocktx"
	"github.com/hyperledger/fabric-x-committer/api/types"
	"github.com/hyperledger/fabric-x-committer/utils/signature"
	"google.golang.org/protobuf/proto"

	"github.com/hyperledger/fabric-x-common/cmd/common/comm"
	"github.com/hyperledger/fabric-x-common/internaltools/configtxgen/encoder"
	"github.com/hyperledger/fabric-x-common/internaltools/pkg/identity"
	"github.com/hyperledger/fabric-x-common/msp"
	"github.com/hyperledger/fabric-x-common/protoutil"
)

// OrdererConfig is helper struct to deal with orderer-related arguments.
type OrdererConfig struct {
	OrderingEndpoint string
	Config           comm.Config
}

// MSPConfig is helper struct to deal with MSP-related arguments.
type MSPConfig struct {
	MSPConfigPath string
	MSPID         string
}

// DeployNamespace creates a namespace transactions and submits it to the ordering service.
func DeployNamespace(chName, nsID string, nsVersion int, odererCfg OrdererConfig, mspCfg MSPConfig, pkPath string) error {
	thisMSP, err := setupMSP(mspCfg)
	if err != nil {
		return fmt.Errorf("msp setup error: %w", err)
	}

	sid, err := thisMSP.GetDefaultSigningIdentity()
	if err != nil {
		return fmt.Errorf("get signer identity error: %w", err)
	}

	var pkData []byte
	if pkPath == "" {
		// if `pkPath` isn't set, use the default MSP signer as namespace EP
		ssid, err := sid.Serialize()
		if err != nil {
			return err
		}

		mspSI := &msppb.SerializedIdentity{}
		if err := proto.Unmarshal(ssid, mspSI); err != nil {
			return err
		}

		pkData = mspSI.IdBytes
	} else {
		var err error
		pkData, err = os.ReadFile(pkPath)
		if err != nil {
			return err
		}
	}

	serializedPublicKey, err := getPubKeyFromPemData(pkData)
	if err != nil {
		return err
	}

	tx, err := createNamespacesTx("ECDSA", serializedPublicKey, nsID, nsVersion)
	if err != nil {
		return err
	}

	env, err := createSignedEnvelope(sid, chName, tx)
	if err != nil {
		return err
	}

	return broadcast(odererCfg, env)
}

func setupMSP(mspCfg MSPConfig) (msp.MSP, error) {
	conf, err := msp.GetLocalMspConfig(mspCfg.MSPConfigPath, nil, mspCfg.MSPID)
	if err != nil {
		return nil, err
	}

	dir := path.Join(mspCfg.MSPConfigPath, "keystore")
	ks, err := sw.NewFileBasedKeyStore(nil, dir, true)
	if err != nil {
		return nil, err
	}

	cp, err := sw.NewDefaultSecurityLevelWithKeystore(ks)
	if err != nil {
		return nil, err
	}

	mspOpts := &msp.BCCSPNewOpts{
		NewBaseOpts: msp.NewBaseOpts{
			Version: msp.MSPv1_0,
		},
	}

	thisMSP, err := msp.New(mspOpts, cp)
	if err != nil {
		return nil, err
	}

	err = thisMSP.Setup(conf)
	if err != nil {
		return nil, err
	}

	return thisMSP, nil
}

// getPubKeyFromPemData looks for ECDSA public key in PEM file, and returns pem content only with the public key.
func getPubKeyFromPemData(pemContent []byte) ([]byte, error) {
	for {
		block, rest := pem.Decode(pemContent)
		if block == nil {
			break
		}
		pemContent = rest

		key, err := encoder.ParseCertificateOrPublicKey(block.Bytes)
		if err != nil {
			continue
		}
		return pem.EncodeToMemory(&pem.Block{
			Type:  "PUBLIC KEY",
			Bytes: key,
		}), nil
	}

	return nil, errors.New("no ECDSA public key in pem file")
}

func createNamespacesTx(policyScheme string, policyVerificationKey []byte, nsID string, nsVersion int) (*protoblocktx.Tx, error) {
	writeToMetaNs := &protoblocktx.TxNamespace{
		NsId: types.MetaNamespaceID,
		// TODO we need the correct version of the metaNamespaceID
		NsVersion:  uint64(0),
		ReadWrites: make([]*protoblocktx.ReadWrite, 0, 1),
	}

	nsPolicy := &protoblocktx.NamespacePolicy{
		Scheme:    policyScheme,
		PublicKey: policyVerificationKey,
	}

	policyBytes, err := protoutil.Marshal(nsPolicy)
	if err != nil {
		return nil, err
	}

	rw := &protoblocktx.ReadWrite{
		Key:   []byte(nsID),
		Value: policyBytes,
	}

	// note that we only set the version if we update a namespace policy
	if nsVersion >= 0 {
		rw.Version = types.Version(uint64(nsVersion))
	}

	writeToMetaNs.ReadWrites = append(writeToMetaNs.ReadWrites, rw)

	tx := &protoblocktx.Tx{
		Namespaces: []*protoblocktx.TxNamespace{
			writeToMetaNs,
		},
	}

	return tx, nil
}

func createSignedEnvelope(signer identity.SignerSerializer, channel string, tx *protoblocktx.Tx) (*cb.Envelope, error) {
	signatureHeader, err := protoutil.NewSignatureHeader(signer)
	if err != nil {
		return nil, err
	}

	tx.Id = protoutil.ComputeTxID(signatureHeader.Nonce, signatureHeader.Creator)
	tx.Signatures = make([][]byte, len(tx.GetNamespaces()))
	for idx := range tx.GetNamespaces() {
		// Note that a default msp signer hash the msg before signing.
		// For that reason we use the TxNamespace message as ASN1 encoded msg
		msg, err := signature.ASN1MarshalTxNamespace(tx, idx)
		if err != nil {
			return nil, fmt.Errorf("failed asn1 marshal tx: %w", err)
		}

		sig, err := signer.Sign(msg)
		if err != nil {
			return nil, fmt.Errorf("failed signing tx: %w", err)
		}
		tx.Signatures[idx] = sig
	}

	txBytes, err := proto.Marshal(tx)
	if err != nil {
		return nil, fmt.Errorf("failed marshaling transaction: %w", err)
	}

	channelHeader := protoutil.MakeChannelHeader(cb.HeaderType_MESSAGE, 0, channel, 0)
	channelHeader.TxId = tx.Id

	hdr := protoutil.MakePayloadHeader(channelHeader, signatureHeader)
	return protoutil.CreateEnvelope(signer, hdr, txBytes)
}

func broadcast(odererCfg OrdererConfig, env *cb.Envelope) error {
	cl, err := comm.NewClient(odererCfg.Config)
	if err != nil {
		return fmt.Errorf("cannot get grpc client: %w", err)
	}

	conn, err := cl.NewDialer(odererCfg.OrderingEndpoint)()
	if err != nil {
		return fmt.Errorf("cannot get grpc client: %w", err)
	}
	defer func() {
		_ = conn.Close()
	}()

	occ := ab.NewAtomicBroadcastClient(conn)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	abc, err := occ.Broadcast(ctx)
	if err != nil {
		return err
	}

	err = abc.Send(env)
	if err != nil {
		return err
	}

	status, err := abc.Recv()
	if err != nil {
		return err
	}

	if status.GetStatus() != cb.Status_SUCCESS {
		return fmt.Errorf("got error %#v", status.GetStatus())
	}

	return nil
}
