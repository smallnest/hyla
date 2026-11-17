#!/bin/sh
# gen-signing-cert.sh makes the certificate that signs Hyla's app for
# releases, with no Apple developer account: a self-signed code signing
# certificate and its key, which macOS records Hyla's permissions by.
# Hyla's updates are signed with another key, which this certificate has
# nothing to do with.
#
# The app signed with it is not notarized, and Gatekeeper asks the user
# to open it once per version, as it asks for an ad-hoc signed one. What
# it gives is an identity that lasts across versions: ad-hoc signing
# makes every build a new app to macOS, which forgets where the user let
# Hyla capture the screen and drive other apps, and a certificate the
# same every release keeps them.
#
# Needs macOS, whose openssl and security it uses. Run it once, from the
# repository's root:
#
#   scripts/gen-signing-cert.sh
#
# It writes scripts/signing/:
#
#   hyla-signing.p12   the certificate with its key, for the secrets
#                      MYGO_SIGNING_P12 and MYGO_SIGNING_KEY_PASSWORD
#   hyla-signing.crt   the certificate alone, which every Mac that runs
#                      Hyla trusts once
#
# It prints the password, the secrets the Release workflow needs, and
# what to do on the Mac that builds and on each Mac that runs Hyla. The
# p12 holds the private key: keep it out of the repository (the script
# ignores its directory in .gitignore) and delete it once the secrets
# have it.
set -eu

cd "$(dirname "$0")" && cd ..
out=scripts/signing
name="Hyla Release Signing"
pw="$(openssl rand -hex 24)"

if [ -e "$out" ]; then
	echo "$PWD/$out exists already: move it away to make a new certificate" >&2
	exit 1
fi
# The password again, in a file only its owner reads: the output can be
# lost to a scroll, the secrets need it.
umask 077
printf '%s\n' "$pw" > "$out.pw"
mkdir -p "$out"

tmp="$(mktemp -d)"
trap 'rm -rf "$tmp"' EXIT
cat > "$tmp/cert.cnf" <<EOF
[ req ]
distinguished_name = dn
x509_extensions = ext
prompt = no
[ dn ]
CN = $name
[ ext ]
basicConstraints = critical,CA:false
keyUsage = critical,digitalSignature
extendedKeyUsage = critical,codeSigning
subjectKeyIdentifier = hash
authorityKeyIdentifier = keyid:always
EOF
# A certificate for signing code alone, which macOS accepts as a
# signature's authority once a Mac trusts it.
openssl req -x509 -newkey rsa:2048 -nodes -keyout "$tmp/key.pem" -out "$out/hyla-signing.crt" \
	-days 3650 -config "$tmp/cert.cnf" 2>/dev/null
# -legacy writes the p12 as security imports it.
openssl pkcs12 -export -inkey "$tmp/key.pem" -in "$out/hyla-signing.crt" \
	-out "$out/hyla-signing.p12" -passout "pass:$pw" -name "$name" -legacy

grep -qxF "$out/" .gitignore 2>/dev/null || echo "$out/" >> .gitignore

cat <<EOF

Made the certificate "$name" in $PWD/$out.

On the Mac that builds, run scripts/sign-macos.sh with it, once:

  MYGO_SIGNING_P12="\$(base64 < $out/hyla-signing.p12)" \\
  MYGO_SIGNING_KEY_PASSWORD=$pw \\
  scripts/sign-macos.sh

Put the same two in the repository's secrets, which the Release
workflow signs the app with:

  MYGO_SIGNING_P12            \$(base64 < $out/hyla-signing.p12)
  MYGO_SIGNING_KEY_PASSWORD   $pw

On each Mac that runs Hyla, trust the certificate once, as an
administrator:

  sudo security add-trusted-cert -d -r trustRoot -k /Library/Keychains/System.keychain $out/hyla-signing.crt

Then grant Hyla Screen Recording and Accessibility in System Settings ›
Privacy & Security, once: the versions after it keep them, where
ad-hoc signed versions lose them with every update.

$out/hyla-signing.p12 holds the private key, and $out.pw the password.
Once their base64 and password are in the secrets, delete them: anyone
with them signs code as Hyla.
EOF
