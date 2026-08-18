/*
Copyright IBM Corp. All Rights Reserved.

SPDX-License-Identifier: Apache-2.0
*/

package helpers_test

import (
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/hyperledger/fabric-x-common/tools/fxadmin/core/helpers"
)

// TestRequireDistinctOutput covers the output-collision guard: distinct inputs
// pass, an output naming any input is rejected, and the comparison resolves
// paths so a differently-spelled path to the same file is still caught.
func TestRequireDistinctOutput(t *testing.T) {
	t.Parallel()

	const modified = "modified.json"

	for _, tc := range []struct {
		name    string
		output  string
		inputs  []string
		wantErr string
	}{
		{
			name:   "all distinct",
			output: "out.pb",
			inputs: []string{"current.json", modified, "block.pb"},
		},
		{
			name:   "no inputs",
			output: "out.pb",
		},
		{
			name:    "output equals an input",
			output:  modified,
			inputs:  []string{"current.json", modified},
			wantErr: "must be a different file from input",
		},
		{
			name:    "output resolves to an input by a different path",
			output:  filepath.Join(".", modified),
			inputs:  []string{modified},
			wantErr: "must be a different file from input",
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			err := helpers.RequireDistinctOutput(tc.output, tc.inputs...)
			if tc.wantErr == "" {
				require.NoError(t, err)
				return
			}
			require.ErrorContains(t, err, tc.wantErr)
		})
	}
}
