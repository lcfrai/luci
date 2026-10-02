#!/usr/bin/env bash
set -euo pipefail
root=$(cd -- "$(dirname -- "$0")/.." && pwd)
cd "$root"
mkdir -p artifacts/dev
go build -o artifacts/dev/luci-api ./control-plane/cmd/api
export LUCI_RUNTIME_DIR="$root"
if [[ "${1:-}" == "--install-runtime" ]]; then docker build -f images/sandbox/Dockerfile -t luci/sandbox:dev .; fi
artifacts/dev/luci-api >artifacts/dev/api.out.log 2>artifacts/dev/api.err.log &
service_pid=$!
trap 'kill "$service_pid" 2>/dev/null || true' EXIT INT TERM
cd desktop
if [[ ! -d node_modules ]]; then npm ci; fi
printf '%s\n' 'Luci: http://localhost:1420 — follow setup in the app.'
npm run dev
