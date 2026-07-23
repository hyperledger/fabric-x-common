This file provides guidance to agents when working with code in this repository.

## Project Overview

**Fabric-X Common** is a Go **library** of shared code for the Fabric-X ecosystem. Most of
it originates from [Hyperledger Fabric](https://github.com/hyperledger/fabric) v3.0.0-rc1 and
[fabric-config](https://github.com/hyperledger/fabric-config) v0.3.0, then modified for
Fabric-X. When editing ported code, preserve upstream structure and idioms — divergence from
Fabric should be intentional, not incidental. Known Fabric-X divergences from upstream config:
the `ARMA` orderer type and the `MetaNamespacePolicyKey` config field.

There are no long-running services here: the artifacts are reusable packages plus three CLI
tools (`configtxgen`, `configtxlator`, `cryptogen`).

### Key Technologies

- **Language**: Go 1.26
- **Serialization**: Protocol Buffers (gRPC where applicable) + a canonical **ASN.1**
  transaction encoding
- **Config/identity core**: configtx, channelconfig, policies, MSP
- **Testing**: standard Go testing with `testify`; a few legacy `ginkgo` suites (do not add new ones)
- **Build**: Make-based

## Building, testing, and linting

Requires the Go toolchain (`go 1.26`) plus dev tools from `scripts/install-dev-dependencies.sh`
(protoc, asn1c, golangci-lint, mockery, and the `tool` directives in `go.mod`). Run `make help`
for the documented target list.

```bash
make test         # full suite via gotestsum with -race
make test-cover   # coverage profile; make cover-report for HTML
make lint         # lint-proto (api-linter) + lint-asn1 (asn1c) + golangci-lint + license headers
make tools        # build configtxgen, configtxlator, cryptogen into bin/
make proto        # regenerate *.pb.go / *_grpc.pb.go from api/*/*.proto (clones fabric-protos)
make mocks        # regenerate counterfeiter fakes via go generate ./...
```

Note: `make lint` runs `golangci-lint run --new-from-rev=main`, so it only flags changes newer
than `main`. Run a single test directly with Go: `go test ./common/configtx/ -run TestName -v`.

## Development Conventions

Follow the **`development` skill** whenever you write or change Go code — it is the
authoritative, detailed source for these conventions. In brief:

- **Match the surrounding Fabric-ported code.** This is a fork; new code should look like it
  belongs. Divergence from upstream must be intentional.
- **Apache-2.0 license header** on every source file (enforced by the `goheader` linter and
  `scripts/license-lint.sh`).
- **Error handling** — `errors.New/Newf/Wrap/Wrapf` (`github.com/cockroachdb/errors`) at the
  origin; `fmt.Errorf("...%w", err)` to add context. `github.com/pkg/errors` is **banned** by
  the `depguard` linter for new code (legacy ported files still use it).
- **Logging** — one package-level `flogging.MustGetLogger(...)`; printf-style `Debugf/Infof/Warnf/Errorf`.
- **Reuse `protoutil`, `common/…`, and `utils/` helpers** before writing new plumbing.
- The Go linter set is strict (see `.golangci.yml`): `gosec`, `gocognit`≤15, `ireturn`,
  `paralleltest`, `dupl`, `lll` (120), `revive` (argument-limit 4), and more. Run `make lint`.

For tests use the **`tests` skill**; for reviewing a PR use **`pr-review`**; for writing a
commit/PR message or filing an issue use **`commit-and-issue`**; after a change, audit docs with
the **`doc-audit`** skill.

## Project Structure

```
/
├── api/          # Protobuf/gRPC definitions (.proto) + generated Go + hand-written helpers
│   ├── applicationpb/   # block transactions (+ ASN.1 deterministic tx encoding)
│   ├── committerpb/     # block query / notify / snapshot / status
│   ├── msppb/           # MSP messages
│   ├── ordererpb/       # ARMA consensus + party config
│   └── types/           # shared API types (e.g. orderer endpoints)
├── common/       # building blocks ported from Fabric
│   ├── configtx/        # parse/validate/update config transactions
│   ├── channelconfig/   # the Bundle of channel config resources
│   ├── policies/        # implicit-meta, signature, BFT policies
│   ├── capabilities/ genesis/ cauthdsl/ policydsl/ ledger/ deliver/
│   ├── crypto/          # crypto helpers incl. tlsgen (test CAs)
│   └── metadata/        # version/commit vars injected at build time
├── core/         # aclmgmt, config, policy
├── msp/          # Membership Service Provider (identity/crypto)
├── protolator/   # reflective JSON <-> protobuf translation (engine behind configtxlator)
├── protoutil/    # block, tx, proposal, signed-data helpers
├── cmd/          # thin CLI main.go entry points -> delegate to tools/
├── tools/        # CLI implementations (configtxgen, configtxlator, cryptogen)
├── utils/        # testcrypto, test (proto require helpers), certificate
├── sampleconfig/ # sample configtx.yaml, core.yaml, crypto material, embedded config
├── docs/         # docs/configtx.md — the config/genesis reference
└── scripts/      # dev-dependency install, license lint, coverage filter
```

## Key Documentation

- `docs/configtx.md` — deep reference on the configuration/genesis block, `configtx.yaml`
  sections, ARMA consensus, and orderer endpoints. **Read it before touching config or genesis
  code.**
- `README.md` — the Fabric/fabric-config provenance and which packages were ported.

## Important Notes for Agents

1. **Generated protobuf is checked in.** Edit `.proto`, then run `make proto` — never hand-edit
   `*.pb.go` / `*_grpc.pb.go`. Proto changes must pass `make lint-proto` (see `.apilinter.yaml`
   for disabled rules).
2. **ASN.1 mirrors the Tx proto.** Any change to the `Tx` message in `api/applicationpb` **must**
   be mirrored in `api/applicationpb/asn1_tx_schema.asn`, which `make lint-asn1` validates.
3. **`cmd/` is thin.** Put logic in `tools/`; `cmd/*/main.go` only wires flags and delegates.
4. **`protolator/` is excluded from `golangci-lint`** (heavy reflection/ported code) — but still
   keep it correct and tested.
5. **Mocks are generated.** `//go:generate counterfeiter ...` directives + `make mocks`; don't
   edit fakes by hand.
6. **This library is consumed by other Fabric-X repos** (committer, orderer). Treat exported API
   and proto changes as cross-repo contracts; cross-repo issues are referenced by full URL.

## Pre-Pull Request Checklist

- [ ] Code matches surrounding (ported) patterns; divergence is intentional and explained
- [ ] License header present on new files
- [ ] `make lint` passes (proto + asn1 + golangci + license)
- [ ] Relevant tests added/updated (see `tests` skill) and passing locally
- [ ] `make proto` re-run if `.proto` changed; ASN.1 schema updated if `Tx` changed
- [ ] Docs audited for staleness (see `doc-audit` skill); `docs/configtx.md` updated if config changed
- [ ] Commit/PR message follows the `commit-and-issue` skill (it becomes the PR description verbatim)
