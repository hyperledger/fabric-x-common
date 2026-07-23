---
name: tests
description: Write and run unit tests in the Fabric-X Common library — standard Go testing with testify, table-driven where it fits, reusing the repo's crypto/config-block/proto fixtures. Use whenever you add or change tests, or need to run the suite for packages you touched. New tests use standard `testing` + `testify`; do NOT add new Ginkgo suites (they are legacy). For authoring the production code under test use the `development` skill.
---

# Testing in Fabric-X Common

Guidelines for writing and running tests here. The bar: meaningful coverage of behavior and
edge cases, minimal mocking, and reuse of the repo's fixtures instead of hand-rolled setup.

## Framework: standard testing + testify (Ginkgo is legacy)

- **New tests use the standard `testing` package with `testify`** (`require` / `assert`). This is
  the overwhelming majority of the suite.
- **Do not write new Ginkgo/Gomega suites.** A handful of packages still use Ginkgo
  (`common/deliver`, `common/deliverclient/blocksprovider`, `common/deliverclient/orderers`,
  `common/grpcmetrics`, `tools/configtxgen`) — that is legacy ported code. When adding tests to
  those packages, prefer a standard `Test*` function in a new `_test.go` file rather than
  extending the spec suite; the `ginkgolinter` only guards the existing specs.
- Prefer **`require`** over `assert` for anything that should stop the test on failure
  (`testifylint` enforces good testify usage). Never call `panic()` in tests — use
  `require.NoError(t, err)`.

## Running tests

```bash
make test                                  # full suite: gotestsum, -race, 30m timeout
make test-cover && make cover-report       # coverage profile + HTML
go test ./common/configtx/ -run TestName -v # a single package / test during development
go test ./common/configtx/... -race         # a subtree with the race detector
```

`make test` is the source of truth (it adds `-race`). Run the narrow `go test` form while
iterating, then the package subtree before you finish.

## Reuse fixtures & helpers — don't hand-roll

Crypto, TLS, config blocks, and proto comparison already have helpers. Re-inventing them is a
common review rejection.

| Need | Use | Package |
|------|-----|---------|
| Proto equality / element-match assertions | `test.RequireProtoEqual`, `test.RequireProtoElementsMatch` | `utils/test` |
| Config-block + crypto fixtures | `testcrypto.CreateOrExtendConfigBlockWithCrypto`, `ConfigBlock`, `PrepareBlockHeaderAndMetadata` | `utils/testcrypto` |
| Signing identities / MSP dirs | `testcrypto.GetSigningIdentities` / `GetPeersIdentities` / `GetConsenterIdentities` / `GetPeersMspDirs` / `GetConsenterMspDirs` | `utils/testcrypto` |
| Test TLS CA & cert/key pairs | `tlsgen.NewCA()`, `CA`, `CertKeyPair` | `common/crypto/tlsgen` |
| Generate crypto material | `cryptogen` | `tools/cryptogen` |
| Static fixtures (certs, blocks, configs) | `testdata/` beside the package | e.g. `common/channelconfig/testdata`, `msp/testdata`, `protoutil/testdata` |

**Never compare protobuf messages with `require.Equal`** — protobuf internals make that unreliable.
Use `test.RequireProtoEqual` / `RequireProtoElementsMatch`. Both accept `require.TestingT`, so they
also work inside `require.EventuallyWithT`.

## Mocks

Mocks are generated, not hand-written:

- **counterfeiter** via `//go:generate counterfeiter -o mock/... --fake-name X . iface` directives,
  regenerated with **`make mocks`** (`go generate ./...`). Fakes live in `mock/` or `mocks/`
  subpackages — don't edit them by hand.
- **mockery** is also installed (by `scripts/install-dev-dependencies.sh`) for packages that use it.
- Prefer testing against **real dependencies** (the fixtures above) over mocks where practical;
  reach for a generated fake only for a genuine seam.

## Table-driven tests

Use a flat, inline table with `tc` as the case variable. Do **not** nest under
`t.Run("success cases", …)` / `t.Run("failure cases", …)` wrappers — keep one loop per shape and
separate success vs failure loops with a comment when their fields differ.

```go
func TestFmtNamespace(t *testing.T) {
	t.Parallel()
	// success cases
	for _, tc := range []struct {
		name     string
		input    string
		expected string
	}{
		{name: "simple namespace", input: "ns1", expected: "ns_ns1"},
		{name: "empty namespace", input: "", expected: "ns_"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			require.Equal(t, tc.expected, FmtNamespace(tc.input))
		})
	}
	// failure cases
	for _, tc := range []struct {
		name  string
		input string
	}{
		{name: "invalid namespace panics", input: "\x00"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			require.Panics(t, func() { FmtNamespace(tc.input) })
		})
	}
}
```

Rules the linters enforce, so write to them up front:

- **`t.Parallel()`** in the top-level test and in every subtest (`paralleltest`).
- **`t.Helper()`** in every helper function (`thelper`); put helpers at the **end** of the file.
- **`require.ErrorContains(t, err, "...")`** instead of `require.Error` followed by
  `require.Contains`.
- For panics, use **`require.Panics`** — avoid `require.PanicsWithValue`/`PanicsWithError`, because
  `cockroachdb/errors` wraps values with stack traces and exact matching is unreliable.
- Use `require.Eventually` / `require.EventuallyWithT` for async conditions — **never `time.Sleep`**.

## Coverage & scope

- Cover meaningful scenarios and edge/error cases, not just the happy path — but don't chase a
  coverage percentage with vacuous tests.
- Round-trip tests matter for the encoders here: proto ⇄ ASN.1 (`api/applicationpb/asn1_test.go`),
  JSON ⇄ proto (`protolator`), and config block encode/inspect. `protolator/` is excluded from
  golangci-lint but must stay covered by tests.
- When you change a `.proto` or the `Tx`/ASN.1 schema, add a test that exercises the new
  field/encoding (see the `development` skill's `references/proto-asn1-and-config.md`).

## Before you finish

- Run the touched packages with `-race`; run `make lint` (tests are linted too — `paralleltest`,
  `thelper`, `testifylint`, `usetesting`, `ginkgolinter`).
- Rely on CI for the full `make test` if the suite is large, but make sure your package passes
  locally first.
