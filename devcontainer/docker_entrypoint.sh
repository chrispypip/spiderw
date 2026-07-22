#!/usr/bin/env bash
#-------------------------------------------------------------------------------
# spiderw - docker_entrypoint.sh
#
# Description:  Entrypoint for the development docker container starting a clean
#               dbus session
# Usage:
#   ./docker_entrypoint.sh
#
# This script is part of the spiderw development environment. It is meant to
# be the entry point into the development docker container created from
# Dockerfile.dev.
#
#-------------------------------------------------------------------------------
set -euo pipefail

# Start dbus-run-session and open a shell inside it.
# This ensures all commands inherit the correct DBUS_SESSION_BUS_ADDRESS.
echo "[entrypoint] Starting dbus-run-session..."

# We wrap bash inside dbus-run-session so DBUS_SESSION_BUS_ADDRESS
# is automatically exported and correct for everything in this shell.
exec dbus-run-session -- "$@"

