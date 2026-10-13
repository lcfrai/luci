#!/bin/bash -p
set -euo pipefail

# Only the control plane's fixed terminal or trusted microVM guest home may
# survive UID bootstrap. Other harnesses keep the normal home; arbitrary HOME
# values are ignored.
luci_bootstrap_home() {
  if [[ "${LUCI_MICROVM_FIXED_HOME:-}" == "1" ]]; then
    printf '%s' '/home/dev/.luci-agent-home'
  elif [[ "${LUCI_COMMAND:-/bin/bash}" == "/bin/bash" && "${HOME:-}" == "/home/dev/.luci-shell" ]]; then
    printf '%s' '/home/dev/.luci-shell'
  else
    printf '%s' '/home/dev'
  fi
}

# Linux bind mounts retain their host owner and 0700/0600 modes. The backend
# supplies these reserved IDs from its own process, never from project data.
# bash -p ignores BASH_ENV and imported shell functions during this bootstrap;
# absolute commands avoid executing anything from a mounted project's PATH.
if [[ "$(/usr/bin/id -u)" == "0" ]]; then
  host_uid="${LUCI_HOST_UID:-1001}"
  host_gid="${LUCI_HOST_GID:-1001}"
  if [[ ! "$host_uid" =~ ^[1-9][0-9]{0,9}$ || ! "$host_gid" =~ ^[1-9][0-9]{0,9}$ ]] \
     || (( host_uid >= 4294967295 || host_gid >= 4294967295 )); then
    printf '%s\n' 'Luci requires an unprivileged numeric host UID/GID.' >&2
    exit 2
  fi
  # Do not use usermod's recursive home chown: ~/.claude/projects is a host
  # bind mount. Change only image metadata and the two image-owned parents.
  /usr/bin/sed -i -E "s/^(dev:[^:]*:)[0-9]+:[0-9]+:/\1${host_uid}:${host_gid}:/" /etc/passwd
  /usr/bin/sed -i -E "s/^(dev:[^:]*:)[0-9]+:/\1${host_gid}:/" /etc/group
  if [[ "${LUCI_MICROVM_FIXED_HOME:-}" == "1" ]]; then
    # The trusted guest bootstrap sets this flag only after it mounted exact
    # reviewed context. Keep every path ancestor root-owned. The upstream
    # process gets a separate private HOME and cannot replace the mount path.
    if [[ "$(/usr/bin/stat -c '%u:%a' /home/dev)" != "0:755" ]] \
       || [[ "$(/usr/bin/stat -c '%u:%a' /home/dev/.luci-agent-home)" != "${host_uid}:700" ]]; then
      printf '%s\n' 'Luci guest home boundary is unavailable.' >&2
      exit 2
    fi
  else
    /usr/bin/chown "$host_uid:$host_gid" /home/dev /home/dev/.claude
  fi
  HOME="$(luci_bootstrap_home)"
  export HOME USER=dev LOGNAME=dev
  unset LUCI_HOST_UID LUCI_HOST_GID LUCI_MICROVM_FIXED_HOME
  exec /usr/bin/setpriv --reuid="$host_uid" --regid="$host_gid" --clear-groups \
    --inh-caps=-all --ambient-caps=-all --bounding-set=-all --no-new-privs \
    /usr/local/bin/entrypoint.sh
fi

# COMMAND selects the harness. Defaults to a plain shell so a sandbox is
# useful even when no agent is requested.
COMMAND="${LUCI_COMMAND:-/bin/bash}"

# ARGS_JSON is a JSON array of strings. It is preferred over the legacy
# comma-separated ARGS because an opening prompt routinely contains a comma.
ARGS_JSON="${LUCI_ARGS_JSON:-}"
ARGS="${LUCI_ARGS:-}"

exec /usr/local/bin/sandboxd \
  -addr :8080 \
  -command "${COMMAND}" \
  -args "${ARGS}" \
  -args-json "${ARGS_JSON}"
