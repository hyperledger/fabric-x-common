---
name: development
description: >-
  Conventions for writing NEW Go code in the Fabric-X Common library: matching the
  Fabric-ported style, license headers, the strict golangci-lint set, error handling
  (cockroachdb/errors), logging (flogging), Go 1.26 idioms, and reuse of protoutil /
  common / utils helpers. Use this skill BEFORE writing or modifying any Go source in
  this repository — adding or changing a package, function, proto/ASN.1 API, or config
  path — so new code matches established patterns and passes `make lint`. For test-only
  work use the `tests` skill; for reviewing a PR use `pr-review`; after a change audit
  docs with `doc-audit`.
---

# Developing in Fabric-X Common

This skill tells you how to write Go that looks like it already belonged in this library.
Follow it whenever you add or change production Go source.

**This is a fork.** Most code originates from [Hyperledger Fabric](https://github.com/hyperledger/fabric)
v3.0.0-rc1 and [fabric-config](https://github.com/hyperledger/fabric-config) v0.3.0. The prime
directive is **match the surrounding code** — new code in a ported package should be
indistinguishable from Fabric's. Divergence from upstream must be intentional and worth it,
because it makes future re-syncs with Fabric harder.

**Scope split** — write tests using the `tests` skill. Review PRs with `pr-review`. This skill
covers *authoring* production code.

**Authoritative sources in the repo** (read them when a topic needs depth):
- `.golangci.yml` — every linter that gates your PR (cheat sheet at the end).
- `.apilinter.yaml` — the api-linter rules for `.proto` files (disabled rules listed there).
- `docs/configtx.md` — the config/genesis-block reference; read before touching config code.
- `README.md` — which packages were ported from Fabric and what was modified.
- `references/proto-asn1-and-config.md` (in this skill) — the two structured-authoring flows
  that are unique and error-prone here: editing a **protobuf/ASN.1 API**, and the
  **configtx → genesis** pipeline.

## The prime directive: match the ported code

Before writing, read the neighbouring files in the package you're touching.

- **Follow the file's existing style** — header format, import grouping, naming, how errors are
  wrapped, whether it uses `flogging`. A package ported from Fabric keeps Fabric's idioms.
- **Prefer reuse over reinvention.** This repo already carries block/tx/proposal/crypto/config
  plumbing (see the reuse table below). Re-implementing it is the most common review rejection.
- **Keep functions simple.** `gocognit` caps complexity at 15 and `maintidx` flags unmaintainable
  functions; use guard clauses and early returns. When a function genuinely can't split, justify
  with a scoped `//nolint:gocognit // <reason>`.
- **No premature interfaces.** The `ireturn` linter blocks returning interfaces; return concrete
  types. Interfaces are for real pluggable seams (and the codebase has many ported ones — extend
  those in place rather than inventing parallel abstractions).

## Step 0 — reuse before you write

Grep these packages before implementing anything infrastructural:

| Need | Use | Package |
|------|-----|---------|
| Block / header / metadata helpers | `BlockDataHash`, `BlockHeaderBytes`, `GetMetadataFromBlock`, `IsConfigBlock`, … | `protoutil` |
| Tx / envelope / proposal helpers | `CreateSignedEnvelope`, `ExtractEnvelope`, `GetPayloads`, `UnmarshalX`, … | `protoutil` |
| Config-tx parse / update / validate | `configtx.*` | `common/configtx` |
| Channel config resources (`Bundle`) | `channelconfig.*` | `common/channelconfig` |
| Policies (implicit-meta, signature, BFT) | `policies.*`, `policydsl.*`, `cauthdsl.*` | `common/policies`, `common/policydsl`, `common/cauthdsl` |
| Genesis block factory | `genesis.*` | `common/genesis` |
| Hashing / bytes / UUID / timestamp | `ComputeSHA256`, `ConcatenateBytes`, `GenerateUUID`, `CreateUtcTimestamp` | `common/util` |
| JSON ⇄ protobuf translation | `protolator.*` | `protolator` (excluded from lint — keep it correct anyway) |
| MSP / identity | `msp.*` | `msp` |
| Config-block encoding from a profile | `encoder.NewChannelGroup`, `encoder.DefaultConfigTemplate` | `tools/configtxgen` |
| Create / originate / wrap errors | `errors.New/Newf/Wrap/Wrapf` | `github.com/cockroachdb/errors` |
| Package logger | `flogging.MustGetLogger` | `github.com/hyperledger/fabric-lib-go/common/flogging` |

**Test fixtures** — don't hand-roll crypto, TLS, or config-block fixtures; see the `tests`
skill. Key helpers, all native to this repo: `testcrypto.CreateOrExtendConfigBlockWithCrypto` /
`GetSigningIdentities` (`utils/testcrypto`), `test.RequireProtoEqual` /
`RequireProtoElementsMatch` (`utils/test`), `tlsgen.NewCA()` (`common/crypto/tlsgen`),
`cryptogen` (`tools/cryptogen`).

## Code organization

- **File header** — every new `.go` file starts with the Apache-2.0 block comment the
  `goheader` linter requires (exact text, no year):
  ```go
  /*
  Copyright IBM Corp. All Rights Reserved.

  SPDX-License-Identifier: Apache-2.0
  */
  ```
  (Older ported files carry Fabric's dated header, e.g. `Copyright IBM Corp. 2017 …`; leave those
  as-is — `make lint` only checks changes newer than `main`. New files use the template above.)
- **Imports** — three blank-line-separated groups: stdlib, third-party, then internal
  `github.com/hyperledger/fabric-x-common/...`. `goimports` with `local-prefixes` enforces this;
  `make lint` fixes most.
- **Hand-written helpers live beside their generated proto.** In `api/<pkg>/`, generated
  `*.pb.go` / `*_grpc.pb.go` sit next to hand-written `types.go` / `asn1.go` / `identity.go` in
  the same package. Add new proto helpers there, not in a separate package.
- **Doc comments end with a period** (`godot`), and exported sentinel errors are named `ErrXxx`
  (`errname`).

## Error handling

Uses `github.com/cockroachdb/errors`. **`github.com/pkg/errors` is banned by `depguard`** for new
code — 82 legacy ported files still import it, but do not add new ones.

1. **Originate or first cross into our code** (a new condition, or an error from a driver /
   `Unmarshal` / gRPC) → `errors.New` / `errors.Newf` / `errors.Wrap` / `errors.Wrapf`. This
   captures the stack trace at the origin.
2. **Add context while propagating** an error that already carries a trace →
   `fmt.Errorf("context: %w", err)`. Always `%w`, never `%v`/`%s` (the `errorlint` linter enforces
   `errorf`).
3. **Compare sentinels with `errors.Is`**, never `==`.

## Logging

One unexported package-level logger via Fabric's zap-based `flogging`; no `New()`, no per-struct
logger:

```go
var logger = flogging.MustGetLogger("common.configtx")
```

Use printf-style `Debugf` / `Infof` / `Warnf` / `Errorf`. Log level is controlled at runtime by
the `FABRIC_LOGGING_SPEC` environment variable (keep it quiet in test/dev runs to reduce noise).
Don't `Fatal`/`Panic` in library code — return an error.

## Concurrency

Unlike some sibling Fabric-X repos, **this library is largely synchronous** — only one package
uses `errgroup`, and there is no repo-specific channel-wrapper doctrine. If you genuinely need
concurrency, use the standard toolkit (`errgroup.WithContext`, `context` cancellation,
`sync`/`atomic`) and pass the group context into goroutines. Don't import a concurrency framework
from another repo; keep it minimal and idiomatic. `containedctx`/`fatcontext` forbid storing a
`context.Context` in a struct.

## Modern Go idioms (1.26)

Prefer in new code (several are linter-enforced):

- `slices.*` / `maps.*` — `Contains`, `Index`, `SortFunc`, `Sorted`, `Collect`, `maps.Clone`.
- Built-in `min` / `max`.
- Range-over-integer: `for range n` / `for i := range n` (the `intrange` linter pushes this).
- `any`, never `interface{}` (only generated `.pb.go` uses the latter).
- `errors.Join` to combine causes.

## Building or changing a structured API

The two authoring flows that are unique here — and easy to get wrong — have their own reference:
read **`references/proto-asn1-and-config.md`** before:

- editing any `.proto` (regeneration, `go_package`, api-linter, checked-in generated code), or
  changing the `Tx` message (the **ASN.1 schema must be kept in sync**), or
- adding/changing a config path, policy, or orderer field that flows through `configtx.yaml` →
  `configtxgen` → the genesis block.

## Before you finish

Before you consider the change done (see `AGENTS.md` and the `Makefile`):

- `make lint` — must pass. It runs `golangci-lint run --new-from-rev=main` **plus** `lint-proto`
  (api-linter), `lint-asn1` (asn1c), and the license-header check. Note the `--new-from-rev=main`:
  only your changes are linted, so write to the rules up front.
- Run the relevant tests for what you touched (`go test ./common/configtx/... -run TestXxx`), and
  add tests per the `tests` skill. `make test` runs the full suite with `-race` via gotestsum.
- If you changed a `.proto`, run `make proto`. If you changed the `Tx` message, update
  `api/applicationpb/asn1_tx_schema.asn`.
- If you changed a `//go:generate counterfeiter` interface, run `make mocks`.
- **Audit docs & config for staleness.** Dispatch a subagent to run the `doc-audit` skill (keeps
  this context clean); apply the **Review** suggestions it returns.

**Enforced-linter cheat sheet** (from `.golangci.yml`) — write to these up front:

- `goheader` — Apache-2.0 `/* */` header (exact template) on every new file.
- `gofumpt` + `goimports` — formatting and 3-group import ordering (local-prefix
  `github.com/hyperledger/fabric-x-common`); `make lint` fixes most.
- `lll` / `revive line-length-limit` — 120-column lines.
- `intrange` — use `for range n`. `ireturn` — don't return interfaces. `errname` — `ErrXxx`.
- `errorlint` — `fmt.Errorf` must use `%w`. `depguard` — no `github.com/pkg/errors`.
- `gocognit` ≤ 15, `maintidx` (under 20), `dupl` — keep functions simple and non-duplicated
  (or justify with a scoped `//nolint:<linter> // reason`).
- `revive argument-limit` 4, `function-result-limit` 3 — use a param/return struct beyond these.
- `godot` — doc comments end with a period. `containedctx`/`fatcontext` — no `context.Context` in
  a struct.
- `gosec` (with G204/G404/G306 excluded), `prealloc`, `unparam`, `wastedassign`, `unconvert`,
  `forcetypeassert`, `nilerr`, `misspell`, `paralleltest`, `thelper`, `testifylint`,
  `ginkgolinter`, `usetesting` — the usual correctness/test/efficiency nits.
- `protolator/` is **excluded** from golangci-lint (heavy reflection/ported code) — but still keep
  it correct and covered by tests.

`//nolint` must be specific (`//nolint:gocognit // <reason>`) — the `nolintlint` linter requires
the linter name, and the codebase expects a reason.
