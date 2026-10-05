#!/bin/sh
set -eu
if [ "${1:-}" != dockerd ]; then
  printf '%s\n' 'Start the private engine with its explicit dockerd command.' >&2
  exit 1
fi
/usr/local/bin/workspace-init.sh
exec /usr/local/bin/dockerd-entrypoint.sh "$@"
