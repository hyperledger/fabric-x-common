/*
Copyright IBM Corp. All Rights Reserved.

SPDX-License-Identifier: Apache-2.0
*/

package msp

import (
	"fmt"
	"reflect"
	"runtime"
	"testing"

	idemixmsp "github.com/IBM/idemix/msp"
	"github.com/hyperledger/fabric-lib-go/bccsp/factory"
	"github.com/hyperledger/fabric-lib-go/bccsp/sw"
	msppb "github.com/hyperledger/fabric-protos-go-apiv2/msp"
	"github.com/stretchr/testify/require"

	"github.com/hyperledger/fabric-x-common/core/config/configtest"
)

const sampleOrgMSPName = "SampleOrg"

func TestNewInvalidOpts(t *testing.T) {
	t.Parallel()
	cryptoProvider, err := sw.NewDefaultSecurityLevelWithKeystore(sw.NewDummyKeyStore())
	require.NoError(t, err)

	i, err := New(nil, cryptoProvider)
	require.Error(t, err)
	require.Contains(t, err.Error(), "Invalid msp.NewOpts instance. It must be either *BCCSPNewOpts or *IdemixNewOpts. It was [<nil>]")
	require.Nil(t, i)

	i, err = New(&BCCSPNewOpts{NewBaseOpts{Version: -1}}, cryptoProvider)
	require.Error(t, err)
	require.Contains(t, err.Error(), "Invalid *BCCSPNewOpts. Version not recognized [-1]")
	require.Nil(t, i)

	i, err = New(&IdemixNewOpts{NewBaseOpts{Version: -1}}, cryptoProvider)
	require.Error(t, err)
	require.Contains(t, err.Error(), "Invalid *IdemixNewOpts. Version not recognized [-1]")
	require.Nil(t, i)
}

func TestNewBCCSP(t *testing.T) {
	t.Parallel()
	cryptoProvider, err := sw.NewDefaultSecurityLevelWithKeystore(sw.NewDummyKeyStore())
	require.NoError(t, err)

	for _, tc := range []struct {
		name                 string
		version              MSPVersion
		expectedSetupFunc    mspSetupFuncType
		expectedValidateFunc validateIdentityOUsFuncType
	}{
		{
			name:                 "MSPv1_0",
			version:              MSPv1_0,
			expectedSetupFunc:    (&bccspmsp{}).setupV1,
			expectedValidateFunc: (&bccspmsp{}).validateIdentityOUsV1,
		},
		{
			name:                 "MSPv1_1",
			version:              MSPv1_1,
			expectedSetupFunc:    (&bccspmsp{}).setupV11,
			expectedValidateFunc: (&bccspmsp{}).validateIdentityOUsV11,
		},
		{
			name:                 "MSPv1_3",
			version:              MSPv1_3,
			expectedSetupFunc:    (&bccspmsp{}).setupV11,
			expectedValidateFunc: (&bccspmsp{}).validateIdentityOUsV11,
		},
		{
			name:                 "MSPv1_4_3",
			version:              MSPv1_4_3,
			expectedSetupFunc:    (&bccspmsp{}).setupV142,
			expectedValidateFunc: (&bccspmsp{}).validateIdentityOUsV142,
		},
		{
			name:                 "MSPv3_0",
			version:              MSPv3_0,
			expectedSetupFunc:    (&bccspmsp{}).setupV3,
			expectedValidateFunc: (&bccspmsp{}).validateIdentityOUsV142,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			i, err := New(&BCCSPNewOpts{NewBaseOpts{Version: tc.version}}, cryptoProvider)
			require.NoError(t, err)
			require.NotNil(t, i)
			bccspMSP, ok := i.(*bccspmsp)
			require.True(t, ok)
			require.Equal(t, tc.version, bccspMSP.version)
			require.Equal(t, funcName(t, tc.expectedSetupFunc), funcName(t, bccspMSP.internalSetupFunc))
			require.Equal(t,
				funcName(t, tc.expectedValidateFunc),
				funcName(t, bccspMSP.internalValidateIdentityOusFunc),
			)
		})
	}
}

func TestNewIdemix(t *testing.T) {
	t.Parallel()
	cryptoProvider, err := sw.NewDefaultSecurityLevelWithKeystore(sw.NewDummyKeyStore())
	require.NoError(t, err)

	for _, version := range []MSPVersion{MSPv1_0, MSPv1_1, MSPv1_3, MSPv1_4_3, MSPv3_0} {
		t.Run(fmt.Sprintf("version_%d", version), func(t *testing.T) {
			t.Parallel()
			i, err := New(&IdemixNewOpts{NewBaseOpts{Version: version}}, cryptoProvider)
			require.NoError(t, err)
			require.NotNil(t, i)
			wrapper, ok := i.(*idemixMSPWrapper)
			require.True(t, ok)
			require.Equal(t, idemixmsp.MSPVersion(version), wrapper.Idemixmsp.GetVersion())
		})
	}
}

func TestLoadLocalMspDir(t *testing.T) {
	t.Parallel()
	mspDir := configtest.GetDevMspDir()

	i, err := LoadLocalMspDir(DirLoadParameters{MspDir: mspDir, MspName: sampleOrgMSPName})
	require.NoError(t, err)
	require.NotNil(t, i)
	id, err := i.GetIdentifier()
	require.NoError(t, err)
	require.Equal(t, sampleOrgMSPName, id)

	i, err = LoadLocalMspDir(DirLoadParameters{MspDir: mspDir})
	require.NoError(t, err)
	require.NotNil(t, i)
	id, err = i.GetIdentifier()
	require.NoError(t, err)
	require.Equal(t, "msp", id)
}

func TestLoadLocalMspDirFailure(t *testing.T) {
	t.Parallel()
	i, err := LoadLocalMspDir(DirLoadParameters{MspDir: "/path/does/not/exist", MspName: sampleOrgMSPName})
	require.ErrorContains(t, err, "error loading local MSP configuration")
	require.Nil(t, i)
}

func TestLoadVerifyingMspDir(t *testing.T) {
	t.Parallel()
	mspDir := configtest.GetDevMspDir()

	i, err := LoadVerifyingMspDir(DirLoadParameters{MspDir: mspDir, MspName: sampleOrgMSPName})
	require.NoError(t, err)
	require.NotNil(t, i)
	id, err := i.GetIdentifier()
	require.NoError(t, err)
	require.Equal(t, sampleOrgMSPName, id)
}

func TestLoadVerifyingMspDirFailure(t *testing.T) {
	t.Parallel()
	i, err := LoadVerifyingMspDir(DirLoadParameters{MspDir: "/path/does/not/exist", MspName: sampleOrgMSPName})
	require.ErrorContains(t, err, "error loading verifing MSP configuration")
	require.Nil(t, i)
}

func TestLoadMSPFailure(t *testing.T) {
	t.Parallel()
	i, err := loadMSP(&factory.FactoryOpts{Default: "unknown-provider"}, &msppb.MSPConfig{})
	require.ErrorContains(t, err, "error getting BCCSP from config")
	require.Nil(t, i)
}

func TestDefaultParameters(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name            string
		input           DirLoadParameters
		expectedMspName string
	}{
		{
			name:            "empty parameters get defaulted",
			input:           DirLoadParameters{},
			expectedMspName: "msp",
		},
		{
			name:            "existing parameters are preserved",
			input:           DirLoadParameters{MspName: sampleOrgMSPName, CspConf: factory.GetDefaultOpts()},
			expectedMspName: sampleOrgMSPName,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			out := defaultParameters(tc.input)
			require.Equal(t, tc.expectedMspName, out.MspName)
			require.NotNil(t, out.CspConf)
		})
	}
}

func funcName(t *testing.T, f any) string {
	t.Helper()
	return runtime.FuncForPC(reflect.ValueOf(f).Pointer()).Name()
}
