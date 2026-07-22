#!/usr/bin/env bash
#-------------------------------------------------------------------------------
# spiderw - preflight-host.sh
#
# Description:  Ensures the development host is set up and configured
#               properly for development in the development container.
# Usage:
#   ./preflight-host.sh
#
# Developers must have docker and docker-compose v2. They must also be able
# to run docker commands without running `sudo`.
#
#-------------------------------------------------------------------------------
set -euo pipefail

RED="$(printf '\033[31m')"
GREEN="$(printf '\033[32m')"
YELLOW="$(printf '\033[33m')"
RESET="$(printf '\033[0m')"

ok() { echo "${GREEN}[OK]${RESET} $*"; }
warn() { echo "${YELLOW}[WARN]${RESET} $*"; }
err() { echo "${RED}[ERROR]${RESET} $*" >&2; exit 1; }

echo "=== spiderw Host Preflight Check ==="

# Check docker installed
if ! command -v docker >/dev/null 2>&1; then
    err "Docker is not installed. Install Docker and try again."
else
    ok "Docker is installed"
fi

# Check docker-compose (v2)
if ! docker compose version >/dev/null 2>&1; then
    warn "docker compose v2 not found. Trying docker-compose..."
    if ! command -v docker-compose >/dev/null 2>&1; then
        err "docker-compose is not installed."
    else
        ok "docker-compose (legacy) installed"
    fi
else
    ok "docker compose v2 installed"
fi

# Check user can access docker without sudo
if ! docker info >/dev/null 2>&1; then
    err "Cannot access Docker daemon. Add your user to the docker group or fix permissions."
else
    ok "User can run Docker commands"
fi

# Check project directory writable
if [[ ! -w . ]]; then
    err "Project directory is not writable."
else
    ok "Project directory writable"
fi

# Check container is not already running in crash loop
if docker ps --filter "name=spiderw-dev" | grep -q spiderw-dev; then
    warn "Container spiderw-dev already running"
else
    ok "No existing spiderw-dev container"
fi

echo ""
echo "${GREEN} Host preflight passed${RESET}"
