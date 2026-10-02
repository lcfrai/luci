#!/usr/bin/env bash
set -euo pipefail
luci_root=$(cd -- "$(dirname -- "$0")" && pwd)
cd "$luci_root"
luci_port=8730
luci_no_browser=0
luci_state="${LUCI_STATE_DIR:-${XDG_STATE_HOME:-$HOME/.local/state}/luci}"
while (($#)); do
 case "$1" in
  --port) luci_port="${2:?Supply a port}"; shift 2 ;;
  --state-dir) luci_state="${2:?Supply an absolute state directory}"; shift 2 ;;
  --no-browser) luci_no_browser=1; shift ;;
  *) printf '%s\n' 'Use --port NUMBER, --state-dir PATH or --no-browser.' >&2; exit 2 ;;
 esac
done
if [[ ! "$luci_port" =~ ^[0-9]{1,5}$ ]] || ((10#$luci_port < 1 || 10#$luci_port > 65535)); then
 printf '%s\n' 'Choose a port from 1 to 65535.' >&2; exit 2
fi
if [[ "$luci_state" != /* ]]; then printf '%s\n' 'Choose an absolute state directory.' >&2; exit 2; fi
if [[ ! -x ./luci ]]; then printf '%s\n' 'Extract the complete Luci archive, then open start-luci.sh again.' >&2; exit 1; fi
umask 077
mkdir -p "$luci_state"
export LUCI_STATE_DIR="$luci_state" LUCI_ADDR="127.0.0.1:$luci_port"
export LUCI_PORTABLE_OWNER_ACCESS=1 LUCI_LOCAL_ACCESS_TOKEN='' LUCI_ALLOW_PUBLIC_BIND='' LUCI_WORKER=''
export LUCI_WEB_DIR="$luci_root/web" LUCI_RUNTIME_DIR="$luci_root/runtime" LUCI_CLOUD_BUNDLE="$luci_root/cloud/luci-cloud-linux-amd64.tar.gz"
luci_lifecycle=$(od -An -N32 -tx1 /dev/urandom | tr -d ' \r\n')
export LUCI_MANAGED_SHUTDOWN_TOKEN="$luci_lifecycle"
luci_base="http://127.0.0.1:$luci_port"
luci_pid=''
luci_cleaned=0
luci_private_request() {
 # Credentials enter curl through stdin, not its arguments or logs.
 printf 'header = "%s: %s"\n' "$1" "$2" | curl --config - --fail --silent --show-error --max-time 5 "${@:3}"
}
luci_cleanup() {
 if ((luci_cleaned)); then return; fi
 luci_cleaned=1
 if [[ -n "$luci_pid" ]] && kill -0 "$luci_pid" 2>/dev/null; then
  luci_private_request X-Luci-Lifecycle-Token "$luci_lifecycle" --request POST --data '' "$luci_base/v1/control-plane/shutdown" >/dev/null 2>&1 || true
  for ((luci_wait=0; luci_wait<50; luci_wait++)); do
   if ! kill -0 "$luci_pid" 2>/dev/null; then break; fi
   sleep 0.1
  done
  if kill -0 "$luci_pid" 2>/dev/null; then kill "$luci_pid" 2>/dev/null || true; fi
  wait "$luci_pid" 2>/dev/null || true
 fi
}
trap luci_cleanup EXIT INT TERM
if ! curl --fail --silent --max-time 2 "$luci_base/v1/control-plane/status" >/dev/null; then
 ./luci >"$luci_state/service.out.log" 2>"$luci_state/service.err.log" &
 luci_pid=$!
fi
luci_ready=0
for ((luci_attempt=0; luci_attempt<30; luci_attempt++)); do
 if [[ -n "$luci_pid" ]] && ! kill -0 "$luci_pid" 2>/dev/null; then
  printf 'Luci could not start. Read %s\n' "$luci_state/service.err.log" >&2; exit 1
 fi
 if curl --fail --silent --max-time 2 "$luci_base/v1/control-plane/status" >/dev/null; then luci_ready=1; break; fi
 sleep 1
done
if ((!luci_ready)); then printf 'Luci could not start. Read %s\n' "$luci_state/service.err.log" >&2; exit 1; fi
luci_owner_path="$luci_state/control-plane/owner-access-token"
if [[ ! -f "$luci_owner_path" || -L "$luci_owner_path" ]]; then
 printf '%s\n' 'An older or different Luci workspace is already running at this address. Close its launcher, then start this version again.' >&2; exit 1
fi
luci_owner_code=$(head -c 128 "$luci_owner_path" | tr -d '\r\n')
if [[ ! "$luci_owner_code" =~ ^[A-Za-z0-9_-]{43}$ ]] || ! luci_private_request X-Luci-Local-Access-Token "$luci_owner_code" "$luci_base/v1/auth/local-owner" >/dev/null 2>&1; then
 printf '%s\n' 'The running workspace does not belong to this launcher. Close its launcher or choose a different --port; your saved data has not been reset.' >&2; exit 1
fi
luci_fragment="owner=$luci_owner_code"
luci_bootstrap_path="$luci_state/control-plane/bootstrap-token"
if [[ -f "$luci_bootstrap_path" && ! -L "$luci_bootstrap_path" ]]; then
 luci_bootstrap_code=$(head -c 128 "$luci_bootstrap_path" | tr -d '\r\n')
 if [[ "$luci_bootstrap_code" =~ ^[A-Za-z0-9_-]{43}$ ]]; then luci_fragment="$luci_fragment&bootstrap=$luci_bootstrap_code"; fi
fi
if ((!luci_no_browser)); then
 if [[ "$(uname)" == Darwin ]]; then open "$luci_base/#$luci_fragment"; else xdg-open "$luci_base/#$luci_fragment"; fi
fi
printf 'Luci is running at %s\n' "$luci_base"
if [[ -n "$luci_pid" ]]; then
 printf '%s\n' 'Keep this launcher open. Press Ctrl+C here to stop Luci.'
 wait "$luci_pid"
else
 printf '%s\n' 'This workspace was already running; closing this launcher will leave it running.'
fi
