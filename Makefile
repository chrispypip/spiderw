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
# This script is editor-agnostic--it does NOT mount any editor config.
# The container runs using the project's Dockerfile.dev and docker-compose.yml.
#
#-------------------------------------------------------------------------------
SHELL := /bin/bash
SERVICE := spiderw-dev
COMPOSE := docker-compose
DEV := ./dev.sh

# Colors
GREEN := \033[1;32m
RED   := \033[1;31m
BLUE  := \033[1;34m
RESET := \033[0m


# -----------------------------------------------------------------------------------
# PHONY targets
# -----------------------------------------------------------------------------------
.PHONY: help bootstrap dev build rebuild up down logs shell lint lint-check
.PHONY: fmt fmt-check check-fmt test test-unit test-regression test-stress test-race
.PHONY: test-stress-race test-race-stress test-bench test-mock test-integration
.PHONY: test-all preflight check
.PHONY: clean all


# -----------------------------------------------------------------------------------
#  Docker compose. Prefer v2, fallback to v1
# -----------------------------------------------------------------------------------
define COMPOSE
	if docker compose version >/dev/null 2>&1; then \
		docker compose --project-directory ./dev $(1); \
	else \
		docker-compose --project-directory ./dev $(1); \
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

build: ## Build dev container image
	@echo -e "$(BLUE)[ build ]$(RESET) Building container image..."
	$(call require_host)
	@$(call COMPOSE,build $(SERVICE))

rebuild: ## Rebuild dev container image
	@echo -e "$(BLUE)[ rebuild ]$(RESET) Rebuilding container images..."
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

fmt: ## Run gofmt/goimports inside container
	@echo -e "$(BLUE)[ fmt ]$(RESET) Formatting..."
	@$(RUN) bash -lc "git ls-files -z '*.go' | xargs -0 gofmt -s -w && \
		                git ls-files -z '*.go' | xargs -0 goimports -w"

fmt-check: ## Run check of gofmt/goimports inside container
	@echo -e "$(BLUE)[ fmt-check ]$(RESET) Checking formatting..."
	@$(RUN) bash -lc '$(FMT_CHECK_CMD)'

check-fmt: fmt-check

test: ## Run Go tests inside container
	@echo -e "$(BLUE)[ test ]$(RESET) Running Go tests..."
	@$(RUN) go test -v ./...

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

test-mock: ## Run mock iwd integration tests inside container
	@echo -e "$(BLUE)[ mock-test ]$(RESET) Running iwd mock tests..."
	@$(RUN) go test -v -tags=integration ./...

test-integration: test-mock

test-all: test-unit test-regression test-stress test-race test-stress-race test-bench test-mock ## Run all Go tests inside container


# -----------------------------------------------------------------------------------
# Preflight
# -----------------------------------------------------------------------------------
preflight: ## Validate container environment
	@echo -e "$(BLUE)[ preflight ]$(RESET) Running host + container preflight..."
	$(call require_host)
	@./dev/scripts/preflight-host.sh
	@$(RUN) ./dev/scripts/preflight-container.sh


# -----------------------------------------------------------------------------------
# Check code quality
# -----------------------------------------------------------------------------------
check: fmt-check lint-check test-unit ## Check for code quality (formatting, linting, tests)


# -----------------------------------------------------------------------------------
# Cleanup
# -----------------------------------------------------------------------------------
clean: down ## Remove build artifacts and dev container
	@echo -e "$(BLUE)[ clean ]$(RESET) Cleaning artifacts..."
	@rm -rf ./build || true
	@find . -name '*.test' -delete || true
	@find . -type d -name 'testdata' -exec rm -rv {} +


# -----------------------------------------------------------------------------------
# All
# -----------------------------------------------------------------------------------
all: bootstrap preflight fmt-check lint
