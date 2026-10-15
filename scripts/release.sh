#!/bin/sh
# Builds the release archives: the interface once, then one static binary per
# platform, each packed with the install files and the documentation.
#
#   scripts/release.sh 1.0.0
#
# → dist/solstice-1.0.0-<os>-<arch>.tar.gz (.zip for Windows) and SHA256SUMS.
set -eu

version=${1:?usage: scripts/release.sh VERSION}
root=$(cd "$(dirname "$0")/.." && pwd)
dist="$root/dist"

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
ls -l "$dist"
