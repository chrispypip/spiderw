#!/usr/bin/env bash
# Prove spiderw can read a REAL iwd (3.12) over the system bus.
#
# This is the first time spiderw's parsers meet a real daemon rather than the
# pure-Go mock, so it is the highest-signal mock-vs-reality check. It is
# read-only: no mode changes, no connects. Every bug this project shipped came
# from the mock being more forgiving than iwd, so any parse error or surprising
# shape here is exactly what we are looking for.
#
# Not `set -e`: run every read and show all output, so a failure in one does not
# hide the others. We assess the full picture together.
set -uo pipefail

run() {
    echo
    echo "== spiderw $* =="
    spiderw "$@" || echo "  (exit $?)"
}

echo "iwd version:"; iw --version 2>/dev/null || true
echo "wireless devices iwd sees (iw dev):"
iw dev 2>&1 | sed 's/^/  /' || true

# No --session flag: these hit the system bus, which is the real iwd.
run daemon info
run adapter list
run adapter status
run device list
run device status

# JSON too - exercises the structured-output path against real replies.
echo
echo "== spiderw --json device status =="
spiderw --json device status || echo "  (exit $?)"

echo
echo "[smoke] done"
