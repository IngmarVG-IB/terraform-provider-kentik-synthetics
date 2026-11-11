NAME            := kentik-synthetics
BINARY          := terraform-provider-$(NAME)
VERSION         ?= $(or $(shell git describe --tags --abbrev=0 2>/dev/null | sed -e 's/^v//'),0.0.0)
OS_ARCH         := $(shell go env GOHOSTOS)_$(shell go env GOHOSTARCH)
PLUGIN_DIR      := $(HOME)/.terraform.d/plugins/registry.terraform.io/kentik/$(NAME)/$(VERSION)/$(OS_ARCH)

# Pinned tool versions. Tools are built with the local Go toolchain via `go run`
# so they always understand the Go version in go.mod.
GOLANGCI_LINT   ?= go run github.com/golangci/golangci-lint/v2/cmd/golangci-lint@v2.15.0
GOVULNCHECK     ?= go run golang.org/x/vuln/cmd/govulncheck@v1.1.4

TEST_API_SERVER_ENDPOINT ?= localhost:9955

.DEFAULT_GOAL := build

.PHONY: build
build: ## Build the provider binary
	go build -trimpath -o $(BINARY) .

.PHONY: install
install: build ## Install the provider into the local plugin directory
	mkdir -p $(PLUGIN_DIR)
	mv $(BINARY) $(PLUGIN_DIR)/

.PHONY: test
test: ## Run offline unit tests (no Kentik API access needed)
	TEST_API_SERVER_ENDPOINT=$(TEST_API_SERVER_ENDPOINT) ./tools/start_test_api_server.sh
	KTAPI_URL="http://$(TEST_API_SERVER_ENDPOINT)" KTAPI_AUTH_EMAIL="dummy@example.com" KTAPI_AUTH_TOKEN="dummy" \
		go test ./... $(TESTARGS) -timeout=10m -count=1 || (./tools/stop_test_api_server.sh && exit 1)
	./tools/stop_test_api_server.sh

.PHONY: testacc
testacc: ## Run acceptance tests against a live tenant (needs TF_ACC=1, KTAPI_AUTH_EMAIL, KTAPI_AUTH_TOKEN)
	@test -n "$$KTAPI_AUTH_EMAIL" -a -n "$$KTAPI_AUTH_TOKEN" || (echo "KTAPI_AUTH_EMAIL and KTAPI_AUTH_TOKEN must be set" && exit 1)
	TF_ACC=1 go test ./... -run '^TestAcc' $(TESTARGS) -timeout=60m -count=1

.PHONY: lint
lint: ## Run golangci-lint
	$(GOLANGCI_LINT) run ./...

.PHONY: fmt
fmt: ## Format Go code (gofumpt + goimports)
	$(GOLANGCI_LINT) fmt ./...
	terraform fmt -recursive ./examples/

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

.PHONY: help
help: ## Show this help
	@grep -E '^[a-zA-Z_-]+:.*?## ' $(MAKEFILE_LIST) | awk 'BEGIN {FS = ":.*?## "}; {printf "%-16s %s\n", $$1, $$2}'

.PHONY: check-manifest
check-manifest: ## Validate kentik-api.json
	go test -run '^TestKentikAPIManifest$$' -count=1 .
