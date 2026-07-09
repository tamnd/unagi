#!/usr/bin/env bash
# The only tests that are safe to run on the laptop.
#
# This runs the fast, compile-free checks: gofmt, go vet, and go test -short,
# which skips every test that compiles a fixture binary. Those compiling tests
# are what churn the SSD and can fill the disk, so they run on server3 through
# test-server3.sh, never here.
#
# Use this for a quick local sanity pass before pushing. For the real
# conformance run, use scripts/test-server3.sh.
#
# Usage: scripts/test-local.sh [packages...]   (default ./...)

source "$(dirname "${BASH_SOURCE[0]}")/lib.sh"

cd "$REPO_ROOT"

targets=("$@")
if [ ${#targets[@]} -eq 0 ]; then
	targets=("./...")
fi

say "gofmt"
unformatted="$(gofmt -l $(git ls-files '*.go'))"
if [ -n "$unformatted" ]; then
	printf 'gofmt needs to run on:\n%s\n' "$unformatted" >&2
	exit 1
fi

say "go vet"
go vet "${targets[@]}"

say "go test -short"
go test -short -count=1 "${targets[@]}"

say "local checks passed (heavy corpus intentionally skipped, run test-server3.sh for it)"
