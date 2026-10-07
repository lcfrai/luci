#!/bin/sh
set -eu
luci_directory=$(CDPATH= cd -- "$(dirname -- "$0")" && pwd)
luci_port=${LUCI_PORT:-8730}
luci_preview_port=${LUCI_PREVIEW_PORT:-8731}
luci_open_browser=1
if [ "${1:-}" = --no-browser ]; then luci_open_browser=0; shift; fi
if [ "$#" -ne 0 ]; then printf '%s\n' 'Usage: ./start-luci.sh [--no-browser]' >&2; exit 2; fi
for luci_value in "$luci_port" "$luci_preview_port"; do
  case "$luci_value" in ''|*[!0-9]*) printf '%s\n' 'Luci ports must be numbers.' >&2; exit 2;; esac
  if [ "$luci_value" -lt 1024 ] || [ "$luci_value" -gt 65535 ]; then
    printf '%s\n' 'Choose Luci ports between 1024 and 65535.' >&2; exit 2
  fi
done
if [ "$luci_port" = "$luci_preview_port" ]; then printf '%s\n' 'The app and preview ports must differ.' >&2; exit 2; fi
if ! command -v docker >/dev/null 2>&1; then
  printf '%s\n' 'Install and open Docker Desktop, then run this launcher again.' 'https://docs.docker.com/desktop/' >&2
  exit 1
fi
if ! docker compose version >/dev/null 2>&1; then printf '%s\n' 'Docker Compose is required. Update Docker Desktop or install the official Compose plugin.' >&2; exit 1; fi
if [ "$(docker info --format '{{.OSType}}' 2>/dev/null || true)" != linux ]; then
  printf '%s\n' 'Open Docker Desktop and select Linux containers, or start your local Linux Docker engine, then retry.' >&2
  exit 1
fi
export LUCI_PORT="$luci_port" LUCI_PREVIEW_PORT="$luci_preview_port"
printf '%s\n' 'Preparing Luci. The first launch downloads its pinned tools and private engine; subsequent launches reuse them.'
docker compose -f "$luci_directory/compose.yaml" up --build --detach --wait --wait-timeout 240
luci_url="http://127.0.0.1:$luci_port/"
luci_bootstrap=$(docker compose -f "$luci_directory/compose.yaml" exec -T controller sh -c 'if [ -r /luci-workspace/state/control-plane/bootstrap-token ]; then cat /luci-workspace/state/control-plane/bootstrap-token; fi')
case "$luci_bootstrap" in
  *[!A-Za-z0-9_-]*) printf '%s\n' 'The first-account token has an unexpected format. Check the controller logs.' >&2; exit 1;;
esac
if [ -n "$luci_bootstrap" ]; then
  if [ "${#luci_bootstrap}" -ne 43 ]; then printf '%s\n' 'The first-account token has an unexpected length.' >&2; exit 1; fi
  luci_url="$luci_url#bootstrap=$luci_bootstrap"
fi
if [ "$luci_open_browser" -eq 1 ]; then
  if command -v open >/dev/null 2>&1; then open "$luci_url"
  elif command -v xdg-open >/dev/null 2>&1; then xdg-open "$luci_url" >/dev/null 2>&1 &
  else printf '%s\n' 'No browser launcher was found. Run this launcher on a desktop to open the first-account link.'; fi
fi
unset luci_bootstrap luci_url
printf 'Luci is running at http://127.0.0.1:%s/\n' "$luci_port"
printf '%s\n' 'Your account, projects and private engine data are retained when the app restarts.'
