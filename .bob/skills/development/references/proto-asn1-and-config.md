# Protobuf + ASN.1 APIs and the configtx → genesis pipeline

Read this when adding or changing a structured API in this repo. Two flows are unique here and
easy to get wrong; both are gated by `make lint` / `make proto`. Cite the model files rather than
guessing.

## Table of contents

1. [Editing a protobuf API](#1-editing-a-protobuf-api)
2. [Keeping the ASN.1 transaction schema in sync](#2-keeping-the-asn1-transaction-schema-in-sync)
3. [The configtx → genesis pipeline](#3-the-configtx--genesis-pipeline)
4. [JSON ⇄ protobuf (protolator / configtxlator)](#4-json--protobuf)
5. [Checklists](#5-checklists)

---

## 1. Editing a protobuf API

The `.proto` sources live under `api/*/` (e.g. `api/committerpb/status.proto`,
`api/ordererpb/configuration.proto`). Generated Go (`*.pb.go`, `*_grpc.pb.go`) is **checked in**
next to the source, and hand-written helpers (`types.go`, `asn1.go`, `identity.go`) live in the
**same package**.

Workflow:

1. Edit the `.proto`. Keep `option go_package = "github.com/hyperledger/fabric-x-common/api/<pkg>";`
   and `package <pkg>;` consistent with the directory.
2. Regenerate: **`make proto`**. This clones `fabric-protos` (pinned to the
   `fabric-protos-go-apiv2` version in `go.mod`) into `.build/`, then runs `protoc` with the
   `--go_out` / `--go-grpc_out` `paths=source_relative` mapping. Never hand-edit the generated
   files.
3. Lint: **`make lint-proto`** runs `api-linter` with `.apilinter.yaml`. Several Google AIP rules
   are intentionally disabled there (versioned packages, method-signature, HTTP annotations, etc.);
   if the linter complains about a rule you believe should be off, change `.apilinter.yaml`, don't
   work around it in the proto.
4. Add hand-written helpers in the same package (e.g. `committerpb/types.go` exposes
   `SystemNamespaces()` / `IsSystemNamespace()` alongside the generated messages).

**Backward compatibility is a cross-repo contract** — this library's protos are consumed by the
committer and orderer. Add new fields with the next free tag; never renumber, retype, or remove a
field without `reserved`. New enum values go at the end (see how `status.proto` appends
`MALFORMED_*` codes without renumbering).

## 2. Keeping the ASN.1 transaction schema in sync

Transactions have a **canonical ASN.1 encoding** used for deterministic signing/hashing, defined
in `api/applicationpb/asn1_tx_schema.asn` and implemented in `api/applicationpb/asn1.go`
(`ASN1Marshal`, `translate`).

**The hard rule** (stated in both files): **any change to the `Tx` / `TxNamespace` proto message
must be mirrored in `asn1_tx_schema.asn` and in the `translate` method in `asn1.go`.** The schema
is validated by **`make lint-asn1`** (which runs `asn1c` over `api/**/*.asn`).

- ASN.1 encodes a nil version as `-1` (`protoToAsnVersion` / `asnToProtoVersion`) — preserve that
  convention when adding version-bearing fields.
- Because this encoding feeds signatures and block-data hashes, a mismatch between the proto and
  the schema is a correctness bug, not a lint nit. Add a round-trip test (`asn1_test.go`).

## 3. The configtx → genesis pipeline

The genesis/config block is produced from `configtx.yaml` by `configtxgen`. Read
**`docs/configtx.md`** first — it is the authoritative reference for the block structure, the
`configtx.yaml` sections, ARMA consensus, and orderer endpoints.

The pipeline and where each piece lives:

```
configtx.yaml ──▶ configtxgen ──▶ ConfigGroup tree ──▶ genesis block
 (sampleconfig/    (tools/configtxgen)                  (common/genesis,
  configtx.yaml)                                          protoutil)
```

- **Parsing**: `tools/configtxgen/config.go` — `LoadTopLevel(...)` / `Load(profile, paths...)`
  parse `configtx.yaml` into `*TopLevel` / `*Profile` structs (viper-based).
- **Encoding**: `tools/configtxgen/encoder.go` — `NewChannelGroup`, `NewOrdererGroup`,
  `NewApplicationGroup`, `DefaultConfigTemplate` build the `common.ConfigGroup` tree with its
  policies.
- **Emitting**: `tools/configtxgen/tools.go` — `GetOutputBlock` / `DoOutputBlock` wrap the group
  into a genesis block; `DoInspectBlock` prints one back.
- **CLI**: `cmd/configtxgen/main.go` only wires flags and delegates to `tools/configtxgen`. Put
  logic in `tools/`, not `cmd/`.

`configtx.yaml` sections (see `sampleconfig/configtx.yaml`): `Organizations`, `Capabilities`,
`Application`, `Orderer` (this is where the Fabric-X **ARMA** orderer type lives), `Channel`,
`Profiles`.

**When you add a config field, policy, or orderer parameter, keep the whole chain in sync:** the
struct in `tools/configtxgen/config.go`, the encoder in `encoder.go`, the sample in
`sampleconfig/configtx.yaml`, and the prose in `docs/configtx.md`. A new application policy, for
example, is added to the encoder, the sample profile, and the policy-path tables/diagrams in
`docs/configtx.md` (see how `SnapshotEndorsement` / `CheckpointEndorsement` were added). Add a test
that the policy resolves via `bundle.PolicyManager().GetPolicy(...)`.

Fabric-X divergences from upstream config to preserve: the `ARMA` orderer type and the
`MetaNamespacePolicyKey` field.

## 4. JSON ⇄ protobuf

`protolator` (engine behind `configtxlator`) does reflective JSON ⇄ protobuf translation for
config messages, including opaque/dynamic/variably-opaque fields. When you add a config proto that
must round-trip through JSON (e.g. for `configtxlator`), check whether it needs a `protoext`
handler registered. `protolator/` is **excluded from golangci-lint** but must stay correct and
tested (`*_test.go` beside each source).

## 5. Checklists

**Changing a `.proto`:**
1. Edit `api/<pkg>/*.proto` (new fields at next free tag; enums appended; no renumber/remove).
2. `make proto` → regenerate checked-in `*.pb.go` / `*_grpc.pb.go`.
3. `make lint-proto` → api-linter passes (adjust `.apilinter.yaml`, not the proto, for rule policy).
4. Add/adjust hand-written helpers in the same package; add tests.
5. If you touched the `Tx` message → do the ASN.1 checklist below.

**Changing the `Tx` message:**
1. Update `api/applicationpb/asn1_tx_schema.asn`.
2. Update `translate` (and version helpers if needed) in `api/applicationpb/asn1.go`.
3. `make lint-asn1` passes; add a round-trip test in `asn1_test.go`.

**Adding a config path / policy / orderer field:**
1. Struct in `tools/configtxgen/config.go`; encoder in `encoder.go`.
2. Sample in `sampleconfig/configtx.yaml`.
3. Prose + policy-path tables/diagrams in `docs/configtx.md`.
4. Test that it resolves (config parses; policy resolvable via the `Bundle`).
