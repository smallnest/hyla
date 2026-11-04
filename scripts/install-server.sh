#!/bin/sh
# Installs the server build of Hyla for the current user, without root, on
# Linux or macOS: hylad and the hyla command in ~/.local/lib/hyla, the
# command in ~/.local/bin, and hylad as a service of the user's (systemd's
# or launchd's), so that Hyla on another machine shows this one's sessions
# (Settings › Hosts). Windows has install-server.ps1.
#
#   curl -fsSL https://github.com/smallnest/hyla/releases/latest/download/install-server.sh | sh
#   sh install-server.sh [--version 2.1.0] [--no-service] [--dry-run]
#   sh install-server.sh --uninstall
#
# The release workflow writes the version of its release in place of
# @VERSION@. $HYLA_RELEASES names a mirror of the releases, as hyla daemon
# update takes it.
set -eu

repo=smallnest/hyla
version='@VERSION@'
lib="$HOME/.local/lib/hyla"
bin="$HOME/.local/bin"
# The file that tells an install of this script's, which updates itself.
marker=.hyla-server

usage() {
	cat <<EOF
Usage: sh install-server.sh [--version VERSION] [--no-service] [--dry-run]
       sh install-server.sh --uninstall

Installs the server build of Hyla in $lib, the hyla command in $bin, and
runs hylad as a service: systemd's on Linux, launchd's on macOS
(--no-service: hylad starts as Hyla connects). --uninstall removes them, and leaves your settings and data in
~/.config/hyla.
EOF
}

fail() {
	echo "install-server.sh: $*" >&2
	exit 1
}

fetch() {
	if command -v curl >/dev/null 2>&1; then
		curl -fsSL "$1"
	elif command -v wget >/dev/null 2>&1; then
		wget -qO- "$1"
	else
		fail "downloading Hyla needs curl or wget"
	fi
}

sha256() {
	if command -v sha256sum >/dev/null 2>&1; then
		sha256sum "$1" | cut -d' ' -f1
	else
		shasum -a 256 "$1" | cut -d' ' -f1
	fi
}

# ours reports whether a path is a link to this install's hyla.
ours() {
	[ -L "$1" ] && [ "$(readlink "$1")" = "$lib/hyla" ]
}

uninstall() {
	if [ -x "$lib/hyla" ]; then
		"$lib/hyla" daemon uninstall-service >/dev/null 2>&1 || true
		"$lib/hyla" daemon stop >/dev/null 2>&1 || true
	fi
	for link in "$bin/hyla" "$HOME/.config/hyla/bin/hyla"; do
		if ours "$link"; then
			rm -f "$link"
		fi
	done
	rm -rf "$lib"
	echo "Removed Hyla from $lib. Your settings and data stay in ~/.config/hyla."
}

main() {
	service=1
	dry_run=0
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
		--no-service) service=0 ;;
		--dry-run) dry_run=1 ;;
		--uninstall)
			uninstall
			return
			;;
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

	case "$(uname -s)" in
	Linux) os=linux ;;
	Darwin) os=darwin ;;
	*) fail "the server build of Hyla is for Linux, macOS and Windows (install-server.ps1), not $(uname -s)" ;;
	esac
	[ "$(id -u)" != 0 ] || fail "run this as the user whose sessions Hyla runs, not as root"
	case "$(uname -m)" in
	x86_64 | amd64) arch=amd64 ;;
	aarch64 | arm64) arch=arm64 ;;
	*) fail "Hyla is not made for $(uname -m)" ;;
	esac
	case "$version" in
	@*) # This script, run from the repository: the latest release.
		version="$(fetch "https://api.github.com/repos/$repo/releases/latest" | sed -n 's/.*"tag_name": *"v\{0,1\}\([^"]*\)".*/\1/p' | head -n 1)"
		[ -n "$version" ] || fail "could not find Hyla's latest release"
		;;
	esac
	case "$version" in
	*[!0-9A-Za-z.-]* | '') fail "$version is not a version of Hyla" ;;
	esac

	name="hyla-server-$version-$os-$arch.tar.gz"
	url="${HYLA_RELEASES:-https://github.com/$repo/releases/download}/v$version/$name"
	work="$(mktemp -d)"
	staging="$lib.new"
	trap 'rm -rf "$work" "$staging"' EXIT
	trap 'exit 1' HUP INT TERM

	echo "Downloading Hyla $version for $os/$arch"
	fetch "$url" >"$work/$name" || fail "could not download $url"
	fetch "$url.sha256" >"$work/$name.sha256" || fail "could not download $url.sha256"
	want="$(cut -d' ' -f1 <"$work/$name.sha256")"
	[ "$(sha256 "$work/$name")" = "$want" ] || fail "$name is not the release's: its SHA-256 differs"
	tar -tzf "$work/$name" >"$work/names" || fail "$name is not a .tar.gz archive"
	if grep -Eq '^/|(^|/)\.\.(/|$)' "$work/names"; then
		fail "$name holds files outside it"
	fi

	if [ "$dry_run" = 1 ]; then
		echo "would install Hyla $version in $lib, link $bin/hyla, and run hylad as a service: $service"
		return
	fi

	# The install of 2.x, when Hyla was Kero: its service and its files
	# go, so that one hylad serves this machine. Its settings and sessions
	# stay: Hyla moves them to ~/.config/hyla as it starts.
	oldlib="$HOME/.local/lib/kero"
	if [ -x "$oldlib/kero" ]; then
		"$oldlib/kero" daemon uninstall-service >/dev/null 2>&1 || true
		"$oldlib/kero" daemon stop >/dev/null 2>&1 || true
		rm -rf "$oldlib"
	fi
	for old in "$bin/kero" "$HOME/.config/hyla/bin/kero"; do
		if [ -L "$old" ] && [ "$(readlink "$old")" = "$oldlib/kero" ]; then
			rm -f "$old"
		fi
	done

	upgrade=0
	if [ -x "$lib/hyla" ]; then
		upgrade=1
	fi
	echo "Installing in $lib"
	rm -rf "$staging"
	mkdir -p "$staging"
	tar -xzf "$work/$name" -C "$staging"
	[ -x "$staging/hyla" ] || fail "$name holds no hyla"
	"$staging/hyla" version >/dev/null || fail "the hyla of $name does not run here"
	: >"$staging/$marker"
	# The whole directory at once, so that no file of another version stays.
	rm -rf "$lib.old"
	if [ -d "$lib" ]; then
		mv "$lib" "$lib.old"
	fi
	mkdir -p "$(dirname "$lib")"
	mv "$staging" "$lib"
	rm -rf "$lib.old"

	mkdir -p "$bin"
	if [ -e "$bin/hyla" ] || [ -L "$bin/hyla" ]; then
		if ! ours "$bin/hyla"; then
			echo "Left $bin/hyla as it is, which is not this install's. Hyla reaches this machine with the address me@host?hyla=$lib/hyla"
		fi
	else
		ln -s "$lib/hyla" "$bin/hyla"
	fi
	case ":$PATH:" in
	*":$bin:"*) ;;
	*) echo "Add $bin to your PATH to run hyla, in your shell's profile: export PATH=\"\$HOME/.local/bin:\$PATH\"" ;;
	esac

	# The service: systemd's user unit, or launchd's agent, which runs in
	# the session of the user logged in at the Mac.
	if [ "$os" = darwin ]; then
		unit="$HOME/Library/LaunchAgents/sh.hyla.hylad.plist"
		manager() { launchctl print "gui/$(id -u)" >/dev/null 2>&1; }
	else
		unit="${XDG_CONFIG_HOME:-$HOME/.config}/systemd/user/hylad.service"
		manager() { command -v systemctl >/dev/null 2>&1 && systemctl --user show-environment >/dev/null 2>&1; }
	fi
	if [ "$upgrade" = 1 ] && "$lib/hyla" daemon status 2>/dev/null | grep -q '^hylad [0-9][0-9]* running'; then
		# Never a restart unasked: it would end the programs of the sessions.
		echo "hylad runs the version of before until it restarts: hyla daemon restart (its sessions start again, the programs in them end)."
	elif [ -f "$unit" ]; then
		: # The service runs this install's hyla as it starts.
	elif [ "$service" = 1 ] && manager; then
		"$lib/hyla" daemon install-service || echo "hylad does not run as a service: Hyla starts it as it connects."
	else
		echo "hylad starts as Hyla connects. To keep it running with the machine: hyla daemon install-service"
	fi

	echo
	echo "Hyla $version is installed. In Hyla on your computer, add this machine in Settings › Hosts:"
	echo "  $(id -un)@$(uname -n)"
	echo "or from its command line: hyla hosts add $(uname -n | cut -d. -f1) $(id -un)@$(uname -n)"
}

# All of it read before any of it runs, as a download cut short could stop
# anywhere.
main "$@"
