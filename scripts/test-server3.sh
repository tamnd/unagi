#!/usr/bin/env bash
# Run the full test suite on server3.
#
# This is the heavy run: the conformance corpus compiles a binary per fixture per
# tier, which is exactly the work that must stay off the laptop. It deploys the
# working tree first, then runs the suite on the remote.
#
# The suite scopes both TMPDIR and GOCACHE to a per-run directory under the
# remote's temp root and deletes it on exit, so a run leaves no build artifacts
# behind and the shared go cache never grows. Build concurrency is bounded so the
# linker's peak memory stays in budget on an 8 core box.
#
# Usage:
#   scripts/test-server3.sh                 # full suite, all packages
#   scripts/test-server3.sh ./pkg/build/    # one package
#   scripts/test-server3.sh -run TestFoo ./pkg/conformance/
#
# Environment:
#   UNAGI_TEST_BUILD_JOBS  concurrent builds on the remote (default 4)
#   UNAGI_ORACLE           set to 1 to run the CPython differential band
#                          (needs python3.14 on the remote PATH)

source "$(dirname "${BASH_SOURCE[0]}")/lib.sh"

"$(dirname "${BASH_SOURCE[0]}")/deploy.sh"

build_jobs="${UNAGI_TEST_BUILD_JOBS:-4}"
oracle="${UNAGI_ORACLE:-}"

# Default to the whole module when no test targets are given.
targets=("$@")
if [ ${#targets[@]} -eq 0 ]; then
	targets=("./...")
fi

say "running tests on $REMOTE_HOST (build_jobs=$build_jobs oracle=${oracle:-0})"
# Quote every argument for the remote shell so a -run pattern with pipes or
# spaces survives the trip intact instead of being reparsed as a pipeline.
remote_args="$(printf '%q ' "${targets[@]}")"
# -count=1 defeats the test result cache so a rerun actually reruns. The remote
# env carries our knobs; the suite scopes GOCACHE and TMPDIR itself.
ssh "$REMOTE_HOST" "cd $(printf '%q' "$REMOTE_DIR") && \
	env UNAGI_TEST_BUILD_JOBS=$(printf '%q' "$build_jobs") \
	${oracle:+UNAGI_ORACLE=$(printf '%q' "$oracle")} \
	go test -count=1 $remote_args"
