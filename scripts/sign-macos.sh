#!/bin/sh
# sign-macos.sh gives this Mac the certificate that signs Hyla's app for
# a release, from the repository's secrets, so that `go tool mygo build`
# signs the app as the released one does. Run it once, on the Mac that
# builds, after gen-signing-cert.sh made the certificate:
#
#   MYGO_SIGNING_P12="$(base64 < scripts/signing/hyla-signing.p12)" \
#   MYGO_SIGNING_KEY_PASSWORD=<the password gen-signing-cert.sh printed> \
#   scripts/sign-macos.sh
#
# It makes the keychain .signing/hyla-signing.keychain with the
# certificate and its key, where codesign finds it, so that `go tool
# mygo build` signs Hyla.app with it as the Release workflow does.
#
# macos.signingIdentity in mygo.json names the certificate, "Hyla
# Release Signing": codesign finds only that name in the app's. Run this
# script, whose keychain is the login keychain's default, and that name
# alone makes codesign search it. With other keychains, give codesign a
# file with macos.signingIdentity's name in it, and the keychain's path
# as "keychain", as codesign(1)'s "Other Options" says.
#
# Run it again only after gen-signing-cert.sh made a new certificate.
set -eu

cd "$(dirname "$0")/.."
keychain="$PWD/.signing/hyla-signing.keychain"
p12="$(printf '%s' "${MYGO_SIGNING_P12:-}" | tr -d '\n')"
pw="${MYGO_SIGNING_KEY_PASSWORD:-}"

if [ -z "$p12" ] || [ -z "$pw" ]; then
	echo "Set MYGO_SIGNING_P12 and MYGO_SIGNING_KEY_PASSWORD" >&2
	exit 1
fi
if [ -e "$keychain" ]; then
	echo "$keychain exists: remove it to sign with a new certificate" >&2
	exit 1
fi

mkdir -p "$(dirname "$keychain")"
printf '%s' "$p12" | base64 -D > "$keychain.p12"
security create-keychain -p "$pw" "$keychain"
security unlock-keychain -p "$pw" "$keychain"
security import "$keychain.p12" -k "$keychain" -P "$pw" -T /usr/bin/codesign -A >/dev/null
rm -f "$keychain.p12"
# The partition list lets codesign use the key without asking. It names
# codesign, so other programs still cannot use the key after a sleep.
security unlock-keychain -p "$pw" "$keychain"
security set-key-partition-list -S apple-tool:,apple:,codesign: -s -k "$pw" "$keychain" >/dev/null
# The keychain locks after 5 idle minutes and when the Mac sleeps, as the
# login keychain does. codesign signs many files and keeps the keychain
# unlocked while it runs, so a build needs no password.
security set-keychain-settings -l -u -t 300 "$keychain"
# The keychain belongs to the user's keychain search, with the login
# keychain after it: what codesign finds there, it finds by the name in
# mygo.json, and the login keychain's secrets stay where they are.
rest="$(security list-keychains -d user | sed -n 's/^ *"\(.*\)"$/\1/p' | grep -v -F -x "$keychain")"
# shellcheck disable=SC2086: the paths have no blanks.
security list-keychains -d user -s "$keychain" $rest >/dev/null
security default-keychain -d user -s "$keychain" >/dev/null

echo 'Hyla signs with "Hyla Release Signing", the name in mygo.json'"'"'s macos.signingIdentity:'
echo '  go tool mygo build -platform darwin/arm64 -skip-notarize'
echo 'Give back the login keychain as the default when you are done:'
echo '  security default-keychain -d user -s login.keychain-db'
