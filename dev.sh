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
# This script is editor-agnostic - it does NOT mount any editor config.
# The container runs using the project's Dockerfile.dev and docker-compose.yml.
#
#-------------------------------------------------------------------------------
set -euo pipefail

./devcontainer/dev.sh "$@"
