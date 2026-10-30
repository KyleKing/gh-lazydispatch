#!/usr/bin/env bash
# Fails when a package with a floor drops below it. The module-wide floor in
# `mise run test:coverage-min` lets one well-tested package hide another that is
# not, so the packages with the largest untested blocks get their own.
set -euo pipefail

floors="
internal/app 80
internal/citiming 50
internal/cli 65
internal/github 50
internal/ui/modal 80
internal/ui/panes 60
"

output=$(go test -cover ./internal/... 2>&1)
status=0

while read -r pkg floor; do
    [ -z "$pkg" ] && continue
    got=$(printf '%s\n' "$output" | awk -v p="github.com/kyleking/gh-lazydispatch/$pkg" '$2 == p {gsub(/%/, "", $5); print $5}')
    if [ -z "$got" ]; then
        echo "no coverage reported for $pkg" >&2
        status=1
    elif awk -v g="$got" -v f="$floor" 'BEGIN {exit !(g < f)}'; then
        echo "$pkg: ${got}% is under its ${floor}% floor" >&2
        status=1
    fi
done <<<"$floors"

exit $status
