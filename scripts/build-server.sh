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

# fetch_lib asks for the file, the library to carry, again a moment later
# each time, at once first.
fetch_lib() {
	for d in 0 1 2 4 8 12; do
		if [ "$d" != 0 ]; then
			sleep "$d"
		fi
		curl -fsSL "$(echo "$file" | jq -r .url)" -o "$stage/$lib" || continue
		if echo "$(echo "$file" | jq -r .sha256)  $stage/$lib" | check; then
			return 0
		fi
	done
	return 1
}

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
	# GitHub's server answers the link of the release with 502 and 504 now and
	# then: the library is asked for again, and, failing that, where the
	# terminal keeps one it downloaded, then by mygo, which downloads one that
	# its program finds missing as the program runs, and checks its SHA-256:
	# that program is built for this machine, and runs on Windows not, a
	# download that failed there failing the build.
	if ! fetch_lib; then
		cached="$(go env GOCACHE)/fetch-lib/$lib"
		if [ -n "$(go env GOCACHE)" ] && echo "$(echo "$file" | jq -r .sha256)  $cached" | check; then
			cp "$cached" "$stage/$lib"
		elif [ "$os" = windows ]; then
			echo "cannot download $lib" >&2
			exit 1
		else
			if [ -n "$(go env GOCACHE)" ]; then
				mkdir -p "$(go env GOCACHE)/fetch-lib"
				MYGO_GHOSTTY_VT="$cached" go run ./scripts/fetch-lib
				cp "$cached" "$stage/$lib"
			else
				MYGO_GHOSTTY_VT="$stage/$lib" go run ./scripts/fetch-lib
			fi
		fi
	fi
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
