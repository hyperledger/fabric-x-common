/*
Copyright IBM Corp. All Rights Reserved.

SPDX-License-Identifier: Apache-2.0
*/

package cli

import (
	"reflect"
	"slices"
	"strings"
	"time"

	"github.com/cockroachdb/errors"
)

// The CLI is responsible for parsing and validating command-line arguments,
// then delegates the selected command to the corresponding handler. Each
// command family has its own small interface.

// LedgerHandler executes the `fxadmin ledger` subcommands.
type LedgerHandler interface {
	Height(configPath, currentBlockPath string) error
	Block(configPath, currentBlockPath, reference, outputPath string) error
	Config(configPath, currentBlockPath, reference, outputPath string) error
}

// DecodeHandler executes `fxadmin decode`.
type DecodeHandler interface {
	Run(blockPath, outputPath string) error
}

// UpdateHandler executes `fxadmin compute-update`.
type UpdateHandler interface {
	Run(currentPath, modifiedPath, currentBlockPath, outputPath string) error
}

// TxHandler executes the `fxadmin tx` subcommands.
type TxHandler interface {
	Endorse(inputPath, configPath, outputPath string) error
	Merge(inputPaths []string, outputPath string) error
	Prepare(inputPath, configPath, outputPath string) error
	Submit(inputPath, configPath, currentBlockPath string) error
	Send(inputPath, configPath, currentBlockPath, outputPath string) error
}

// FollowHandler executes `fxadmin follow`.
type FollowHandler interface {
	Run(configPath, currentBlockPath string, timeout time.Duration) error
}

// Handlers groups the per-command handlers the CLI dispatches to. It is
// assembled by the caller (main) from the per-command core packages.
type Handlers struct {
	Ledger LedgerHandler
	Decode DecodeHandler
	Update UpdateHandler
	Tx     TxHandler
	Follow FollowHandler
}

// validate reports any command handler that was left nil, so a misconfigured
// Handlers fails with a clear error.
func (h Handlers) validate() error {
	var missing []string
	for name, set := range map[string]bool{
		"ledger":         isHandlerNil(h.Ledger),
		"decode":         isHandlerNil(h.Decode),
		"compute-update": isHandlerNil(h.Update),
		"tx":             isHandlerNil(h.Tx),
		"follow":         isHandlerNil(h.Follow),
	} {
		if !set {
			missing = append(missing, name)
		}
	}
	if len(missing) > 0 {
		slices.Sort(missing)
		return errors.Newf("missing command handlers: %s", strings.Join(missing, ", "))
	}
	return nil
}

// isHandlerNil reports whether a handler interface value is usable. A plain
// h != nil check misses a typed nil, e.g. a nil *T that satisfies the
// interface: the interface is non-nil but calling through it panics. Treat
// such a value, and a pointer/interface that wraps a nil, as missing.
func isHandlerNil(h any) bool {
	if h == nil {
		return false
	}
	v := reflect.ValueOf(h)
	switch v.Kind() {
	case reflect.Pointer, reflect.Interface:
		return !v.IsNil()
	default:
		return true
	}
}
