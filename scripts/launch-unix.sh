#!/usr/bin/env bash
set -euo pipefail
root=$(cd -- "$(dirname -- "$0")" && pwd)
cd "$root"
export LUCI_STATE_DIR="${LUCI_STATE_DIR:-${XDG_STATE_HOME:-$HOME/.local/state}/luci}"
./luci &
pid=$!
trap 'kill "$pid" 2>/dev/null || true' EXIT INT TERM
for attempt in $(seq 1 30); do
 if curl --fail --silent http://127.0.0.1:8730/v1/control-plane/status >/dev/null; then
  luci_url=http://127.0.0.1:8730
  if [[ -f "$LUCI_STATE_DIR/control-plane/bootstrap-token" ]]; then
   luci_code=$(tr -d '\r\n' < "$LUCI_STATE_DIR/control-plane/bootstrap-token")
   if [[ "$luci_code" =~ ^[A-Za-z0-9_-]{43}$ ]]; then luci_url="${luci_url}/#bootstrap=${luci_code}"; fi
  fi
  if [[ "$(uname)" == Darwin ]]; then open "$luci_url"; else xdg-open "$luci_url"; fi
  printf '%s\n' 'Luci is running. Close this terminal or press Ctrl+C to stop.'
  wait "$pid"
  exit 0
 fi
 sleep 1
done
printf '%s\n' 'Luci could not start. Check the error above.' >&2
exit 1
