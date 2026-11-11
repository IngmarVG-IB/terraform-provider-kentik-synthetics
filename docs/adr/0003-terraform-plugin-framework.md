# 0003. Migrate to terraform-plugin-framework (protocol 6)

- Status: Proposed
- Date: 2026-11-10

## Context

The provider is built on terraform-plugin-sdk/v2 and serves plugin protocol 5. HashiCorp develops new
features only in terraform-plugin-framework (protocol 6): nested attributes instead of single-element
blocks, real null/unknown handling, plan modifiers, write-only and ephemeral attributes, functions, and
`moved` support across resource types. Decision D10 targets protocol 6.

Several limitations of this provider come from SDKv2:

- Optional+Computed attributes cannot be cleared by removing them from the configuration, and an empty
  list in configuration is treated as unset (documented in docs/UPGRADING.md).
- A nil set is stored as an empty set, which needs a workaround to avoid null -> [] refresh diffs.
- Nested objects are lists with `MaxItems: 1`, so every reference needs `settings.0.` / `settings[0]`.

At the same time:

- The v202309 migration (this branch) already changes a lot for users; a second schema shape change in the
  same release (blocks to nested attributes changes `settings[0].ping[0].count` to `settings.ping.count`)
  would break every reference in user configurations, which needs a major version.
- The cloudexport provider is being modernised in parallel with the same `internal/kentikapi` shape, and
  consolidating into one `kentik/kentik` provider is under consideration (findings F3, F10).

## Options

1. **Migrate this provider now**, keeping block syntax (framework `ListNestedBlock` with size validators) so
   configurations do not change, and port the state upgrader. Gains protocol 6 and better null handling,
   but keeps the block shape, and the work would be repeated or reshaped during consolidation.
2. **Mux first**: serve the existing SDKv2 resources and new framework resources side by side with
   terraform-plugin-mux (protocol 6 via tf5to6server), and write new resources in the framework. Low risk,
   incremental, no user-visible change.
3. **Do it in the consolidated `kentik/kentik` provider**: build that provider on the framework from the
   start, with nested attributes and generated schemas, and give it its own migration path (new resource
   type names, `moved` blocks from `kentik-synthetics_*` once supported across providers, or documented
   `terraform state mv`/import).

## Recommendation

Option 3, preceded by option 2 if new resources are needed before consolidation. Ship this branch as a
v0.3.0 security and API release on SDKv2 (protocol 5), so users get the fixes and v202309 without a schema
shape change, and put the framework rewrite where a major, user-visible change is already planned. The
proto-reflection conversion (ADR 0002) carries over: it maps proto fields to framework `attr.Value`s with the
same rules.

## Consequences if accepted

- `terraform-registry-manifest.json` stays at protocol 5.0 for 0.3.x.
- The SDKv2 workarounds listed above stay until the framework provider exists.
- The consolidated provider needs its own ADR covering naming, migration of state from both providers and
  generation of schemas from api-schema-public.
