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

# An ssh command of its own, for a host that lives in a config file of its
# own: RSYNC_RSH="ssh -F ~/.ssh/elsewhere" ./scripts/deploy-static.sh host:/dir
rsh="${RSYNC_RSH:-ssh}"

bun run build
rsync -az --delete $dry -e "$rsh" dist/client/ "$dest/"

# rsync keeps each file's mode, and a file only its owner can read is a 403
# from nginx, which runs as another user: make the whole tree readable.
host="${dest%%:*}"
dir="${dest#*:}"
if [ -n "$dry" ]; then
  echo "dist/client -> $dest (dry run)"
elif [ "$host" = "$dest" ]; then
  chmod -R a+rX "$dir"
  echo "dist/client -> $dest"
else
  $rsh "$host" "chmod -R a+rX '$dir'"
  echo "dist/client -> $dest"
fi
