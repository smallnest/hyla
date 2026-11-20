#!/bin/sh
# Builds and packs first-party Hyla plugins under plugins/ into build/plugins:
# for each compiled plugin and platform, <id>-<version>-<os>-<arch>.hylapkg
# and its .sha256 checksum; declarative plugins produce <id>-<version>.hylapkg.
#
#   sh scripts/build-plugins.sh [os/arch ...]
set -eu

if [ "$#" -gt 0 ]; then
	platforms=""
	for p in "$@"; do
		if [ -n "$platforms" ]; then
			platforms="$platforms,$p"
		else
			platforms="$p"
		fi
	done
	exec go run ./scripts/build-plugins -platforms "$platforms"
fi
exec go run ./scripts/build-plugins
