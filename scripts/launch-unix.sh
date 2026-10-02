#!/usr/bin/env bash
set -euo pipefail
root=$(cd -- "$(dirname -- "$0")" && pwd)
cd "$root"
./luci &
pid=$!
trap 'kill "$pid" 2>/dev/null || true' EXIT INT TERM
for attempt in $(seq 1 30); do
 if curl --fail --silent http://127.0.0.1:8730/v1/setup/status >/dev/null; then
  if [[ "$(uname)" == Darwin ]]; then open http://127.0.0.1:8730; else xdg-open http://127.0.0.1:8730; fi
  printf '%s\n' 'Luci is running. Close this terminal or press Ctrl+C to stop.'
  wait "$pid"
  exit 0
 fi
 sleep 1
done
printf '%s\n' 'Luci could not start. Check the error above.' >&2
exit 1
