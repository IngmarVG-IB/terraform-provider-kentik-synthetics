# Changelog

All notable changes to this project are documented in this file.
The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/), and this project adheres to
[Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

Moves the provider from synthetics API v202101beta1 to the stable v202309. Existing configurations keep
working with deprecation warnings; see [docs/UPGRADING.md](docs/UPGRADING.md).

### Changed (API)

- The provider now calls the Kentik v6 gRPC API `kentik.synthetics.v202309` through the generated clients of
  `github.com/kentik/api-schema-public`, wrapped by the new `internal/kentikapi` adapter (TLS, per-RPC
  `X-CH-Auth-Email`/`X-CH-Auth-API-Token`, timeout, retry with backoff). `community_sdk_golang` is no longer
  used.
- `kentik-synthetics_test` has schema version 1. A state upgrader converts 0.2.x state automatically.
- `kentik-synthetics_test.status` is optional (default `TEST_STATUS_ACTIVE`), accepts `active`/`paused`,
  and is applied with SetTestStatus.
- `kentik-synthetics_test.type` is validated and forces replacement; exactly the matching `settings` block
  must be set (checked at plan time).
- Settings that the API fills with defaults are Optional+Computed, so they no longer cause diffs.
- The `id` of `kentik-synthetics_agents` and `kentik-synthetics_tests` is a constant.
- Empty lists and maps are reported as null.
- Retry defaults: 5 attempts, 1s initial and 30s maximum delay (was 100 attempts, up to 5m).

### Added

- Test settings from v202309: `labels`, `notes`, `throughput`, `schedule`, `alerting` (including grouped
  alerting), `health_settings.activation`, `health_settings.disabled_metrics`, DNS, throughput and certificate
  thresholds, `health_disabled`, `disable_alerts`, `ping.dscp`, `ping.timeout`, `trace.dscp`, `trace.mtu`,
  `trace.timeout`, `agent.use_local_ip`, `agent.reciprocal`, `ip.use_local_ip`, `network_grid.use_local_ip`,
  `flow.max_providers`, `flow.max_ip_targets`, `url.*` and `page_load.*` request options, `dns.record_type`,
  `dns.servers`, `dns.port`, `settings.notification_channels`.
- Test type `network_mesh`.
- Resource `kentik-synthetics_agent`: adopts an existing agent (by `agent_id` or import) and manages
  `alias`, `site_id`, `site_name`, `labels`, `status` (authorize with `AGENT_STATUS_OK`), `family` and
  location. Agents cannot be created through the API; destroy only forgets the agent unless
  `delete_on_destroy = true`.
- Resource `kentik-synthetics_agent_alert` (agent offline alerts; CRUD and import).
- Data sources `kentik-synthetics_agent_alert` and `kentik-synthetics_agent_alerts`.
- Import of `kentik-synthetics_test` by ID.
- Agent attributes `site_name`, `labels`, `cloud_region`, `cloud_provider`, `metadata`.
- Provider attributes `endpoint` (`KTAPI_ENDPOINT`), `region` (`KTAPI_REGION`, `US`/`EU`) and `timeout`
  (`KTAPI_TIMEOUT`).
- Offline unit tests for every resource and data source against an in-memory gRPC fake
  (`internal/fakesynthetics`), including a verbatim 0.2.x configuration; acceptance tests gated by `TF_ACC`.
- One example per test type in `examples/test-types/`, included in the docs; `make validate-examples`.
- `docs/UPGRADING.md` and ADRs in `docs/adr/` (0003, terraform-plugin-framework migration, is a proposal).

### Deprecated

- Provider `api_url` / `KTAPI_URL` (use `endpoint` or `region`).
- Test attributes without a v202309 equivalent or with a new name (`settings.protocol`, `settings.port`,
  `settings.servers`, `settings.count`, `settings.expiry`, `settings.limit`, `settings.rollup_level`,
  `settings.monitoring_settings`, `settings.http`, `settings.site`, `settings.tag`, `ping.period`,
  `ping.expiry`, `trace.period`, `trace.expiry`, `dns.type`, `dns_grid.type`, `dns_grid.targets`,
  `flow.max_tasks`, `expires_on`); mapped or ignored as documented in the upgrade guide.
- Agent attributes `name` (use `site_name`) and `cloud_vpc`.

### Removed

- The REST test server and the scripts that started it (`tools/`), `synthetics/test-data.json` and the
  `run.sh` example scripts.

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
