#!/usr/bin/env bash
set -euo pipefail

# COMMAND selects the harness. Defaults to a plain shell so a sandbox is
# useful even when no agent is requested.
COMMAND="${PANACEA_APP_COMMAND:-/bin/bash}"

# ARGS_JSON is a JSON array of strings. It is preferred over the legacy
# comma-separated ARGS because an opening prompt routinely contains a comma.
ARGS_JSON="${PANACEA_APP_ARGS_JSON:-}"
ARGS="${PANACEA_APP_ARGS:-}"

exec /usr/local/bin/sandboxd \
  -addr :8080 \
  -command "${COMMAND}" \
  -args "${ARGS}" \
  -args-json "${ARGS_JSON}"
