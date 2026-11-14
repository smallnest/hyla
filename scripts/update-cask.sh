#!/bin/sh
# Point the Homebrew tap's cask at a release, after that release is published:
#
#   ./scripts/update-cask.sh          # the latest release
#   ./scripts/update-cask.sh 3.8.1    # one named version
#
# It reads each dmg's sha256 from the release (GitHub keeps the digest), so
# nothing is downloaded here.
set -eu
cd "$(dirname "$0")/.."

repo="smallnest/hyla"
tap="smallnest/homebrew-hyla"

version="${1:-$(gh api "repos/$repo/releases/latest" --jq '.tag_name' | sed 's/^v//')}"
digest() {
  gh api "repos/$repo/releases/tags/v$version" \
    --jq ".assets[] | select(.name | endswith(\"-$1.dmg\")) | .digest" | sed 's/^sha256://'
}

arm=$(digest arm64)
intel=$(digest amd64)
if [ -z "$arm" ] || [ -z "$intel" ]; then
  echo "no dmgs with digests for v$version" >&2
  exit 1
fi

tmp=$(mktemp -d)
trap 'rm -rf "$tmp"' EXIT
git clone -q "https://github.com/$tap" "$tmp/tap"

python3 - "$tmp/tap/Casks/hyla.rb" "$version" "$arm" "$intel" <<'PY'
import re
import sys

path, version, arm, intel = sys.argv[1:5]
s = open(path).read()
s = re.sub(r'version "[^"]+"', f'version "{version}"', s, count=1)
s = re.sub(
    r'sha256 arm:\s+"[^"]+",\s*\n\s*intel: "[^"]+"',
    f'sha256 arm:   "{arm}",\n         intel: "{intel}"',
    s,
    count=1,
)
open(path, "w").write(s)
PY

if git -C "$tmp/tap" diff --quiet; then
  echo "cask already at $version"
  exit 0
fi
git -C "$tmp/tap" commit -qam "hyla $version"
git -C "$tmp/tap" push -q
echo "cask: hyla $version"
