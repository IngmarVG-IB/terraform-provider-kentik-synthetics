# Security policy

## Reporting a vulnerability

Please report security issues privately through GitHub's
[private vulnerability reporting](https://github.com/kentik/terraform-provider-kentik-synthetics/security/advisories/new)
on the upstream repository. Do not open a public issue or pull request for a suspected vulnerability.

Include the affected provider version, a description of the problem and, if possible, steps to reproduce.
Never include Kentik API tokens or other credentials in a report.

## Supported versions

| Version | Supported |
|---------|-----------|
| Unreleased (`master`, synthetics API v202309) | Yes |
| 0.2.x (synthetics API v202101beta1) | No: upgrade to the next release |
| < 0.2 | No |

## Dependency scanning

`make vuln` runs `govulncheck ./...`; CI runs it on every push and pull request, and Dependabot proposes
weekly grouped updates for Go modules and GitHub Actions. There are no known, unresolved vulnerabilities
at the time of writing. Any accepted exception must be listed here with a justification.
