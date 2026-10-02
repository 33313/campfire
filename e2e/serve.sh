#!/bin/sh
# One server on a throwaway data directory, so every run starts with an
# instance that has no accounts yet.
set -eu
cd "$(dirname "$0")/.."
# Build first: the client is embedded in the binary, so serving a stale one
# silently tests the previous build.
./build.sh >/dev/null
root=$(mktemp -d)
mkdir -p "$root/campfire"
cat > "$root/campfire/campfire.json" <<JSON
{"addr":"127.0.0.1:8791","database":"$root/campfire/campfire.db",
 "snapshots":{"enabled":false},"mcp":{"enabled":false},
 "liveUpdates":{"hiddenTabGrace":"2s"}}
JSON
XDG_DATA_HOME="$root" exec ./campfire
