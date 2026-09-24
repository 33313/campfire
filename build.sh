#!/bin/sh
# Build the client into internal/web/app, then the binary that embeds it.
#
#   ./build.sh                    ./campfire, for this machine
#   ./build.sh release <version>  dist/: every platform a release ships, and SHA256SUMS
#
# Release binaries are named without the version, so the newest one is always at
# https://github.com/<repo>/releases/latest/download/campfire-<os>-<arch>.
set -eu
cd "$(dirname "$0")"
if [ -f package-lock.json ] && [ ! -d node_modules ]; then npm ci; fi
npm run build

if [ "${1:-}" != release ]; then
  go build -o campfire .
  echo "built ./campfire"
  exit 0
fi

version=${2:?usage: ./build.sh release <version>}
rm -rf dist
mkdir dist
# CGO off: the SQLite driver is pure Go, so every target is a static binary
# cross-compiled from here.
for target in linux/amd64 linux/arm64 darwin/amd64 darwin/arm64 windows/amd64 windows/arm64; do
  os=${target%/*}
  arch=${target#*/}
  ext=
  if [ "$os" = windows ]; then ext=.exe; fi
  CGO_ENABLED=0 GOOS=$os GOARCH=$arch go build -trimpath \
    -ldflags "-s -w -X main.version=$version" -o "dist/campfire-$os-$arch$ext" .
done
(cd dist && sha256sum -- * > SHA256SUMS)
echo "built $version into dist/"
