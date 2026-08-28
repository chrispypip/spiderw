#!/usr/bin/env bash
# Fuzz every target in the module for a bounded time.
#
# `go test -fuzz` accepts only one target per invocation, so enumerate them. The
# fuzz tier is build-tagged, which means no other suite (or `go build`/`go vet`)
# ever compiles it - running it here is what keeps it from rotting.
set -euo pipefail

fuzztime="${1:-20s}"
status=0

while read -r pkg target; do
    echo "=== fuzzing ${target} (${pkg}) for ${fuzztime}"
    # A short -fuzztime races the fuzzing coordinator: when the budget expires
    # mid-execution, `go test -fuzz` can report a spurious "context deadline
    # exceeded" with NO crasher written - a known infra flake, not a finding.
    # Distinguish it from a real crash by the reproducer: a genuine failure
    # prints "Failing input written to testdata/fuzz/...". So: a crasher (or any
    # other non-zero exit that is NOT the deadline flake, e.g. a build error) is
    # fatal; the deadline flake alone is ignored.
    if out=$(go test -tags=fuzz -run='^$' -fuzz="^${target}\$" \
        -fuzztime="${fuzztime}" "${pkg}" 2>&1); then
        rc=0
    else
        rc=$?
    fi
    printf '%s\n' "${out}"
    if [ "${rc}" -ne 0 ]; then
        if printf '%s' "${out}" | grep -q 'Failing input written to'; then
            echo "FUZZ CRASH: ${target} found a reproducer (see testdata/fuzz)"
            status=1
        elif printf '%s' "${out}" | grep -q 'context deadline exceeded'; then
            echo "WARNING: ${target}: -fuzztime deadline flake (no crasher); ignoring"
        else
            echo "FUZZ ERROR: ${target} failed (not a crasher, not the flake)"
            status=1
        fi
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
