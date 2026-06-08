#!/usr/bin/env bash
#-------------------------------------------------------------------------------
# spiderw - dev.sh
#
# Description:  Launch an interactive development environment inside the
#               spiderw-dev- container.
# Usage:
#   ./dev.sh                 # Open a bash shell inside the dev container
#   ./dev.sh nvim            # Run nvim inside the container
#   ./dev.sh go test ./...   # Run Go tests inside the container
#
# This script is editor-agnostic--it does NOT mount any editor config.
# The container runs using the project's Dockerfile.dev and docker-compose.yml.
#
#-------------------------------------------------------------------------------
set -euo pipefail

SERVICE="spiderw-dev"
REBUILD=0

if [[ "${1:-}" == "--rebuild" ]]; then
    REBUILD=1
    shift
fi

docker_compose=()
if docker compose version >/dev/null 2>&1; then
    docker_compose=(docker compose --project-directory ./dev)
elif command -v docker-compose >/dev/null 2>&1; then
    docker_compose=(docker-compose --project-directory ./dev)
else
    echo "[dev.sh] ERROR: Docker Compose not found (need 'docker compose' v2 or 'docker-compose')." >&2
    exit 1
fi

ensure_container() {
    if [[ "$REBUILD" == "1" ]]; then
        "${docker_compose[@]}" --project-directory ./dev build "$SERVICE"
    fi

    # Is it running?
    if "${docker_compose[@]}" --project-directory ./dev ps --services --filter "status=running" | grep -q \
            "$SERVICE"; then
        echo "[dev.sh] $SERVICE container already running"
        return
    fi

    # Does it exist but is stopped?
    if "${docker_compose[@]}" --project-directory ./dev ps --services --filter "status=exited" | grep -q \
            "$SERVICE"; then
        echo "[dev.sh] $SERVICE container exists but is stopped. Restarting it."
        "${docker_compose[@]}" --project-directory ./dev start "$SERVICE"
        return
    fi

    # Create the container
    "${docker_compose[@]}" --project-directory ./dev up -d --build "$SERVICE"
}
ensure_container

# Default command
if [ $# -eq 0 ]; then
    set -- bash
fi

echo "[dev.sh] Entering development container…"
echo "[dev.sh] Running command inside container:"
echo "         $*"
echo

# Run inside container
#   --rm: remove container after exit
#   --service-ports: allows port exposure if tests or hostapd require it
#   --use-aliases: ensures hostnames inside the network resolve
exec "${docker_compose[@]}" --project-directory ./dev run --rm \
    --service-ports \
    --use-aliases \
    "$SERVICE" "$@"
