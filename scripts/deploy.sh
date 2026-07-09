#!/usr/bin/env bash
# Push the current working tree to server3.
#
# This mirrors the local checkout, uncommitted changes and all, to REMOTE_DIR on
# the remote with rsync --delete, so what runs there is exactly what is on disk
# here. It excludes .git and local build output: the tests do not need git
# history, and shipping a stale local binary would only get in the way.
#
# Usage: scripts/deploy.sh

source "$(dirname "${BASH_SOURCE[0]}")/lib.sh"

say "syncing $REPO_ROOT to $REMOTE_HOST:$REMOTE_DIR"
ssh "$REMOTE_HOST" "mkdir -p '$REMOTE_DIR'"
rsync -az --delete \
	--exclude '.git/' \
	--exclude 'bin/' \
	--exclude '*.test' \
	--exclude 'unagi' \
	"$REPO_ROOT/" "$REMOTE_HOST:$REMOTE_DIR/"
say "synced"
