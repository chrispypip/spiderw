#-------------------------------------------------------------------------------
# spiderw - Makefile
#
# Description:  Provides common development tasks such as building binaries,
#               running tests, running linters, and building development
#               spiderw-dev container.
# Usage:
#    make help      # Print out help info
#    make <target>  # Run specified target
#
# This script is editor-agnostic - it does NOT mount any editor config.
# The container runs using the project's Dockerfile.dev and docker-compose.yml.
#
#-------------------------------------------------------------------------------
SHELL := /bin/bash
SERVICE := spiderw-dev
COMPOSE := docker-compose
DEV := ./dev.sh

# CLI build. `make build` builds for the host; override GOOS/GOARCH to
# cross-compile (e.g. `make build GOARCH=arm64`). The CLI is pure Go, so
# CGO_ENABLED=0 yields a fully static binary that runs on any Linux host of the
# target arch (e.g. Raspberry Pi).
DIST := dist
CLI_PKG := ./cmd/spiderw
# Single-quote the ldflags: this string is interpolated inside a bash -lc "..."
# recipe, so double quotes here would collide with the outer ones.
CLI_BUILD := CGO_ENABLED=0 go build -trimpath -ldflags '-s -w'

# Target platform. Empty means "host" (go build's native default).
GOOS ?=
GOARCH ?=
# Only set the env vars that were actually overridden, so an unset one falls back
# to the host default rather than an empty (invalid) value.
CLI_ENV = $(if $(GOOS),GOOS=$(GOOS) )$(if $(GOARCH),GOARCH=$(GOARCH) )
# Output: plain `spiderw` for a host build, suffixed for a cross build so several
# arches can coexist in dist/.
CLI_OUT = $(DIST)/spiderw$(if $(or $(GOOS),$(GOARCH)),-$(or $(GOOS),$(shell go env GOOS))-$(or $(GOARCH),$(shell go env GOARCH)))

# Colors
GREEN := \033[1;32m
RED   := \033[1;31m
BLUE  := \033[1;34m
RESET := \033[0m


# -----------------------------------------------------------------------------------
# PHONY targets
# -----------------------------------------------------------------------------------
.PHONY: help
.PHONY: bootstrap preflight dev image rebuild-image up down logs shell
.PHONY: lint lint-check codespell fmt fmt-check check-fmt check
.PHONY: test test-unit test-regression test-stress test-race test-stress-race
.PHONY: test-race-stress test-bench test-fuzz test-fuzz-seed test-mock
.PHONY: test-integration test-all
.PHONY: build amd64 arm64
.PHONY: clean all


# -----------------------------------------------------------------------------------
#  Docker compose. Prefer v2, fallback to v1
# -----------------------------------------------------------------------------------
define COMPOSE
	if docker compose version >/dev/null 2>&1; then \
		docker compose --project-directory ./devcontainer $(1); \
	else \
		docker-compose --project-directory ./devcontainer $(1); \
	fi
endef


# -----------------------------------------------------------------------------------
# Command to check if gofmt and/or goimports detected formatting changes
# -----------------------------------------------------------------------------------
FMT_CHECK_CMD = unformatted=$$(git ls-files -z "*.go" | xargs -0 gofmt -s -l && \
								               git ls-files -z "*.go" | xargs -0 goimports -l); \
								if [ -n "$$unformatted" ]; then \
								  echo "ERROR: files will be reformatted. Run '\''make fmt'\'' before committing."; \
									echo "$$unformatted"; \
									exit 1; \
								fi


# -----------------------------------------------------------------------------------
#  In container, run dev.sh; else run commands directly
# -----------------------------------------------------------------------------------
IN_DEV_CONTAINER := $(shell test -f /etc/spiderw-dev-container && echo 1 || echo 0)
ifeq ($(IN_DEV_CONTAINER),1)
  RUN :=
else
	RUN := $(DEV)
endif

define require_host
  @if [ "$(IN_DEV_CONTAINER)" = "1" ]; then \
		echo "ERROR: '$@' must be run on the host (outside the dev container)."; \
		exit 1; \
	fi
endef

define require_container
  @if [ "$(IN_DEV_CONTAINER)" != "1" ]; then \
		echo "ERROR: '$@' must be run inside the dev container (or via dev.sh)."; \
		exit 1; \
	fi
endef


# -----------------------------------------------------------------------------------
# Help
# -----------------------------------------------------------------------------------
help: ### Show this help
	@awk 'BEGIN {FS = ":.*##"} \
		/^[a-zA-Z0-9_.-]+:.*##/ {printf "%-20s %s\n", $$1, $$2}' $(MAKEFILE_LIST)


# -----------------------------------------------------------------------------------
# Dev container workflow
# -----------------------------------------------------------------------------------
bootstrap: ## One-time bootstrap
	@echo -e "$(BLUE)[ bootstrap ]$(RESET) Running bootstrap command..."
	$(call require_host)
	@$(call COMPOSE,up -d --build $(SERVICE))

dev: ## Launch dev container shel
	@echo -e "$(BLUE)[ dev ]$(RESET) Entering development shell..."
	@$(RUN)

image: ## Build the dev container image
	@echo -e "$(BLUE)[ image ]$(RESET) Building container image..."
	$(call require_host)
	@$(call COMPOSE,build $(SERVICE))

rebuild-image: ## Rebuild the dev container image
	@echo -e "$(BLUE)[ rebuild-image ]$(RESET) Rebuilding container images..."
	@$(RUN) --rebuild

up: ## Start dev container (daemon)
	@echo -e "$(BLUE)[ up ]$(RESET) Starting $(SERVICE)..."
	$(call require_host)
	@$(call COMPOSE,up -d $(SERVICE))

down: ## Stop dev container
	@echo -e "$(BLUE)[ down ]$(RESET) Stopping containers..."
	$(call require_host)
	@$(call COMPOSE,down)

logs: ## Show container logs
	$(call require_host)
	@$(call COMPOSE,logs -f $(SERVICE))

shell: ## Exec into running container
	@echo -e "$(BLUE)[ shell ]$(RESET) Exec into running container..."
	$(call require_host)
	@$(call COMPOSE,exec $(SERVICE) bash)


# -----------------------------------------------------------------------------------
# Go tasks (Always run INSIDE the container)
# -----------------------------------------------------------------------------------
lint: ## Run Go linting (golangci-lint) inside container
	@echo -e "$(BLUE)[ lint ]$(RESET) Running golangci-lint..."
	@$(RUN) bash -lc "golangci-lint run ./..."

lint-check: lint

ascii-check: ## Fail if any tracked file contains a non-ASCII character
	@echo -e "$(BLUE)[ ascii ]$(RESET) Checking for non-ASCII characters..."
	@$(RUN) ./scripts/ascii-check.sh

codespell: ## Run codespell spell-check inside container
	@echo -e "$(BLUE)[ codespell ]$(RESET) Running codespell..."
	@$(RUN) bash -lc "codespell"

fmt: ## Run gofmt/goimports inside container
	@echo -e "$(BLUE)[ fmt ]$(RESET) Formatting..."
	@$(RUN) bash -lc "git ls-files -z '*.go' | xargs -0 gofmt -s -w && \
		                git ls-files -z '*.go' | xargs -0 goimports -w"

fmt-check: ## Run check of gofmt/goimports inside container
	@echo -e "$(BLUE)[ fmt-check ]$(RESET) Checking formatting..."
	@$(RUN) bash -lc '$(FMT_CHECK_CMD)'

check-fmt: fmt-check

# Every test file in this repo carries a build tag, so a bare `go test ./...`
# matches nothing and exits green having run zero tests. `test` therefore runs the
# tiers explicitly.
test: test-unit test-race test-stress test-regression test-fuzz-seed test-integration ## Run every test tier inside container
	@echo -e "$(BLUE)[ test ]$(RESET) All test tiers passed."

test-unit: ## Run Go unit tests inside container
	@echo -e "$(BLUE)[ test ]$(RESET) Running Go unit tests..."
	@$(RUN) go test -v -tags=unit ./...

test-regression: ## Run Go regression tests inside container
	@echo -e "$(BLUE)[ test ]$(RESET) Running Go regression tests..."
	@$(RUN) go test -v -tags=regression ./...

test-race: ## Run Go race tests inside container
	@echo -e "$(BLUE)[ test ]$(RESET) Running Go race tests..."
	@$(RUN) go test -v -race -tags=race ./...

test-stress: ## Run Go stress tests inside container
	@echo -e "$(BLUE)[ test ]$(RESET) Running Go stress tests..."
	@$(RUN) go test -v -tags=stress -timeout=1m ./...

test-stress-race: ## Run Go stress tests with race inside container
	@echo -e "$(BLUE)[ test ]$(RESET) Running Go race stress tests..."
	@$(RUN) go test -v -race -tags=stress ./...

test-race-stress: test-stress-race

test-bench: ## Run Go benchmarks inside container
	@echo -e "$(BLUE)[ test ]$(RESET) Running Go benchmark tests..."
	@$(RUN) go test -v -bench=. -benchmem -tags=bench ./...

# The fuzz tier is build-tagged, so no other target (and neither `go build` nor
# `go vet`) ever compiles it. Without these it can rot unnoticed.
test-fuzz-seed: ## Run the fuzz seed corpus (compiles + executes every target once)
	@echo -e "$(BLUE)[ test ]$(RESET) Running fuzz seed corpus..."
	@$(RUN) go test -tags=fuzz ./...

test-fuzz: ## Fuzz every target briefly (FUZZTIME=30s make test-fuzz)
	@echo -e "$(BLUE)[ test ]$(RESET) Fuzzing all targets for $(or $(FUZZTIME),20s)..."
	@$(RUN) ./scripts/fuzz.sh $(or $(FUZZTIME),20s)

test-mock: ## Run mock iwd integration tests inside container
	@echo -e "$(BLUE)[ mock-test ]$(RESET) Running iwd mock tests..."
	@$(RUN) go test -v -tags=integration ./...

test-integration: test-mock

test-all: test-unit test-regression test-stress test-race test-stress-race test-bench test-fuzz-seed test-mock ## Run all Go tests inside container


# -----------------------------------------------------------------------------------
# CLI binary (output to dist/). `make build` targets the host; override GOOS/
# GOARCH to cross-compile, or use the arch aliases below.
# -----------------------------------------------------------------------------------
build: ## Build the CLI (host by default; override GOOS/GOARCH, e.g. GOARCH=arm64)
	@echo -e "$(BLUE)[ build ]$(RESET) Building CLI for $(or $(GOOS),host)/$(or $(GOARCH),native)..."
	@$(RUN) bash -lc "mkdir -p $(DIST) && $(CLI_ENV)$(CLI_BUILD) -o $(CLI_OUT) $(CLI_PKG)"
	@echo -e "$(GREEN)[ ok ]$(RESET) $(CLI_OUT)"

amd64: ## Build the CLI for linux/amd64 -> dist/spiderw-linux-amd64
	@$(MAKE) --no-print-directory build GOOS=linux GOARCH=amd64

arm64: ## Build the CLI for linux/arm64 (e.g. Raspberry Pi) -> dist/spiderw-linux-arm64
	@$(MAKE) --no-print-directory build GOOS=linux GOARCH=arm64


# -----------------------------------------------------------------------------------
# Preflight
# -----------------------------------------------------------------------------------
preflight: ## Validate container environment
	@echo -e "$(BLUE)[ preflight ]$(RESET) Running host + container preflight..."
	$(call require_host)
	@./devcontainer/scripts/preflight-host.sh
	@$(RUN) ./devcontainer/scripts/preflight-container.sh


# -----------------------------------------------------------------------------------
# Check code quality
# -----------------------------------------------------------------------------------
check: fmt-check lint-check codespell test-unit ## Check for code quality (formatting, linting, tests)


# -----------------------------------------------------------------------------------
# Cleanup
# -----------------------------------------------------------------------------------
clean: down ## Remove build artifacts and dev container
	@echo -e "$(BLUE)[ clean ]$(RESET) Cleaning artifacts..."
	@rm -rf ./build ./$(DIST) || true
	@find . -name '*.test' -delete || true
	@find . -type d -name 'testdata' -exec rm -rv {} +


# -----------------------------------------------------------------------------------
# All
# -----------------------------------------------------------------------------------
all: bootstrap preflight fmt-check lint
