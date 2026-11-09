#!/bin/sh
# Builds the server build of Hyla as releases publish it, into build/server:
# for each platform (update.Platforms), an archive of hyla, libghostty-vt,
# on Windows the ConPTY of resources/windows-*, and the marker of an
# install that updates itself (update.Marker), a .zip for Windows, else a
# .tar.gz, named as update.Asset names it; and the install scripts, with
# the version of mygo.json in them. It needs go, jq, curl and zip.
#
#   sh scripts/build-server.sh [os/arch ...]
set -eu

version="$(jq -r .version mygo.json)"
platforms="${*:-linux/amd64 linux/arm64 darwin/amd64 darwin/arm64 windows/amd64 windows/arm64}"
out=build/server
mkdir -p "$out"

go mod download github.com/egoist/mygo
manifest="$(go list -m -f '{{.Dir}}' github.com/egoist/mygo)/plugins/terminal/mygo-plugin.json"

check() {
	if command -v sha256sum >/dev/null 2>&1; then
		sha256sum -c - >/dev/null
	else
		shasum -a 256 -c - >/dev/null
	fi
}

for p in $platforms; do
	os="${p%/*}"
	arch="${p#*/}"
	stage="$out/$os-$arch"
	rm -rf "$stage"
	mkdir -p "$stage"
	exe=hyla
	if [ "$os" = windows ]; then
		exe=hyla.exe
	fi
	echo "Building Hyla $version for $os/$arch"
	CGO_ENABLED=0 GOOS="$os" GOARCH="$arch" go build -trimpath \
		-ldflags "-s -w -X main.version=$version -X github.com/egoist/mygo.production=1" \
		-o "$stage/$exe" ./cmd/hyla-server
	# libghostty-vt, which the archive carries, as the app does: hyla finds
	# it beside itself, and downloads none.
	file="$(jq -c ".libraries[] | select(.name == \"libghostty-vt\") | .files[\"$os-$arch\"]" "$manifest")"
	lib="$(echo "$file" | jq -r .name)"
	# GitHub's server may answer the link of the release with 500 of a kind,
	# as 504 Gateway Timeout: asked for again, a moment later.
	for i in 1 2 3 4 5; do
		if curl -fsSL --retry 3 --retry-delay 5 "$(echo "$file" | jq -r .url)" -o "$stage/$lib"; then
			break
		fi
		if [ "$i" = 5 ]; then
			echo "cannot download $lib" >&2
			exit 1
		fi
		echo "$lib: no answer, asking again" >&2
		sleep $((i * 10))
	done
	echo "$(echo "$file" | jq -r .sha256)  $stage/$lib" | check
	# On Windows, the ConPTY the app ships too, which hylad finds beside
	# itself (resources/windows-*).
	extra=
	if [ "$os" = windows ]; then
		cp "resources/$os-$arch/conpty.dll" "resources/$os-$arch/OpenConsole.exe" "resources/$os-$arch/ConPTY-LICENSE.txt" "$stage/"
		extra="conpty.dll OpenConsole.exe ConPTY-LICENSE.txt"
	fi
	: >"$stage/.hyla-server"
	if [ "$os" = windows ]; then
		rm -f "$out/hyla-server-$version-$os-$arch.zip"
		(cd "$stage" && zip -q -X "../hyla-server-$version-$os-$arch.zip" "$exe" "$lib" $extra .hyla-server)
	else
		tar -C "$stage" -czf "$out/hyla-server-$version-$os-$arch.tar.gz" "$exe" "$lib" .hyla-server
	fi
done

sed "s/@VERSION@/$version/" scripts/install-server.sh >"$out/install-server.sh"
sed "s/@VERSION@/$version/" scripts/install-server.ps1 >"$out/install-server.ps1"
