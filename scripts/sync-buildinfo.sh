#!/bin/sh
# Copies the code hylad runs into internal/buildinfo/hyladSource, where
# package buildinfo embeds it: an embed pattern reaches only files of the
# package's own directory, and these are the module's. TestSourceSync
# reports a copy that is stale, and a release runs this first.
set -eu
cd "$(dirname "$0")/.."
exec python3 scripts/sync-buildinfo.py "$@"
