# Developer convenience targets.
#
# Release artifacts are built by GoReleaser, not by this file -- see
# .goreleaser.yaml and .github/workflows/release.yaml. `make snapshot`
# reproduces a full release locally without publishing anything.

BINARY_NAME := understackctl
BUILD_DIR   := build
GORELEASER  ?= goreleaser

VERSION_PKG := github.com/rackerlabs/understackctl/internal/version
VERSION     ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo dev)
COMMIT      ?= $(shell git rev-parse --short HEAD 2>/dev/null || echo unknown)
DATE        ?= $(shell date -u +%Y-%m-%dT%H:%M:%SZ)

LDFLAGS := -s -w \
	-X $(VERSION_PKG).version=$(VERSION) \
	-X $(VERSION_PKG).commit=$(COMMIT) \
	-X $(VERSION_PKG).date=$(DATE)

.DEFAULT_GOAL := help

.PHONY: help
help: ## Show this help
	@grep -hE '^[a-zA-Z_-]+:.*?## ' $(MAKEFILE_LIST) \
		| awk 'BEGIN {FS = ":.*?## "}; {printf "  \033[36m%-14s\033[0m %s\n", $$1, $$2}'

.PHONY: build
build: ## Build for the current platform into build/
	@mkdir -p $(BUILD_DIR)
	CGO_ENABLED=0 go build -trimpath -ldflags '$(LDFLAGS)' -o $(BUILD_DIR)/$(BINARY_NAME) .
	@echo "built $(BUILD_DIR)/$(BINARY_NAME) ($(VERSION))"

.PHONY: install
install: ## Install into $$GOBIN (or $$GOPATH/bin)
	CGO_ENABLED=0 go install -trimpath -ldflags '$(LDFLAGS)' .

.PHONY: test
test: ## Run tests
	go test ./...

.PHONY: lint
lint: ## Run golangci-lint
	golangci-lint run

.PHONY: fmt
fmt: ## Format all Go sources
	gofmt -w -l .

.PHONY: fmt-check
fmt-check: ## Fail if any Go source is not gofmt-clean
	@out=$$(gofmt -l .); \
	if [ -n "$$out" ]; then \
		echo "not gofmt-clean:"; echo "$$out"; exit 1; \
	fi

.PHONY: tidy
tidy: ## Tidy go.mod / go.sum
	go mod tidy

.PHONY: completions
completions: ## Generate shell completions into completions/
	sh ./scripts/completions.sh

.PHONY: check
check: ## Validate .goreleaser.yaml
	$(GORELEASER) check

.PHONY: snapshot
snapshot: ## Build all release artifacts locally into dist/ (no publishing)
	$(GORELEASER) release --snapshot --clean --skip=validate

.PHONY: clean
clean: ## Remove build output
	rm -rf $(BUILD_DIR) dist completions
