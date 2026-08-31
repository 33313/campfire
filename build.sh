#!/bin/sh
# Build the client into internal/web/app, then the binary that embeds it.
set -eu
cd "$(dirname "$0")"
if [ -f package-lock.json ] && [ ! -d node_modules ]; then npm ci; fi
npm run build
go build -o campfire .
echo "built ./campfire"
