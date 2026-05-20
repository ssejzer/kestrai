# Kestrai monorepo Makefile.
#
# Conventions:
#   - Top-level targets fan out to per-language targets.
#   - Per-language targets are self-contained; you can run them without a
#     full repo bootstrap if you only have that toolchain installed.
#   - Build output goes to ./bin/ (gitignored).
#
# Run `make help` for the full list.

SHELL := /bin/bash

GO            ?= go
UV            ?= uv
PNPM          ?= pnpm
BUF           ?= buf

VERSION       ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo 0.0.0-dev)
GO_LDFLAGS    := -X main.version=$(VERSION)

BIN_DIR       := bin
KESTRAI_BIN   := $(BIN_DIR)/kestrai

.DEFAULT_GOAL := help

# ---------------------------------------------------------------------------
# Help
# ---------------------------------------------------------------------------

.PHONY: help
help: ## Show this help.
	@awk 'BEGIN {FS = ":.*?## "} /^[a-zA-Z0-9_.-]+:.*?## / {printf "  \033[36m%-22s\033[0m %s\n", $$1, $$2}' $(MAKEFILE_LIST)

# ---------------------------------------------------------------------------
# Setup
# ---------------------------------------------------------------------------

.PHONY: setup
setup: setup-go setup-python setup-node ## Bootstrap every language workspace.

.PHONY: setup-go
setup-go: ## Download Go module deps.
	$(GO) work sync
	$(GO) mod download

.PHONY: setup-python
setup-python: ## Sync the uv workspace.
	$(UV) sync

.PHONY: setup-node
setup-node: ## Install pnpm workspace deps.
	$(PNPM) install

# ---------------------------------------------------------------------------
# Build
# ---------------------------------------------------------------------------

.PHONY: build
build: $(KESTRAI_BIN) ## Build the kestrai binary into ./bin/.

$(KESTRAI_BIN): $(BIN_DIR)
	$(GO) build -ldflags "$(GO_LDFLAGS)" -o $(KESTRAI_BIN) ./cmd/kestrai

$(BIN_DIR):
	@mkdir -p $(BIN_DIR)

# ---------------------------------------------------------------------------
# Lint
# ---------------------------------------------------------------------------

.PHONY: lint
lint: lint-go lint-python lint-node lint-headers lint-proto ## Lint everything.

.PHONY: lint-go
lint-go: ## Go vet + gofmt check.
	$(GO) vet ./...
	@unformatted=$$(gofmt -l . 2>/dev/null | grep -v '^gen/' || true); \
	if [ -n "$$unformatted" ]; then \
		echo "gofmt found unformatted files:"; echo "$$unformatted"; exit 1; \
	fi

.PHONY: lint-python
lint-python: ## Ruff check.
	$(UV) run ruff check .

.PHONY: lint-node
lint-node: ## Run lint scripts across pnpm workspaces (if any).
	$(PNPM) -r --if-present run lint

.PHONY: lint-headers
lint-headers: ## Verify Apache 2.0 headers on every source file.
	./scripts/check-license-headers.sh

.PHONY: lint-proto
lint-proto: ## Lint .proto files with buf.
	@command -v $(BUF) >/dev/null 2>&1 || { \
		echo "buf is required: https://buf.build/docs/installation"; exit 1; }
	$(BUF) lint

# ---------------------------------------------------------------------------
# Test
# ---------------------------------------------------------------------------

.PHONY: test
test: test-go test-python test-node ## Run all unit tests.

.PHONY: test-go
test-go: ## Run Go tests.
	$(GO) test ./...

.PHONY: test-python
test-python: ## Run Python tests.
	$(UV) run pytest

.PHONY: test-node
test-node: ## Run test scripts across pnpm workspaces (if any).
	$(PNPM) -r --if-present run test

.PHONY: test-e2e
test-e2e: ## Phase-0 end-to-end smoke (placeholder until `kestrai up` lands).
	@echo "test-e2e is a Phase-1 deliverable; nothing to run yet."

# ---------------------------------------------------------------------------
# Protobuf
# ---------------------------------------------------------------------------

.PHONY: proto
proto: ## Regenerate protobuf stubs into gen/go/.
	@command -v $(BUF) >/dev/null 2>&1 || { \
		echo "buf is required: https://buf.build/docs/installation"; exit 1; }
	$(BUF) generate
	@cd gen/go && $(GO) mod tidy

# ---------------------------------------------------------------------------
# Clean
# ---------------------------------------------------------------------------

.PHONY: clean
clean: ## Remove build artifacts.
	rm -rf $(BIN_DIR) dist/ build/
	find . -type d -name __pycache__ -prune -exec rm -rf {} +
	find . -type d -name .pytest_cache -prune -exec rm -rf {} +
	find . -type d -name .ruff_cache -prune -exec rm -rf {} +

.PHONY: clean-all
clean-all: clean ## Also remove installed deps (node_modules, .venv).
	rm -rf node_modules .venv
	find . -type d -name node_modules -prune -exec rm -rf {} +
