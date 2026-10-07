#!/bin/sh
set -eu
umask 077

if [ "$(id -u):$(id -g)" != 1001:1001 ]; then
  printf '%s\n' 'The Luci controller must run as its private non-root user 1001:1001.' >&2
  exit 1
fi
for luci_directory in /luci-workspace /luci-workspace/home /luci-workspace/state /luci-workspace/projects /luci-workspace/tmp; do
  if [ -L "$luci_directory" ] || [ ! -d "$luci_directory" ] ||
     [ "$(stat -c '%u:%g:%a' "$luci_directory")" != 1001:1001:700 ]; then
    printf '%s\n' 'The private engine has not prepared the Luci workspace for user 1001. Retained files have not been changed; inspect the engine startup error.' >&2
    exit 1
  fi
done
if [ -L /run/luci-engine ] || [ ! -d /run/luci-engine ]; then
  printf '%s\n' 'Luci refused an unavailable or linked private engine directory.' >&2
  exit 1
fi

luci_attempt=0
until docker --host=unix:///run/luci-engine/docker.sock info >/dev/null 2>&1; do
  luci_attempt=$((luci_attempt + 1))
  if [ "$luci_attempt" -ge 60 ]; then
    printf '%s\n' 'The private Docker engine did not become ready. Check the engine service logs, then restart Luci.' >&2
    exit 1
  fi
  sleep 1
done
exec /opt/luci/luci "$@"
