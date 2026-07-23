---
name: doc-audit
description: >-
  Audit the repository's documentation, agent instructions, skills, and config samples for
  staleness introduced by a code change — every *.md and *.yaml/*.yml file. Use AFTER finishing
  any development task that touched Go code, protobuf/ASN.1, config fields, CLI flags, make
  targets, scripts, or file/package layout, and whenever the user asks to "check the docs",
  "audit the docs", or "did I make anything obsolete?". It regenerates the one derived artifact
  this repo has (protobuf) to catch drift, then reports likely-stale prose, agent instructions,
  skills, and config samples with file:line and a suggested edit. Runs well on a dispatched
  subagent to keep the parent context clean. It only audits EXISTING files for staleness a code
  change introduced — do not use it to author new documentation. For writing new Go code use the
  `development` skill; for tests use `tests`; for PR review use `pr-review`.
---

# Auditing docs, config, and agent instructions after a change

A development task can quietly invalidate documentation, agent-instruction files, other skills, or
config samples: a renamed config field, a moved package, a removed CLI flag, a changed make target,
a renamed helper the skills cite. This skill audits the `*.md` / `*.yaml` corpus against the change
that just landed.

**Unlike some sibling repos, this project has almost no generated docs** — no metrics/CLI/sample
generators. The one derived artifact is the checked-in **protobuf** (`*.pb.go`), so Tier A is thin.
The real value is Tier B: the semantic sweep that nothing checks — including the **skills
themselves**, which cite exact paths, symbols, helper names, and linters and therefore rot when
code moves.

## Run this on a subagent

Prefer dispatching this skill to a subagent: the corpus grep and any `make proto` output would
otherwise flood the parent context. The subagent's **only** deliverable is the report in the
[Report](#report) format below — grounded in the diff, not conversation history.

## Step 1 — Establish the change set

Everything is keyed off the actual diff. Do not audit from memory.

```bash
base=$(git merge-base main HEAD)
git diff --stat "$base"   # files touched since branching from main, incl. uncommitted work
git diff "$base"          # the full diff
```

When you are on `main`, `base` equals `HEAD`, so it degrades to the uncommitted diff — the correct
fallback.

From the diff, extract the change vocabulary you will grep for in Step 3:

- **Proto** — added/renamed/removed messages, fields, enum values, services in `api/*/*.proto`.
- **ASN.1** — changes to `api/applicationpb/asn1_tx_schema.asn` or the `Tx`/`TxNamespace` proto.
- **Config fields** — added/renamed/removed keys in `configtx.yaml` structs (`tools/configtxgen/config.go`) or policies/orderer params.
- **CLI flags / commands** — changes under `cmd/`.
- **Symbols** — renamed/removed exported and unexported funcs, types, methods, consts.
- **Paths** — moved/renamed/deleted files and packages.
- **Make targets / scripts** — changes to `Makefile` or `scripts/`.

## Step 2 — Tier A: mechanical checks

Run the ones relevant to the diff.

```bash
# If any .proto changed: regenerate and check for drift the developer forgot.
make proto && git diff --stat -- 'api/**/*.pb.go' 'api/**/*_grpc.pb.go'

# Always cheap to run and report-only:
make lint-proto     # api-linter over api/**/*.proto
make lint-asn1      # asn1c over api/**/*.asn
scripts/license-lint.sh   # license headers (also part of `make lint`)
```

- **`make proto` produces a diff** → the checked-in generated code was stale. Regenerate is the fix
  (the command already did it); record the regenerated files under **Auto-fixed**.
- **`make lint-proto` / `make lint-asn1` fail** → a proto/schema bug (e.g. the `Tx` change was not
  mirrored in `asn1_tx_schema.asn`). That is a code bug for the developer — put it under **Must-fix**,
  don't try to "regenerate" a schema.
- There are **no generated documentation files** in this repo (no metrics/CLI/sample-tree docs), so
  there is nothing else to auto-fix. Everything below is report-only.

## Step 3 — Tier B: semantic sweep (report, do not edit)

The judgment pass over the corpus that no tool checks: prose docs, agent-instruction files, the
skills (which cite exact paths/symbols), and config samples.

For each changed artifact from Step 1, grep the corpus and judge each hit against this map.
**Ground every finding in the diff — no speculative findings.**

| Change in code | Files that can go stale |
|---|---|
| Proto message / field / enum | `docs/configtx.md` (config protos), `README.md` (provenance), proto comments, the `development` skill's `references/proto-asn1-and-config.md` |
| ASN.1 / `Tx` change | the `development` skill's `references/proto-asn1-and-config.md`, `docs/configtx.md` if it describes tx encoding, `asn1.go` comments |
| Config field / policy / orderer param | `sampleconfig/configtx.yaml`, `sampleconfig/core.yaml`, `docs/configtx.md` (policy-path tables + ASCII diagrams + usage examples) |
| CLI flag / command (`cmd/`) | `README.md`, `docs/configtx.md` usage examples, `AGENTS.md` |
| Package / file / symbol / helper name | `docs/*.md`, `AGENTS.md` (the canonical agent guide; other agent-instruction files may symlink to it, so edit it once), and **every skill's `SKILL.md` + its `references/`** (their reuse tables, fixture lists, file maps, and linter cheat sheet cite exact paths/symbols), `README.md` |
| Make target / script / dev workflow | `AGENTS.md`, `README.md`, any skill that names the command (`development`, `tests`, `pr-review`, `doc-audit` all cite `make` targets) |
| Linter added/removed/reconfigured (`.golangci.yml`) | the `development` skill's linter cheat sheet, `pr-review`'s compliance checklist |

Grep the whole corpus, not just `docs/` — and explicitly include the skills:

```bash
git grep -n "<old-name>" -- '*.md' '*.yaml' '*.yml'
```

For each hit ask: does the change make this reference **false, obsolete, or incomplete** (a renamed
config key missing from the sample, a moved package still cited in a skill's reuse table, a deleted
helper named in the `tests` skill, a removed make target in `AGENTS.md`)? If yes, it is a finding.

Note: `AGENTS.md` is the single canonical agent guide — other agent-instruction files may be
symlinks to it, so a fix there covers all of them. Config `*.yaml` under `*/testdata/` are test
fixtures, not docs — only flag them if the change specifically renames something they encode.

## Report

Return one Markdown report, these four sections, in this order. Omit a section only if it is empty
(but always emit **Clean** when there are no findings at all).

```markdown
## doc-audit report

### Auto-fixed
- `api/committerpb/status.pb.go` — regenerated via `make proto` (enum added in status.proto but generated code was stale).

### Must-fix
- `api/applicationpb/asn1_tx_schema.asn` — `make lint-asn1` fails / the `Tx` change was not mirrored here and in `asn1.go:translate`. Update the schema and the translate method.

### Review
- `docs/configtx.md:88` — references policy path `/Channel/Application/OldName`, renamed to `NewName`. Suggested edit: rename in the policy-path table and the two ASCII diagrams.
- the `development` skill's `SKILL.md:64` — reuse table cites `common/util.OldHelper`, renamed to `NewHelper`. Suggested edit: update the table row.

### Clean
- No stale references found for: <list the changed artifacts you checked and cleared>.
```

## Guardrails

- **Never edit** prose, agent-instruction, or skill files. Tier A only regenerates *generated code*
  (`make proto`); everything else is a suggested edit in the report.
- **Diff-grounded only.** Every finding must trace to a specific change in Step 1's diff. Do not
  report pre-existing doc issues unrelated to this change.
- **State uncertainty.** If you cannot confirm a reference is stale, mark it "⚠️ possible" in
  **Review** rather than asserting it.
- **No silent scope cuts.** If you skip part of the corpus (e.g. `make proto` could not run because
  the proto toolchain is missing), say so explicitly in the report.
