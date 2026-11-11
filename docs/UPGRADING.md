# Upgrading from 0.2.x to 0.3.0

Version 0.3.0 moves the provider from the beta synthetics API **v202101beta1** (REST, through
`community_sdk_golang`) to the stable **v202309** gRPC API from
[`api-schema-public`](https://github.com/kentik/api-schema-public). Resource and data source type names are
unchanged, and configurations written for 0.2.x keep working: attributes that no longer exist in the API
are still accepted, marked deprecated, and either mapped to their replacement or ignored.

## Steps

1. Raise the version constraint, for example `version = "~> 0.3"`, and run `terraform init -upgrade`.
2. Run `terraform plan`. Existing state is upgraded automatically (schema version 0 to 1, see below). The
   plan should show no changes for tests whose settings are representable in v202309; deprecation warnings
   point at the attributes to move.
3. Replace deprecated attributes with their replacements from the table below, one resource at a time, and
   confirm with `terraform plan` that nothing changes.

## Provider configuration

| 0.2.x | 0.3.0 |
|-------|-------|
| `api_url` / `KTAPI_URL` (REST URL) | Deprecated. Use `endpoint` / `KTAPI_ENDPOINT` (`host:port`, gRPC) or `region = "EU"`. Known Kentik hostnames in `api_url` (`synthetics.api.kentik.com`, `api.kentik.eu`, ...) are mapped to the gRPC endpoint of the same cluster. |
| `email`, `token` | Unchanged. |
| `retry { max_attempts, min_delay, max_delay }` | Unchanged names. Defaults are now 5 attempts, 1s, 30s (was 100, 1s, 5m). Only `UNAVAILABLE` and `RESOURCE_EXHAUSTED` are retried. |
| `log_payloads` | Unchanged; logs gRPC payloads at debug level. |
| (new) | `timeout` / `KTAPI_TIMEOUT`: per-request timeout, default `60s`. |

## kentik-synthetics_test

### State upgrade

State written by 0.2.x (schema version 0) is upgraded on the first plan:

- values of legacy attributes are copied to their replacements (table below) when the replacement is empty,
- `ping.count`, `trace.count`, `trace.port` and `trace.limit` change from float to integer and are rounded,
- an empty `status` becomes `TEST_STATUS_ACTIVE`.

Legacy values stay in state, so a configuration that still uses them does not show a diff.

### Attribute mapping

Paths are relative to the resource; `settings.ping` means the `ping` block inside `settings`.

| 0.2.x attribute (deprecated) | 0.3.0 replacement | Notes |
|------------------------------|-------------------|-------|
| `settings.monitoring_settings.notification_channels` | `settings.notification_channels` | |
| `settings.protocol` | `settings.ping.protocol` | |
| `settings.port` | `settings.ping.port` | |
| `settings.servers` | `settings.dns.servers` / `settings.dns_grid.servers` | |
| `settings.ping.expiry` | `settings.ping.timeout` | milliseconds |
| `settings.trace.expiry` | `settings.trace.timeout` | milliseconds |
| `settings.http.method`, `.body` | `settings.url.method`, `.body` | |
| `settings.http.headers`, `.ignore_tls_errors` | `settings.url.*` / `settings.page_load.*` | |
| `settings.http.css_selectors` | `settings.page_load.css_selectors` | |
| `settings.http.expiry` | `settings.url.timeout` / `settings.page_load.timeout` | |
| `settings.dns.type` | `settings.dns.record_type` | |
| `settings.dns_grid.type` | `settings.dns_grid.record_type` | |
| `settings.dns_grid.targets` (list) | `settings.dns_grid.target` (string) | first element is used |
| `settings.flow.max_tasks` | `settings.flow.max_ip_targets` | |
| `settings.ping.period`, `settings.trace.period`, `settings.http.period` | `settings.period` | ignored |
| `settings.count`, `settings.expiry`, `settings.limit` | `ping.*` / `trace.*` | ignored |
| `settings.rollup_level` | (none) | ignored |
| `settings.site`, `settings.tag` | (none) | `site` and `tag` tests do not exist in v202309 |
| `expires_on` | (none) | always empty |

A mapped legacy attribute is only used when its replacement is not set in the configuration.

### Behaviour changes

- `status` is optional (default `TEST_STATUS_ACTIVE`) and also accepts `active` / `paused`. Changes are
  applied with the SetTestStatus API call; the test is not re-created.
- `type` now forces a new test when changed (the API never allowed changing it) and is validated:
  `hostname`, `ip`, `network_grid`, `agent`, `network_mesh`, `flow`, `url`, `page_load`, `dns`, `dns_grid`.
  The `settings` block for that type must be set, and no other one. Tests of type `site`, `tag` or
  `application_mesh` cannot be managed with v202309.
- `settings.family`, `settings.protocol`, `settings.rollup_level`, `settings.trace` and
  `settings.trace.period` / `settings.ping.period` are no longer required.
- New attributes: `labels`, `settings.notes`, `settings.network_mesh`, `settings.throughput`,
  `settings.schedule`, `settings.alerting`, `settings.health_settings.activation`, `disabled_metrics`,
  DNS, throughput and certificate thresholds, `ping.dscp`, `trace.dscp`, `trace.mtu`, `agent.reciprocal`,
  `use_local_ip`, `flow.max_providers`, `url.*` and `page_load.*` request options, `dns.port`.
- Tests can be imported: `terraform import kentik-synthetics_test.example <id>`.
- Most optional settings are now also computed: values filled in by the API are kept in state instead of
  causing a diff. Removing such an attribute from the configuration keeps the current value; set it
  explicitly to change it.

## Data sources

- `kentik-synthetics_agent` / `_agents`: `name` is deprecated in favour of `site_name` (same value).
  `cloud_vpc` is deprecated and always empty. New: `site_name`, `labels`, `cloud_region`,
  `cloud_provider`, `metadata`.
- `kentik-synthetics_test` / `_tests`: same schema as the resource. Deprecated attributes are filled from
  their replacements so existing references keep working.
- The `id` of `kentik-synthetics_agents` and `kentik-synthetics_tests` is now a constant instead of the
  current Unix time.
- Empty lists and maps are reported as null instead of empty.
