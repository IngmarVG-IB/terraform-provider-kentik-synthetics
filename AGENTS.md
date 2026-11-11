# AGENTS.md

Guide for maintainers and coding agents working on the Kentik synthetics Terraform provider
(`registry.terraform.io/kentik/kentik-synthetics`).

## Purpose

Terraform provider for [Kentik synthetic monitoring](https://kb.kentik.com/v4/Ma00.htm): synthetic tests,
agents. The provider currently uses `github.com/kentik/community_sdk_golang` (synthetics v202101beta1, REST).

## Layout

| Path | What it is |
|------|------------|
| `main.go` | Plugin entry point; `go:generate` runs tfplugindocs. |
| `synthetics/` | Provider, resources, data sources, schema and conversions. |
| `examples/` | Terraform examples; tfplugindocs embeds `examples/{resources,data-sources}/<type>/*.tf` into `docs/`. |
| `templates/` | tfplugindocs templates (`index.md.tmpl`). |
| `docs/` | **Generated** registry docs (`make docs`), except `docs/UPGRADING.md` and `docs/adr/`. Do not hand-edit the generated files. |
| `docs/adr/` | Architecture decision records. |
| `kentik-api.json` | Machine-readable API coverage manifest (read by the kentikcommunity indexer). |
| `terraform-registry-manifest.json` | Plugin protocol version for the Terraform registry. |
| `.github/workflows/` | `ci.yml` (lint, test matrix, vuln, generate drift, manifest) and `release.yml` (tag-triggered GoReleaser). |

## Commands

| Task | Command |
|------|---------|
| Build | `make build` |
| Unit tests (offline) | `make test` |
| One test | `make test TESTARGS='-run TestResourceTest -v'` |
| Acceptance tests (live tenant) | `TF_ACC=1 KTAPI_AUTH_EMAIL=... KTAPI_AUTH_TOKEN=... make testacc` |
| Lint | `make lint` (golangci-lint v2, pinned in the Makefile, built with your Go) |
| Format | `make fmt` |
| Vulnerability scan | `make vuln` (govulncheck) |
| Regenerate docs | `make docs` (alias: `make generate`) |
| Check generated drift | `make check-generate`, `make check-go-mod` |
| Validate `kentik-api.json` | `make check-manifest` |

Unit tests use Terraform's test harness and need a `terraform` (or `tofu`, via `TF_ACC_TERRAFORM_PATH`) binary on
`PATH`. They never need credentials or network access to Kentik.

## Conventions

- Go: `go 1.26` minimum; CI runs Go 1.26 and 1.27. Format with gofumpt + goimports (`make fmt`).
- Terraform type names (`kentik-synthetics_*`) and attribute names are public API. Do not rename or remove them.
  Deprecate instead (`Deprecated:` in the schema), and when stored state changes shape bump `SchemaVersion` and
  add a `StateUpgraders` entry with a unit test. Document every user-visible change in `CHANGELOG.md`
  (Keep a Changelog, `Unreleased` section) and, for breaking or migrating changes, in `docs/UPGRADING.md`.
- Every attribute needs a `Description` (it becomes the registry docs).
- Use the generated `api-schema-public` types. Do not hand-write API models.
- Record significant design decisions as a short ADR in `docs/adr/NNNN-title.md`.

## Do and don't

- Do keep `make test` offline. Tests run against local fakes; never against a real Kentik tenant.
- Do gate tests that need a live tenant behind `TF_ACC=1` and name them `TestAcc*`.
- Don't commit credentials, tenant IDs, real agent IDs or captured production payloads.
- Don't read `.env` files or look for Kentik credentials on the machine.
- Don't call `terraform apply` against a real API while developing; use `terraform validate` with a
  `dev_overrides` CLI config (see below).
- Don't edit generated files under `docs/` by hand; change schema descriptions, examples or templates instead.

## Local development with Terraform

```sh
make build
cat > /tmp/dev.tfrc <<EOF
provider_installation {
  dev_overrides { "kentik/kentik-synthetics" = "$(pwd)" }
  direct {}
}
EOF
TF_CLI_CONFIG_FILE=/tmp/dev.tfrc terraform -chdir=examples/resources/kentik-synthetics_test validate
```

## Release process

1. Move the `Unreleased` entries in `CHANGELOG.md` under a new version heading.
2. Tag `vX.Y.Z` on the default branch and push the tag. `release.yml` builds, signs (GPG secrets
   `GPG_PRIVATE_KEY`, `PASSPHRASE`) and creates a **draft** GitHub release with GoReleaser.
3. Review the draft and publish it; the Terraform registry picks it up from there.
