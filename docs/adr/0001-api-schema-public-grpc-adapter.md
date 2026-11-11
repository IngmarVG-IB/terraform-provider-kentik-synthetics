# 0001. Build on api-schema-public gRPC clients through internal/kentikapi

- Status: Accepted
- Date: 2026-11-10

## Context

Up to 0.2.x the provider used `github.com/kentik/community_sdk_golang` (2021 pseudo-version), which wraps the
beta synthetics API v202101beta1 over REST with OpenAPI-generated models. That SDK is unmaintained, pulled
in vulnerable dependencies and has no support for the stable v202309 API (network_mesh tests, agent
alerts, SetTestStatus, agent updates). Kentik publishes generated Go gRPC clients for every v6 API in
`github.com/kentik/api-schema-public/gen/go`. A cloud export provider is being modernised the same way, and
the two may later merge into one `kentik/kentik` provider.

## Decision

- Call `kentik.synthetics.v202309.SyntheticsAdminService` with the generated gRPC client from
  api-schema-public, pinned by pseudo-version in `go.mod` and recorded in `kentik-api.json`.
- Put connection concerns in `internal/kentikapi`, which knows nothing about Terraform:
  `kentikapi.New(ctx, kentikapi.Config{Email, Token, Endpoint, Region, UserAgent, Timeout, Retry, ...})`.
  It resolves the endpoint (default `grpc.api.kentik.com:443`, EU `grpc.api.kentik.eu:443`, legacy REST
  hostnames mapped), sets up TLS 1.2+, attaches `X-CH-Auth-Email` / `X-CH-Auth-API-Token` as per-RPC
  credentials (refusing to send them over plaintext unless `Insecure` is set for local fakes), applies a
  per-attempt timeout and retries `Unavailable` / `ResourceExhausted` with exponential backoff and jitter.
- Typed service accessors (`SyntheticsAdmin()`, ...) live in `services.go`; any other service can be built
  from `Conn()`.
- Tests use `internal/fakesynthetics`, an in-memory `SyntheticsAdminService` served over gRPC bufconn.

## Consequences

- No hand-written API models; new API fields appear by bumping api-schema-public.
- The package shape matches the one planned for the cloudexport provider, so the two can share it when the
  providers merge, or both can switch to a future official Go SDK by replacing the adapter.
- REST/grpc-gateway is not used. Environments that only allow HTTPS/1.1 egress to Kentik need HTTP/2
  (gRPC) to `grpc.api.kentik.com:443`.
- Retrying `CreateTest` on `Unavailable` could in theory create a duplicate if the server committed the test
  before failing; the default of 5 attempts keeps the window small. Users can set `retry { max_attempts = 1 }`.
