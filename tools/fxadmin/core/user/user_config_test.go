/*
Copyright IBM Corp. All Rights Reserved.

SPDX-License-Identifier: Apache-2.0
*/

package user_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/hyperledger/fabric-x-common/tools/fxadmin/core/user"
)

const (
	testMspID  = "org1"
	testMspDir = "/crypto/ordererOrganizations/org1/msp"
)

func TestLoadAdminConfig(t *testing.T) {
	t.Parallel()

	// success cases
	for _, tc := range []struct {
		name           string
		content        string
		expectedMspID  string
		expectedMspDir string
		expectedTLS    bool
		expectedCert   string
		expectedKey    string
	}{
		{
			name: "mutual TLS",
			content: `
msp:
  localMspID: org1
  localMspDir: /crypto/ordererOrganizations/org1/msp
tls:
  enabled: true
  clientCert: /crypto/ordererOrganizations/org1/users/Admin@org1/tls/client.crt
  clientKey: /crypto/ordererOrganizations/org1/users/Admin@org1/tls/client.key
`,
			expectedMspID:  testMspID,
			expectedMspDir: testMspDir,
			expectedTLS:    true,
			expectedCert:   "/crypto/ordererOrganizations/org1/users/Admin@org1/tls/client.crt",
			expectedKey:    "/crypto/ordererOrganizations/org1/users/Admin@org1/tls/client.key",
		},
		{
			name: "server-only TLS",
			content: `
msp:
  localMspID: org1
  localMspDir: /crypto/ordererOrganizations/org1/msp
tls:
  enabled: true
`,
			expectedMspID:  testMspID,
			expectedMspDir: testMspDir,
			expectedTLS:    true,
		},
		{
			name: "TLS disabled",
			content: `
msp:
  localMspID: org1
  localMspDir: /crypto/ordererOrganizations/org1/msp
tls:
  enabled: false
`,
			expectedMspID:  testMspID,
			expectedMspDir: testMspDir,
			expectedTLS:    false,
		},
		{
			name: "TLS omitted defaults to disabled",
			content: `
msp:
  localMspID: org1
  localMspDir: /crypto/ordererOrganizations/org1/msp
`,
			expectedMspID:  testMspID,
			expectedMspDir: testMspDir,
			expectedTLS:    false,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			config, err := user.LoadConfig(writeTempFile(t, tc.content))
			require.NoError(t, err)
			require.Equal(t, tc.expectedMspID, config.MSP.LocalMspID)
			require.Equal(t, tc.expectedMspDir, config.MSP.LocalMspDir)
			require.Equal(t, tc.expectedTLS, config.TLS.IsEnabled())
			require.Equal(t, tc.expectedCert, config.TLS.ClientCert)
			require.Equal(t, tc.expectedKey, config.TLS.ClientKey)
		})
	}

	// failure cases
	for _, tc := range []struct {
		name          string
		content       string
		expectedError string
	}{
		{
			name:          "missing msp id",
			content:       "msp:\n  localMspDir: /crypto/ordererOrganizations/org1/msp\n",
			expectedError: "msp.localMspID is required",
		},
		{
			name:          "missing msp dir",
			content:       "msp:\n  localMspID: org1\n",
			expectedError: "msp.localMspDir is required",
		},
		{
			name: "TLS enabled with cert but no key",
			content: `
msp:
  localMspID: org1
  localMspDir: /crypto/ordererOrganizations/org1/msp
tls:
  enabled: true
  clientCert: /crypto/ordererOrganizations/org1/users/Admin@org1/tls/client.crt
`,
			expectedError: "tls.clientCert and tls.clientKey must be set together",
		},
		{
			name:          "malformed yaml",
			content:       "msp: [not-a-map\n",
			expectedError: "failed to unmarshal",
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			_, err := user.LoadConfig(writeTempFile(t, tc.content))
			require.ErrorContains(t, err, tc.expectedError)
		})
	}

	t.Run("missing file", func(t *testing.T) {
		t.Parallel()
		_, err := user.LoadConfig(filepath.Join(t.TempDir(), "absent.yaml"))
		require.ErrorContains(t, err, "failed to read user configuration file")
	})
}

// writeTempFile writes content to a temp file and returns its path.
func writeTempFile(t *testing.T, content string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "admin.yaml")
	require.NoError(t, os.WriteFile(path, []byte(content), 0o600))
	return path
}
