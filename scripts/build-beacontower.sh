#!/bin/sh
# Builds the standalone encrypted BeaconTower relay server (cmd/beacontower)
# for every release platform into build/beacontower: a .zip for Windows, else
# a .tar.gz, each paired with a .sha256 checksum, plus install-beacontower.sh
# with the version from mygo.json. Needs go, jq and zip.
#
#   sh scripts/build-beacontower.sh [os/arch ...]
set -eu

version="$(jq -r .version mygo.json)"
platforms="${*:-linux/amd64 linux/arm64 darwin/amd64 darwin/arm64 windows/amd64 windows/arm64}"
out=build/beacontower
mkdir -p "$out"

sha256_of() {
	if command -v sha256sum >/dev/null 2>&1; then
		sha256sum "$1" | cut -d' ' -f1
	else
		shasum -a 256 "$1" | cut -d' ' -f1
	fi
}

for p in $platforms; do
	os="${p%/*}"
	arch="${p#*/}"
	stage="$out/$os-$arch"
	rm -rf "$stage"
	mkdir -p "$stage"
	exe=beacontower
	if [ "$os" = windows ]; then
		exe=beacontower.exe
	fi
	echo "Building beacontower $version for $os/$arch"
	CGO_ENABLED=0 GOOS="$os" GOARCH="$arch" go build -trimpath \
		-ldflags "-s -w -X main.version=$version" \
		-o "$stage/$exe" ./cmd/beacontower

	if [ "$os" = windows ]; then
		archive="beacontower-$version-$os-$arch.zip"
		rm -f "$out/$archive"
		(cd "$stage" && zip -q -X "../$archive" "$exe")
	else
		archive="beacontower-$version-$os-$arch.tar.gz"
		tar -C "$stage" -czf "$out/$archive" "$exe"
	fi
	(cd "$out" && printf '%s  %s\n' "$(sha256_of "$archive")" "$archive" >"$archive.sha256")
	rm -rf "$stage"
done

sed "s/@VERSION@/$version/" scripts/install-beacontower.sh >"$out/install-beacontower.sh"
chmod +x "$out/install-beacontower.sh"
