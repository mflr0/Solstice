#!/bin/sh
# Builds the release archives: the interface once, then one static binary per
# platform, each packed with the install files and the documentation.
#
#   SOLSTICE_RELEASE_KEY=/path/to/release-key.pem scripts/release.sh 1.0.0
#
# → dist/solstice-1.0.0-<os>-<arch>.tar.gz (.zip for Windows), SHA256SUMS and
# SHA256SUMS.sig, an Ed25519 signature of SHA256SUMS. Publish all of them.
#
# Signing. The update helper (deploy/linux/update.sh) installs a release only
# if SHA256SUMS.sig verifies against the public key written in the helper, so
# a release without it cannot be installed from the interface. The private
# key is a file outside the repository, named by SOLSTICE_RELEASE_KEY; this
# script refuses to run without it. Once, on a trusted machine:
#
#   umask 077
#   openssl genpkey -algorithm ed25519 -out ~/solstice-release-key.pem
#   openssl pkey -in ~/solstice-release-key.pem -pubout
#
# Paste the printed "-----BEGIN PUBLIC KEY-----" block over the placeholder
# release_pubkey in deploy/linux/update.sh, commit that (the public key is not
# a secret), and keep the private key and a backup of it off the repository
# and off GitHub. Needs OpenSSL 1.1.1 or later. Lost key: generate a new pair
# and ship a new update.sh through install.sh; installed helpers keep
# trusting the old key until they are reinstalled.
#
# deploy/linux/test-update-signature.sh checks the verification with a
# throwaway key.
set -eu

version=${1:?usage: scripts/release.sh VERSION}
root=$(cd "$(dirname "$0")/.." && pwd)
dist="$root/dist"

key=${SOLSTICE_RELEASE_KEY:-}
if [ -z "$key" ]; then
  echo "SOLSTICE_RELEASE_KEY must name the Ed25519 private key that signs the release (see the top of this file)." >&2
  exit 1
fi
if [ ! -f "$key" ] || [ ! -r "$key" ]; then
  echo "SOLSTICE_RELEASE_KEY: cannot read $key." >&2
  exit 1
fi
if ! command -v openssl >/dev/null 2>&1; then
  echo "openssl is required to sign the release." >&2
  exit 1
fi
if ! openssl pkey -in "$key" -noout -text 2>/dev/null | grep -q 'ED25519'; then
  echo "SOLSTICE_RELEASE_KEY: $key is not an Ed25519 private key (or this openssl lacks Ed25519)." >&2
  exit 1
fi

rm -rf "$dist"
mkdir -p "$dist"

echo "› interface"
(cd "$root/frontend" && pnpm install --frozen-lockfile && pnpm build)

for target in linux/amd64 linux/arm64 darwin/amd64 darwin/arm64 windows/amd64; do
  os=${target%/*}
  arch=${target#*/}
  name="solstice-$version-$os-$arch"
  dir="$dist/$name"
  ext=""
  [ "$os" = windows ] && ext=.exe
  echo "› $name"
  mkdir -p "$dir"
  (cd "$root/backend" && CGO_ENABLED=0 GOOS=$os GOARCH=$arch go build -trimpath \
    -ldflags "-s -w -X main.version=$version" -o "$dir/solstice$ext" ./cmd/solstice)
  cp "$root/README.md" "$dir/"
  [ -f "$root/LICENSE" ] && cp "$root/LICENSE" "$dir/"
  [ -d "$root/docs" ] && cp -R "$root/docs" "$dir/docs"
  if [ "$os" = linux ]; then
    # The installer, the service, and the update helper it sets up.
    cp "$root/deploy/linux/install.sh" "$root/deploy/linux/solstice.service" \
      "$root/deploy/linux/update.sh" "$root/deploy/linux/solstice-update.path" \
      "$root/deploy/linux/solstice-update.service" "$dir/"
  fi
  if [ "$os" = windows ]; then
    (cd "$dist" && zip -qr "$name.zip" "$name")
  else
    (cd "$dist" && tar -czf "$name.tar.gz" "$name")
  fi
  rm -rf "$dir"
done

cd "$dist"
if command -v sha256sum >/dev/null 2>&1; then
  sha256sum solstice-* > SHA256SUMS
else
  shasum -a 256 solstice-* > SHA256SUMS
fi

# The signature covers SHA256SUMS, which names every archive. Checked at once
# with the public half of the key, so a wrong key stops the release here.
openssl pkeyutl -sign -inkey "$key" -rawin -in SHA256SUMS -out SHA256SUMS.sig
openssl pkey -in "$key" -pubout -out "$dist/.release.pub"
openssl pkeyutl -verify -pubin -inkey "$dist/.release.pub" -rawin -in SHA256SUMS -sigfile SHA256SUMS.sig >/dev/null
rm -f "$dist/.release.pub"
ls -l "$dist"
