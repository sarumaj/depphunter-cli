#!/usr/bin/env bash
# Cross-compiles release archives into dist/: one per target, plus checksums.txt.
#
#   scripts/dist.sh [version]            # every target, version defaults to "dev"
#   TARGETS="linux/amd64" scripts/dist.sh v1.2.3
#
# depphunter is pure Go (no cgo), so every target builds from any host. Windows
# archives are zip files, all others gzipped tarballs; each holds the binary, README,
# LICENSE and the licenses of the vendored web libraries embedded in the binary.
set -euo pipefail

version=${1:-dev}
targets=${TARGETS:-"
  linux/amd64 linux/arm64 linux/armv7 linux/386 linux/riscv64
  darwin/amd64 darwin/arm64
  windows/amd64 windows/arm64 windows/386
  freebsd/amd64 freebsd/arm64
"}

cd "$(dirname "$0")/.."
rm -rf dist
mkdir dist

for target in $targets; do
  goos=${target%/*} arch=${target#*/}
  goarch=$arch goarm=""
  if [ "$arch" = armv7 ]; then goarch=arm goarm=7; fi
  name="depphunter_${version#v}_${goos}_${arch}"
  ext=""; [ "$goos" = windows ] && ext=.exe
  echo "building $name"
  mkdir -p "dist/$name/licenses"
  CGO_ENABLED=0 GOOS=$goos GOARCH=$goarch GOARM=$goarm go build -trimpath \
    -ldflags "-s -w -X main.version=$version" -o "dist/$name/depphunter$ext" ./cmd/depphunter
  cp README.md LICENSE "dist/$name/"
  cp web/static/vendor/*.LICENSE "dist/$name/licenses/"
  if [ "$goos" = windows ]; then
    (cd dist && zip -qr "$name.zip" "$name")
  else
    tar -C dist -czf "dist/$name.tar.gz" "$name"
  fi
  rm -r "dist/$name"
done

(cd dist && sha256sum -- * > checksums.txt)
