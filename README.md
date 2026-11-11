# Terraform Provider for Kentik synthetic monitoring

Manage [Kentik synthetic monitoring](https://kb.kentik.com/v4/Ma00.htm) with Terraform: synthetic tests of
every type (`hostname`, `ip`, `network_grid`, `agent`, `network_mesh`, `flow`, `url`, `page_load`, `dns`,
`dns_grid`) and agent lookups, using the stable Kentik v6 gRPC API `kentik.synthetics.v202309`.

- User documentation: [Terraform registry](https://registry.terraform.io/providers/kentik/kentik-synthetics/latest/docs)
  (source in [`docs/`](docs/)).
- Upgrading from 0.2.x: [`docs/UPGRADING.md`](docs/UPGRADING.md).
- Examples: [`examples/`](examples/), including one per test type in [`examples/test-types/`](examples/test-types/).

## Requirements

- Terraform >= 1.13 or OpenTofu
- Go >= 1.26 (to build)

## Development

See [AGENTS.md](AGENTS.md) for the layout, conventions and release process. Common commands:

```sh
make build              # build the provider binary
make test               # offline unit tests (fake gRPC API, no credentials needed)
make lint vuln          # golangci-lint and govulncheck
make docs               # regenerate docs/ with tfplugindocs
make validate-examples  # terraform validate every example against the local build
```

Acceptance tests create real resources and need a Kentik tenant:
`TF_ACC=1 KTAPI_AUTH_EMAIL=... KTAPI_AUTH_TOKEN=... KTAPI_ACC_AGENT_ID=... make testacc`.

### Debugging

```sh
make build
dlv exec ./terraform-provider-kentik-synthetics -- -debug
# then export the printed TF_REATTACH_PROVIDERS value in the shell that runs terraform
```

## Security

See [SECURITY.md](SECURITY.md).
