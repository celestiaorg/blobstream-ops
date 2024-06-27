#!/usr/bin/env bash
set -e

PKGS=$(go list ./...)

set -e
echo "mode: atomic" > coverage.txt
# shellcheck disable=SC2068
for pkg in ${PKGS[@]}; do
    go test -v -timeout 30m -race -test.short -coverprofile=profile.out -covermode=atomic "$pkg"
    if [ -f profile.out ]; then
        tail -n +2 profile.out >> coverage.txt;
        rm profile.out
    fi
done
