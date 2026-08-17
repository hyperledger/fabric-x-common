/*
Copyright IBM Corp. All Rights Reserved.

SPDX-License-Identifier: Apache-2.0
*/

// Package utils holds small helpers shared across the fxadmin commands.
package utils //nolint:revive

import (
	"path/filepath"

	"github.com/cockroachdb/errors"
)

// RequireDistinctOutput reports an error if outputPath resolves to the same
// file as any of inputPaths, so writing the output never silently overwrites
// one of the command's own inputs. Paths are compared after resolving to
// absolute form, so distinct strings that name the same file are still caught.
func RequireDistinctOutput(outputPath string, inputPaths ...string) error {
	outputAbs, err := filepath.Abs(outputPath)
	if err != nil {
		return errors.Wrapf(err, "failed to resolve output path %q", outputPath)
	}
	for _, inputPath := range inputPaths {
		inputAbs, err := filepath.Abs(inputPath)
		if err != nil {
			return errors.Wrapf(err, "failed to resolve input path %q", inputPath)
		}
		if inputAbs == outputAbs {
			return errors.Newf("output %q must be a different file from input %q", outputPath, inputPath)
		}
	}
	return nil
}
