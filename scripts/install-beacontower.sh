#!/bin/sh
# Installs the standalone BeaconTower encrypted relay binary on Linux or macOS
# from GitHub Releases with SHA-256 verification.
#
#   curl -fsSL https://github.com/smallnest/hyla/releases/latest/download/install-beacontower.sh | sh
#   sh install-beacontower.sh [--version 3.23.0] [--bin-dir /usr/local/bin] [--dry-run]
#   sh install-beacontower.sh --uninstall [--bin-dir DIR]
#
# The release workflow replaces @VERSION@ with the release version.
# $HYLA_RELEASES overrides the release download base URL (supports file:// mirrors).
set -eu

repo=smallnest/hyla
version='@VERSION@'

default_bin_dir() {
	if [ "$(id -u)" -eq 0 ]; then
		printf '%s\n' "/usr/local/bin"
	else
		printf '%s\n' "$HOME/.local/bin"
	fi
}

usage() {
	cat <<EOF
Usage: sh install-beacontower.sh [--version VERSION] [--bin-dir DIR] [--dry-run]
       sh install-beacontower.sh --uninstall [--bin-dir DIR]

Installs the standalone beacontower relay binary into /usr/local/bin (as root)
or ~/.local/bin (for the current user), verifying its SHA-256 checksum.
EOF
}

fail() {
	echo "install-beacontower.sh: $*" >&2
	exit 1
}

fetch() {
	if command -v curl >/dev/null 2>&1; then
		curl -fsSL "$1"
	elif command -v wget >/dev/null 2>&1; then
		wget -qO- "$1"
	else
		fail "downloading beacontower needs curl or wget"
	fi
}

sha256() {
	if command -v sha256sum >/dev/null 2>&1; then
		sha256sum "$1" | cut -d' ' -f1
	else
		shasum -a 256 "$1" | cut -d' ' -f1
	fi
}

main() {
	bin_dir=""
	dry_run=0
	do_uninstall=0
	while [ $# -gt 0 ]; do
		case "$1" in
		--version)
			[ $# -ge 2 ] || fail "--version needs a version"
			version="${2#v}"
			shift
			;;
		--version=*)
			version="${1#--version=}"
			version="${version#v}"
			;;
		--bin-dir)
			[ $# -ge 2 ] || fail "--bin-dir needs a directory"
			bin_dir="$2"
			shift
			;;
		--bin-dir=*)
			bin_dir="${1#--bin-dir=}"
			;;
		--dry-run) dry_run=1 ;;
		--uninstall) do_uninstall=1 ;;
		-h | --help)
			usage
			return
			;;
		*)
			usage >&2
			exit 2
			;;
		esac
		shift
	done

	if [ -z "$bin_dir" ]; then
		bin_dir="$(default_bin_dir)"
	fi

	if [ "$do_uninstall" = 1 ]; then
		if [ -e "$bin_dir/beacontower" ] || [ -L "$bin_dir/beacontower" ]; then
			rm -f "$bin_dir/beacontower"
			echo "Removed $bin_dir/beacontower."
		else
			echo "No beacontower binary found at $bin_dir/beacontower."
		fi
		return
	fi

	case "$(uname -s)" in
	Linux) os=linux ;;
	Darwin) os=darwin ;;
	*) fail "install-beacontower.sh is for Linux and macOS, not $(uname -s)" ;;
	esac
	case "$(uname -m)" in
	x86_64 | amd64) arch=amd64 ;;
	aarch64 | arm64) arch=arm64 ;;
	*) fail "beacontower is not built for $(uname -m)" ;;
	esac
	case "$version" in
	@*)
		version="$(fetch "https://api.github.com/repos/$repo/releases/latest" | sed -n 's/.*"tag_name": *"v\{0,1\}\([^"]*\)".*/\1/p' | head -n 1)"
		[ -n "$version" ] || fail "could not determine latest release version"
		;;
	esac
	case "$version" in
	*[!0-9A-Za-z.-]* | '') fail "$version is not a valid version" ;;
	esac

	name="beacontower-$version-$os-$arch.tar.gz"
	url="${HYLA_RELEASES:-https://github.com/$repo/releases/download}/v$version/$name"
	work="$(mktemp -d)"
	trap 'rm -rf "$work"' EXIT
	trap 'exit 1' HUP INT TERM

	echo "Downloading beacontower $version for $os/$arch"
	fetch "$url" >"$work/$name" || fail "could not download $url"
	fetch "$url.sha256" >"$work/$name.sha256" || fail "could not download $url.sha256"
	want="$(cut -d' ' -f1 <"$work/$name.sha256")"
	[ "$(sha256 "$work/$name")" = "$want" ] || fail "$name failed SHA-256 verification"

	tar -tzf "$work/$name" >"$work/names" || fail "$name is not a valid .tar.gz archive"
	if grep -Eq '^/|(^|/)\.\.(/|$)' "$work/names"; then
		fail "$name contains unsafe paths"
	fi

	if [ "$dry_run" = 1 ]; then
		echo "would install beacontower $version to $bin_dir/beacontower"
		return
	fi

	mkdir -p "$work/stage"
	tar -xzf "$work/$name" -C "$work/stage"
	[ -x "$work/stage/beacontower" ] || fail "$name does not contain an executable beacontower binary"
	"$work/stage/beacontower" -version >/dev/null || fail "downloaded beacontower binary failed health check"

	mkdir -p "$bin_dir"
	tmp_target="$bin_dir/.beacontower.new.$$"
	cp "$work/stage/beacontower" "$tmp_target"
	chmod 0755 "$tmp_target"
	mv -f "$tmp_target" "$bin_dir/beacontower"

	case ":$PATH:" in
	*":$bin_dir:"*) ;;
	*) echo "Add $bin_dir to your PATH: export PATH=\"$bin_dir:\$PATH\"" ;;
	esac

	echo
	echo "Installed $("$bin_dir/beacontower" -version) at $bin_dir/beacontower"
	echo "Quick start:"
	echo "  beacontower -listen :7474 -token your-relay-secret"
}

main "$@"
