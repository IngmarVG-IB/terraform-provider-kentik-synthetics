# Changelog

All notable changes to this project are documented in this file.
The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/), and this project adheres to
[Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

### Security

- Raised the Go toolchain to `go 1.26` and updated every dependency to its current release, which fixes all
  known vulnerabilities reported by `govulncheck` (before: 14 reachable vulnerabilities in 4 modules, including
  `golang.org/x/crypto`, `golang.org/x/net`, `google.golang.org/grpc` and `google.golang.org/protobuf`).
- CI now runs with a read-only `GITHUB_TOKEN`, every GitHub Action is pinned to a full commit SHA, and the
  release job is the only one with `contents: write`.

### Changed

- `terraform-plugin-sdk/v2` v2.20.0 to v2.40.1, `terraform-plugin-log` v0.7.0 to v0.11.0,
  `terraform-plugin-docs` v0.13.0 to v0.25.0.
- Build tools are tracked with `tool` directives in `go.mod` instead of `tools/tools.go`.
- golangci-lint configuration migrated to v2.
- GoReleaser configuration migrated to v2, with a `terraform-registry-manifest.json` and draft releases.
- Makefile targets: `build`, `test`, `testacc`, `lint`, `fmt`, `vuln`, `docs`, `generate`, `check-generate`,
  `check-go-mod`, `check-manifest`.
- Dependabot runs weekly with grouped updates.

### Added

- `AGENTS.md` (maintainer and agent guide), `CLAUDE.md`, `SECURITY.md`, this changelog and the
  `kentik-api.json` API coverage manifest.

## [0.2.1] - 2021-12-08

Last upstream release. See the [GitHub releases](https://github.com/kentik/terraform-provider-kentik-synthetics/releases).

[Unreleased]: https://github.com/kentik/terraform-provider-kentik-synthetics/compare/v0.2.1...HEAD
[0.2.1]: https://github.com/kentik/terraform-provider-kentik-synthetics/releases/tag/v0.2.1
