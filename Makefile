NAME            := kentik-synthetics
BINARY          := terraform-provider-$(NAME)
VERSION         ?= $(or $(shell git describe --tags --abbrev=0 2>/dev/null | sed -e 's/^v//'),0.0.0)
OS_ARCH         := $(shell go env GOHOSTOS)_$(shell go env GOHOSTARCH)
PLUGIN_DIR      := $(HOME)/.terraform.d/plugins/registry.terraform.io/kentik/$(NAME)/$(VERSION)/$(OS_ARCH)

# Pinned tool versions. Tools are built with the local Go toolchain via `go run`
# so they always understand the Go version in go.mod.
GOLANGCI_LINT   ?= go run github.com/golangci/golangci-lint/v2/cmd/golangci-lint@v2.15.0
GOVULNCHECK     ?= go run golang.org/x/vuln/cmd/govulncheck@v1.1.4
# Terraform-compatible CLI used by fmt and validate-examples (terraform or tofu).
TF              ?= terraform

.DEFAULT_GOAL := build

.PHONY: build
build: ## Build the provider binary
	go build -trimpath -o $(BINARY) .

.PHONY: install
install: build ## Install the provider into the local plugin directory
	mkdir -p $(PLUGIN_DIR)
	mv $(BINARY) $(PLUGIN_DIR)/

.PHONY: test
test: ## Run offline unit tests (fake gRPC API over bufconn; needs terraform or tofu on PATH)
	go test ./... $(TESTARGS) -timeout=15m -count=1

.PHONY: testacc
testacc: ## Acceptance tests against a LIVE tenant (KTAPI_AUTH_EMAIL, KTAPI_AUTH_TOKEN, KTAPI_ACC_AGENT_ID)
	@test -n "$$KTAPI_AUTH_EMAIL" -a -n "$$KTAPI_AUTH_TOKEN" || (echo "KTAPI_AUTH_EMAIL and KTAPI_AUTH_TOKEN must be set" && exit 1)
	TF_ACC=1 go test ./synthetics/ -run '^TestAcc' $(TESTARGS) -timeout=60m -count=1 -v

.PHONY: lint
lint: ## Run golangci-lint
	$(GOLANGCI_LINT) run ./...

.PHONY: fmt
fmt: ## Format Go code (gofumpt + goimports) and Terraform examples
	$(GOLANGCI_LINT) fmt ./...
	$(TF) fmt -recursive ./examples/ ./synthetics/testdata/

.PHONY: vuln
vuln: ## Scan for known vulnerabilities
	$(GOVULNCHECK) ./...

.PHONY: docs
docs: ## Regenerate docs/ with tfplugindocs
	go generate ./...

.PHONY: generate
generate: docs ## Run all code and doc generators

.PHONY: check-generate
check-generate: generate ## Fail if generated files are out of date
	git diff --exit-code -- docs/ || (echo "Generated files are out of date: run 'make generate' and commit." && exit 1)

.PHONY: check-go-mod
check-go-mod: ## Fail if go.mod/go.sum are not tidy
	go mod tidy
	git diff --exit-code -- go.mod go.sum

.PHONY: check-manifest
check-manifest: ## Validate kentik-api.json
	go test -run '^TestKentikAPIManifest$$' -count=1 .

# Uses a throw-away CLI config with dev_overrides pointing at the local build, so no provider is
# downloaded, no `init` is needed and no API is called.
.PHONY: validate-examples
validate-examples: build ## terraform validate every example against the local build
	@set -e; rc=$$(mktemp); \
	printf 'provider_installation {\n  dev_overrides {\n    "kentik/kentik-synthetics" = "%s"\n  }\n  direct {}\n}\n' "$(CURDIR)" > "$$rc"; \
	for d in $$(find examples -name '*.tf' -not -path '*/common/*' -exec dirname {} \; | sort -u); do \
		echo "==> $$d"; \
		TF_CLI_CONFIG_FILE="$$rc" $(TF) -chdir="$$d" validate -no-color; \
	done

.PHONY: help
help: ## Show this help
	@grep -E '^[a-zA-Z_-]+:.*?## ' $(MAKEFILE_LIST) | awk 'BEGIN {FS = ":.*?## "}; {printf "%-18s %s\n", $$1, $$2}'
