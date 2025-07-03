/*
Copyright IBM Corp. All Rights Reserved.

SPDX-License-Identifier: Apache-2.0
*/

package orderer

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"
)

func TestReadWrite(t *testing.T) {
	t.Parallel()
	valSchema := "id=5,msp-id=org,broadcast,deliver,localhost:5050"
	valJSON := `{"id":5,"msp-id":"org","api":["broadcast","deliver"],"host":"localhost","port":5050}`
	valYAML := `id: 5
msp-id: org
api:
    - broadcast
    - deliver
host: localhost
port: 5050
`
	expected := &Endpoint{
		ID:    5,
		MspID: "org",
		API:   []string{"broadcast", "deliver"},
		Host:  "localhost",
		Port:  5050,
	}
	require.Equal(t, valSchema, expected.String())

	valJSONRaw, err := json.Marshal(expected)
	require.NoError(t, err)
	require.JSONEq(t, valJSON, string(valJSONRaw))

	valYamlRaw, err := yaml.Marshal(expected)
	require.NoError(t, err)
	require.YAMLEq(t, valYAML, string(valYamlRaw))

	e, err := ParseOrdererEndpoint(valSchema)
	require.NoError(t, err)
	require.Equal(t, expected, e)

	e, err = ParseOrdererEndpoint(valJSON)
	require.NoError(t, err)
	require.Equal(t, expected, e)

	e, err = ParseOrdererEndpoint(valYAML)
	require.NoError(t, err)
	require.Equal(t, expected, e)

	valJSONNoID := `{"msp-id":"org","api":["broadcast","deliver"],"host":"localhost","port":5050}`
	e, err = ParseOrdererEndpoint(valJSONNoID)
	require.NoError(t, err)
	require.Equal(t, &Endpoint{
		ID:    NoID,
		MspID: "org",
		API:   []string{"broadcast", "deliver"},
		Host:  "localhost",
		Port:  5050,
	}, e)
}
