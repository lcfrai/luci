#!/bin/sh
set -eu
docker --host=unix:///run/luci-engine/docker.sock info >/dev/null 2>&1
curl --fail --silent --max-time 3 http://127.0.0.1:8730/v1/control-plane/status |
  jq --exit-status '.product == "Luci"' >/dev/null
