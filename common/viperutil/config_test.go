/*
Copyright IBM Corp. All Rights Reserved.

SPDX-License-Identifier: Apache-2.0
*/

package viperutil

import (
	"fmt"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/hyperledger/fabric-lib-go/bccsp/factory"
	"github.com/stretchr/testify/require"

	"github.com/hyperledger/fabric-x-common/api/types"
)

const (
	testConfigName = "viperutil"
	testEnvPrefix  = "VIPERUTIL"
)

func TestEnvSlice(t *testing.T) {
	envVar := testEnvPrefix + "_SLICE"
	t.Setenv(envVar, "[a, b, c]")

	data := "---\nSlice: [d,e,f]"

	config := New()
	config.SetConfigName(testConfigName)
	err := config.ReadConfig(strings.NewReader(data))
	require.NoError(t, err, "error reading %s plugin config", testConfigName)

	var conf struct{ Slice []string }
	err = config.EnhancedExactUnmarshal(&conf)
	require.NoError(t, err, "failed to unmarshal")

	expected := []string{"a", "b", "c"}
	require.Exactly(t, expected, conf.Slice, "did not get the expected slice")
}

func TestByteSize(t *testing.T) {
	t.Parallel()
	testCases := []struct {
		data     string
		expected uint32
	}{
		{"", 0},
		{"42", 42},
		{"42k", 42 * 1024},
		{"42kb", 42 * 1024},
		{"42K", 42 * 1024},
		{"42KB", 42 * 1024},
		{"42 K", 42 * 1024},
		{"42 KB", 42 * 1024},
		{"42m", 42 * 1024 * 1024},
		{"42mb", 42 * 1024 * 1024},
		{"42M", 42 * 1024 * 1024},
		{"42MB", 42 * 1024 * 1024},
		{"42 M", 42 * 1024 * 1024},
		{"42 MB", 42 * 1024 * 1024},
		{"3g", 3 * 1024 * 1024 * 1024},
		{"3gb", 3 * 1024 * 1024 * 1024},
		{"3G", 3 * 1024 * 1024 * 1024},
		{"3GB", 3 * 1024 * 1024 * 1024},
		{"3 G", 3 * 1024 * 1024 * 1024},
		{"3 GB", 3 * 1024 * 1024 * 1024},
	}
	for _, tc := range testCases {
		t.Run(tc.data, func(t *testing.T) {
			t.Parallel()
			data := fmt.Sprintf("---\nByteSize: %s", tc.data)

			config := New()
			err := config.ReadConfig(strings.NewReader(data))
			require.NoError(t, err, "error reading config")

			var conf struct{ ByteSize uint32 }
			err = config.EnhancedExactUnmarshal(&conf)
			require.NoError(t, err, "failed to unmarshal")
			require.Exactly(t, tc.expected, conf.ByteSize, "incorrect byte size")
		})
	}
}

func TestByteSize64(t *testing.T) {
	t.Parallel()
	testCases := []struct {
		data     string
		expected uint64
	}{
		{"8 GB", 8 * 1024 * 1024 * 1024},
		{"128 GB", 128 * 1024 * 1024 * 1024},
	}
	for _, tc := range testCases {
		t.Run(tc.data, func(t *testing.T) {
			t.Parallel()
			data := fmt.Sprintf("---\nByteSize: %s", tc.data)

			config := New()
			err := config.ReadConfig(strings.NewReader(data))
			require.NoError(t, err, "error reading config")

			var conf struct{ ByteSize uint64 }
			err = config.EnhancedExactUnmarshal(&conf)
			require.NoError(t, err, "failed to unmarshal")
			require.Exactly(t, tc.expected, conf.ByteSize, "incorrect byte size")
		})
	}
}

func TestByteSizeOverflow(t *testing.T) {
	t.Parallel()
	data := "---\nByteSize: 4GB"

	config := New()
	err := config.ReadConfig(strings.NewReader(data))
	require.NoError(t, err, "error reading config")

	var conf struct{ ByteSize uint32 }
	err = config.EnhancedExactUnmarshal(&conf)
	require.Error(t, err)
	require.Contains(t, err.Error(), "ByteSize")
	require.Contains(t, err.Error(), "value '4GB' overflows uint32")
}

type stringFromFileConfig struct {
	Inner struct {
		Single   string
		Multiple []string
	}
}

func TestStringNotFromFile(t *testing.T) {
	t.Parallel()
	yaml := "---\nInner:\n  Single: expected_value\n"

	config := New()
	err := config.ReadConfig(strings.NewReader(yaml))
	require.NoError(t, err, "error reading config")

	var uconf stringFromFileConfig
	err = config.EnhancedExactUnmarshal(&uconf)
	require.NoError(t, err, "failed to unmarshal")
	require.Equal(t, "expected_value", uconf.Inner.Single)
}

func TestStringFromFile(t *testing.T) {
	t.Parallel()
	file, err := os.CreateTemp(t.TempDir(), "test")
	require.NoError(t, err, "failed to create temp file")

	expectedValue := "this is the text in the file"

	err = os.WriteFile(file.Name(), []byte(expectedValue), 0o644)
	require.NoError(t, err, "uname to write temp file")

	yaml := fmt.Sprintf("---\nInner:\n  Single:\n    File: %s", file.Name())

	config := New()
	err = config.ReadConfig(strings.NewReader(yaml))
	require.NoError(t, err, "error reading config")

	var uconf stringFromFileConfig
	err = config.EnhancedExactUnmarshal(&uconf)
	require.NoError(t, err, "unmarshal failed")
	require.Equal(t, expectedValue, uconf.Inner.Single)
}

func TestPEMBlocksFromFile(t *testing.T) {
	t.Parallel()
	file, err := os.CreateTemp(t.TempDir(), "test")
	require.NoError(t, err, "failed to create temp file")

	var pems []byte
	for range 3 {
		publicKeyCert, _, _ := generateMockPublicPrivateKeyPairPEM(true)
		pems = append(pems, publicKeyCert...)
	}

	err = os.WriteFile(file.Name(), pems, 0o644)
	require.NoError(t, err, "failed to write temp file")

	yaml := fmt.Sprintf("---\nInner:\n  Multiple:\n    File: %s", file.Name())

	config := New()
	err = config.ReadConfig(strings.NewReader(yaml))
	require.NoError(t, err, "error reading config")

	var uconf stringFromFileConfig
	err = config.EnhancedExactUnmarshal(&uconf)
	require.NoError(t, err, "failed to unmarshal")
	require.Len(t, uconf.Inner.Multiple, 3)
}

func TestPEMBlocksFromFileEnv(t *testing.T) {
	file, err := os.CreateTemp(t.TempDir(), "test")
	require.NoError(t, err, "failed to create temp file")

	var pems []byte
	for range 3 {
		publicKeyCert, _, _ := generateMockPublicPrivateKeyPairPEM(true)
		pems = append(pems, publicKeyCert...)
	}

	err = os.WriteFile(file.Name(), pems, 0o644)
	require.NoError(t, err, "failed to write temp file")

	envVar := testEnvPrefix + "_INNER_MULTIPLE_FILE"
	t.Setenv(envVar, file.Name())

	testCases := []struct {
		name string
		data string
	}{
		{"Override", "---\nInner:\n  Multiple:\n    File: wrong_file"},
		{"NoFileElement", "---\nInner:\n  Multiple:\n"},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			config := New()
			config.SetConfigName(testConfigName)

			err := config.ReadConfig(strings.NewReader(tc.data))
			require.NoError(t, err, "error reading config")

			var uconf stringFromFileConfig
			err = config.EnhancedExactUnmarshal(&uconf)
			require.NoError(t, err, "failed to unmarshal")
			require.Len(t, uconf.Inner.Multiple, 3)
		})
	}
}

func TestStringFromFileNotSpecified(t *testing.T) {
	t.Parallel()
	yaml := "---\nInner:\n  Single:\n    File:\n"

	config := New()
	err := config.ReadConfig(strings.NewReader(yaml))
	require.NoError(t, err, "error reading config")

	var uconf stringFromFileConfig
	err = config.EnhancedExactUnmarshal(&uconf)
	require.Error(t, err, "umarshal should fail")
}

func TestStringFromFileEnv(t *testing.T) {
	expectedValue := "this is the text in the file"

	file, err := os.CreateTemp(t.TempDir(), "test")
	require.NoError(t, err, "failed to create temp file")

	err = os.WriteFile(file.Name(), []byte(expectedValue), 0o644)
	require.NoError(t, err, "failed to write temp file")

	envVar := testEnvPrefix + "_INNER_SINGLE_FILE"
	t.Setenv(envVar, file.Name())

	testCases := []struct {
		name string
		data string
	}{
		{"Override", "---\nInner:\n  Single:\n    File: wrong_file"},
		{"NoFileElement", "---\nInner:\n  Single:\n"},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			config := New()
			config.SetConfigName(testConfigName)

			err := config.ReadConfig(strings.NewReader(tc.data))
			require.NoError(t, err, "error reading config")

			var uconf stringFromFileConfig
			err = config.EnhancedExactUnmarshal(&uconf)
			require.NoError(t, err, "failed to unmarshal")
			require.Exactly(t, expectedValue, uconf.Inner.Single)
		})
	}
}

func TestDecodeOpaqueField(t *testing.T) {
	t.Parallel()
	yaml := "---\nFoo: bar\nHello:\n  World: 42\n"

	config := New()
	err := config.ReadConfig(strings.NewReader(yaml))
	require.NoError(t, err, "error reading config")

	var conf struct {
		Foo   string
		Hello struct{ World int }
	}
	err = config.EnhancedExactUnmarshal(&conf)
	require.NoError(t, err, "failed to unmarshal")
	require.Equal(t, "bar", conf.Foo)
	require.Equal(t, 42, conf.Hello.World)
}

func TestBCCSPDecodeHookOverride(t *testing.T) {
	yaml := "---\nBCCSP:\n  Default: default-provider\n  SW:\n    Security: 999\n"

	overrideVar := testEnvPrefix + "_BCCSP_SW_SECURITY"
	t.Setenv(overrideVar, "1111")

	config := New()
	config.SetConfigName(testConfigName)
	err := config.ReadConfig(strings.NewReader(yaml))
	require.NoError(t, err, "error reading config")

	var tc struct {
		BCCSP *factory.FactoryOpts
	}
	err = config.EnhancedExactUnmarshal(&tc)
	require.NoError(t, err, "failed to unmarshal")
	require.NotNil(t, tc.BCCSP)
	require.NotNil(t, tc.BCCSP.SW)
	require.Equal(t, 1111, tc.BCCSP.SW.Security)
}

func TestDurationDecode(t *testing.T) {
	t.Parallel()
	tests := []struct {
		input    string
		expected time.Duration
	}{
		{"", 0},
		{"100", 100 * time.Nanosecond},
		{"1s", time.Second},
		{"1m", time.Minute},
		{"1m1s", 61 * time.Second},
		{"90s", 90 * time.Second},
	}
	for _, tt := range tests {
		t.Run(tt.expected.String(), func(t *testing.T) {
			t.Parallel()
			yaml := fmt.Sprintf("---\nDuration: %s\n", tt.input)

			config := New()
			config.SetConfigName(testConfigName)
			err := config.ReadConfig(strings.NewReader(yaml))
			require.NoError(t, err, "error reading config")

			var conf struct{ Duration time.Duration }
			err = config.EnhancedExactUnmarshal(&conf)
			require.NoError(t, err, "failed to unmarshal")
			require.Equal(t, tt.expected, conf.Duration)
		})
	}
}

func TestOrdererEndpointDecoder(t *testing.T) {
	t.Parallel()
	expected := &types.OrdererEndpoint{
		ID:    5,
		MspID: "org",
		API:   []string{"broadcast", "deliver"},
		Host:  "localhost",
		Port:  5050,
	}
	tests := []struct {
		input    string
		expected *types.OrdererEndpoint
	}{
		{"", nil},
		{"Endpoint: ", nil},
		{"Endpoint: localhost:5050", &types.OrdererEndpoint{ID: types.NoID, Host: expected.Host, Port: expected.Port}},
		{"Endpoint: id=5,msp-id=org,broadcast,deliver,localhost:5050", expected},
		{`Endpoint: {"id":5,"msp-id":"org","api":["broadcast","deliver"],"host":"localhost","port":5050}`, expected},
		{`
Endpoint:
  id: 5
  msp-id: org
  api:
    - broadcast
    - deliver
  host: localhost
  port: 5050
`, expected},
	}
	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			t.Parallel()
			yaml := fmt.Sprintf("---\n%s\n", tt.input)
			t.Log(yaml)

			config := New()
			config.SetConfigName(testConfigName)
			err := config.ReadConfig(strings.NewReader(yaml))
			require.NoError(t, err, "error reading config")

			var conf struct{ Endpoint *types.OrdererEndpoint }
			err = config.EnhancedExactUnmarshal(&conf)
			require.NoError(t, err, "failed to unmarshal")
			require.Equal(t, tt.expected, conf.Endpoint)
		})
	}
}
