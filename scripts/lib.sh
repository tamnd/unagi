#!/usr/bin/env bash
# Shared configuration for the deploy and test scripts.
#
# The heavy test suite compiles a binary per fixture per tier, which is a lot of
# short-lived files and a lot of linker memory. That work belongs on server3, an
# 8 core box with plenty of free disk, not on the laptop where it churns the SSD
# and can fill the volume mid link. These scripts push the working tree to
# server3 and run the suite there. The only thing that runs locally is the fast,
# compile-free subset in test-local.sh.

set -euo pipefail

# The SSH host from ~/.ssh/config. Override with UNAGI_REMOTE_HOST if your config
# names it differently.
REMOTE_HOST="${UNAGI_REMOTE_HOST:-server3}"

# Where the working tree lands on the remote. It is a scratch checkout owned by
# these scripts, safe to wipe.
REMOTE_DIR="${UNAGI_REMOTE_DIR:-/root/unagi}"

# The repo root, resolved from this script's location so the scripts work from
# any working directory.
REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"

# say prints a short status line to stderr so it never mixes into piped output.
say() { printf '>> %s\n' "$*" >&2; }
