# 0002. Reflection-based proto/Terraform conversion and legacy attribute compatibility

- Status: Accepted
- Date: 2026-11-10

## Context

The v202309 test model has roughly 150 fields across nested messages, a oneof for the test definition, enums,
maps, timestamps and float32 thresholds. The 0.2.x provider converted every field by hand (about 750 lines),
which made adding fields error-prone. At the same time, 0.2.x configurations and state use attribute names
from v202101beta1 that no longer exist (for example `settings.protocol`, `ping.expiry`, `dns_grid.targets`),
some of which were required.

## Decision

- Terraform attribute names equal proto field names (snake_case). `synthetics/protoconv.go` converts in both
  directions with protobuf reflection: enums as value names, timestamps as RFC 3339, float32 through its
  shortest decimal representation (so `0.1` stays `0.1`), messages as single-element blocks, one-field
  wrapper messages (`AgentMetadata.IpValue`) as string lists, empty collections as null.
- Schemas stay hand-written (explicit Required/Optional/Computed choices), but attribute descriptions are
  read from the `openapiv2_field` annotations in the protos. A unit test fails if an attribute has no
  description, and schema construction panics if an attribute has no proto field, which keeps the schema
  in step with the API.
- v202101beta1 attributes are kept as deprecated attributes. `synthetics/legacy.go` holds one alias table
  used to (a) send a configured legacy value as its replacement when the replacement is not configured,
  (b) keep legacy values in state on read, (c) fill legacy attributes in data sources and (d) upgrade
  schema version 0 state. `docs/UPGRADING.md` documents the same table.

## Consequences

- A new API field needs one schema line (plus an example or test); no conversion code.
- Behaviour that is not a plain field copy (status via SetTestStatus, definition oneof validation,
  `network_mesh` default block, agent `name` alias) stays explicit in the resource code.
- Optional settings are mostly Optional+Computed to accept server defaults. With terraform-plugin-sdk/v2 this
  means removing such an attribute from the configuration keeps the last value; this is documented.
- Moving to terraform-plugin-framework later can reuse the same proto reflection approach with framework
  types.
