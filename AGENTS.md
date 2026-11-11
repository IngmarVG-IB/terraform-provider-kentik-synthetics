# AGENTS.md

Guide for maintainers and coding agents working on the Kentik synthetics Terraform provider
(`registry.terraform.io/kentik/kentik-synthetics`).

## Purpose

Terraform provider for [Kentik synthetic monitoring](https://kb.kentik.com/v4/Ma00.htm): synthetic tests,
agents and agent alerts. It calls the Kentik v6 gRPC API `kentik.synthetics.v202309` through the generated Go
clients of [`github.com/kentik/api-schema-public`](https://github.com/kentik/api-schema-public) (pinned in
`go.mod` and in `kentik-api.json`). Built on terraform-plugin-sdk/v2 (plugin protocol 5).

## Layout

| Path | What it is |
|------|------------|
| `main.go` | Plugin entry point; `go:generate` runs tfplugindocs. |
| `internal/kentikapi/` | Terraform-agnostic gRPC adapter: endpoint/region, TLS, `X-CH-Auth-*` per-RPC credentials, timeout, retry. Typed service accessors in `services.go`. |
| `internal/fakesynthetics/` | In-memory fake `SyntheticsAdminService` over gRPC bufconn, used by every unit test. Seed data with `AddAgent`/`AddTest`/`AddAgentAlert`, inject errors with `FailNext`, inspect `Calls()`. |
| `synthetics/provider.go` | Provider schema and configuration (`New(version, opts...)`). |
| `synthetics/schema.go` | Schema builder: one definition rendered for resources (`create`) and data sources (`readSingle`, `readList`); descriptions come from the proto annotations (`protodoc.go`). |
| `synthetics/protoconv.go` | Generic proto <-> Terraform conversion via protobuf reflection (attribute name = proto field name). |
| `synthetics/legacy.go` | v202101beta1 attribute aliases (write fallback, read preservation, data source fill, state upgrade). |
| `synthetics/test_schema.go`, `agent_schema.go` | Schemas for tests and agents. |
| `synthetics/resource_*.go`, `data_source_*.go` | Resources and data sources (`*_impl.go` names avoid the `_test.go` suffix). |
| `synthetics/test_state_upgrade.go` | `kentik-synthetics_test` schema v0 -> v1 upgrader and the frozen v0 schema. |
| `synthetics/*_test.go` | Unit tests (package `synthetics_test`, against the fake); `*_internal_test.go` test unexported helpers; `acceptance_test.go` is gated by `TF_ACC`. |
| `synthetics/testdata/` | Fixtures, e.g. a verbatim 0.2.x configuration that must keep applying cleanly. |
| `examples/` | `provider/`, `resources/<type>/`, `data-sources/<type>/` (embedded in docs by tfplugindocs), `test-types/<type>/main.tf` (one per test type, embedded via `templates/resources/test.md.tmpl` and applied by the unit tests), use-case examples. `provider.tf` files are symlinks to `examples/common/provider.tf`. |
| `templates/` | tfplugindocs templates. |
| `docs/` | **Generated** registry docs (`make docs`), except `docs/UPGRADING.md` and `docs/adr/`. |
| `kentik-api.json` | API coverage manifest (read by the kentikcommunity indexer); validated by `manifest_internal_test.go`. |
| `.github/workflows/` | `ci.yml` (lint, Go x Terraform/OpenTofu test matrix, vuln, generate drift, manifest) and `release.yml`. |

## Commands

| Task | Command |
|------|---------|
| Build | `make build` |
| Unit tests (offline) | `make test` |
| One test | `make test TESTARGS='-run TestResourceTest_UpdateAndStatus -v'` |
| Acceptance tests (live tenant, creates resources) | `TF_ACC=1 KTAPI_AUTH_EMAIL=... KTAPI_AUTH_TOKEN=... KTAPI_ACC_AGENT_ID=... make testacc` |
| Lint | `make lint` (golangci-lint v2, pinned in the Makefile, built with your Go) |
| Format | `make fmt` |
| Vulnerability scan | `make vuln` (govulncheck) |
| Regenerate docs | `make docs` (alias `make generate`) |
| Check drift | `make check-generate`, `make check-go-mod` |
| Validate examples | `make validate-examples` (dev_overrides, no API calls) |
| Validate `kentik-api.json` | `make check-manifest` |

Unit tests drive the real Terraform CLI through terraform-plugin-testing with the provider served
in-process; they need `terraform` (or `tofu` via `TF_ACC_TERRAFORM_PATH`) on `PATH`, but no credentials and
no network access to Kentik.

## Adding an attribute, an API service or an API version

New field in an existing message:
1. Bump api-schema-public: `go get github.com/kentik/api-schema-public@master && go mod tidy`; update
   `schema_source.ref` in `kentik-api.json` (the manifest test checks it against `go.mod`).
2. Add one line to the schema in `synthetics/*_schema.go` with the proto field name. The description comes
   from the proto; the conversion is automatic. Choose `required`, `optional`, `optionalComputed` (server
   fills a default) or `computedOnly` (output only).
3. Add the field to an example or a unit test assertion, run `make test docs`, add a CHANGELOG line.

New service (for example `kentik.label.v202210`):
1. Add an accessor to `internal/kentikapi/services.go` returning the generated client for `c.conn`.
2. Add a fake in `internal/fake<service>/` following `fakesynthetics` (embed `Unimplemented...Server`, keep
   state in maps, record calls, check both auth headers, `Start(t)` with bufconn); register it on the same
   bufconn server if tests need several services.
3. Add the client to `apiClient` in `synthetics/provider.go`, then resources/data sources with schemas built
   by `newBuilder(mode, md(&pb.Message{}))`, tests, examples and docs.
4. Add the service to `kentik-api.json` and to the CHANGELOG.

New version of a service (for example synthetics v2027xx):
1. Switch the import path, compile, and diff the protos (`proto/kentik/synthetics/<old>` vs `<new>`).
2. Renamed or removed fields: keep the old attribute as deprecated, add an alias in `synthetics/legacy.go`,
   bump `SchemaVersion` with a `StateUpgraders` entry (freeze the previous schema like `testResourceV0`)
   and a unit test, and update `docs/UPGRADING.md`.
3. Update `kentik-api.json`, examples, docs, CHANGELOG. Prefer stable versions; label alpha/beta experimental.

## Conventions

- Go: `go 1.26` minimum; CI runs Go 1.26 and 1.27. Format with gofumpt + goimports (`make fmt`).
- Terraform type names (`kentik-synthetics_*`) and attribute names are public API. Do not rename or remove
  them; deprecate instead and provide a state upgrader when stored state changes shape.
- Every attribute has a `Description` (a unit test enforces it); docs come from it.
- Use the generated api-schema-public types; do not hand-write API models.
- Record significant design decisions in `docs/adr/NNNN-title.md`.
- Every user-visible change goes into `CHANGELOG.md` (Keep a Changelog, `Unreleased`).

## Do and don't

- Do keep `make test` offline: unit tests use `internal/fakesynthetics`, never a real tenant.
- Do gate live-tenant tests behind `TF_ACC=1` and name them `TestAcc*`.
- Don't commit credentials, tenant IDs, real agent IDs or captured production payloads.
- Don't read `.env` files or look for Kentik credentials on the machine.
- Don't `terraform apply` against a real API while developing; use `make validate-examples` or a
  `dev_overrides` CLI config with `terraform validate`/`plan`.
- Don't edit generated files under `docs/` by hand; change schema descriptions, examples or templates.
- Don't use code or knowledge from non-public Kentik repositories.

## Release process

1. Move the `Unreleased` entries in `CHANGELOG.md` under a new version heading.
2. Tag `vX.Y.Z` on the default branch and push the tag. `release.yml` builds, signs (secrets
   `GPG_PRIVATE_KEY`, `PASSPHRASE`) and creates a **draft** GitHub release with GoReleaser.
3. Review the draft and publish it; the Terraform registry picks it up from there.
