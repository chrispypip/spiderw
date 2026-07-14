#!/usr/bin/env bash
# Fuzz every target in the module for a bounded time.
#
# `go test -fuzz` accepts only one target per invocation, so enumerate them. The
# fuzz tier is build-tagged, which means no other suite (or `go build`/`go vet`)
# ever compiles it — running it here is what keeps it from rotting.
set -euo pipefail

fuzztime="${1:-20s}"
status=0

while read -r pkg target; do
    echo "=== fuzzing ${target} (${pkg}) for ${fuzztime}"
    if ! go test -tags=fuzz -run='^$' -fuzz="^${target}\$" -fuzztime="${fuzztime}" "${pkg}"; then
        status=1
    fi
done < <(
    grep -rhoE '^func (Fuzz\w+)\(' --include='*_test.go' . |
        sed -E 's/^func (Fuzz[A-Za-z0-9_]+)\($/\1/' |
        while read -r fn; do
            file="$(grep -rl "func ${fn}(" --include='*_test.go' . | head -1)"
            echo "./$(dirname "${file#./}") ${fn}"
        done
)

exit "${status}"
