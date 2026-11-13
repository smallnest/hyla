#!/bin/sh
# Build the site and copy it to a web root. The build is a folder of files:
# nothing runs on the server but the web server itself.
#
#   ./scripts/deploy-static.sh                    # rpcx.io:/opt/hyla
#   ./scripts/deploy-static.sh me@host:/var/www   # somewhere else
#   ./scripts/deploy-static.sh --dry-run          # see what would go
#
# The copy mirrors dist/client into the destination: files it no longer has
# are deleted there. Point it at a directory the site owns, not at a shared
# web root.
set -eu
cd "$(dirname "$0")/.."

dry=""
if [ "${1:-}" = "--dry-run" ]; then
  dry="--dry-run"
  shift
fi
dest="${1:-rpcx.io:/opt/hyla}"

bun run build
rsync -az --delete --chmod=D755,F644 $dry dist/client/ "$dest/"
echo "dist/client -> $dest"
