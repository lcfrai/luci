#!/bin/sh
set -eu
umask 077

# This runs only in the trusted, private engine before dockerd starts. It never
# walks or changes retained project/account files and accepts no path arguments.
if [ "$#" -ne 0 ] || [ "$(id -u)" != 0 ]; then
  printf '%s\n' 'Private workspace initialization requires the trusted engine startup.' >&2
  exit 1
fi
refuse() {
  printf '%s\n' 'Luci refused a linked, non-directory, or incompatible retained workspace.' \
    'Existing files were preserved. Use the original installation to export your projects and data, then restore them into a fresh Luci installation, or have an administrator inspect the named volume ownership. Do not remove the retained volume.' >&2
  exit 1
}
valid_directory() {
  [ ! -L "$1" ] && [ -d "$1" ]
}
private_directory() {
  valid_directory "$1" && [ "$(stat -c '%u:%g:%a' "$1")" = 1001:1001:700 ]
}

# A new named volume already has an empty root-owned mount point. Treat only an
# empty root as new; populated roots must already belong to the private user.
luci_new_workspace=0
if [ -e /luci-workspace ] || [ -L /luci-workspace ]; then
  valid_directory /luci-workspace || refuse
  if ! private_directory /luci-workspace; then
    [ -z "$(find /luci-workspace -mindepth 1 -maxdepth 1 -print -quit)" ] || refuse
    luci_new_workspace=1
  fi
else
  luci_new_workspace=1
fi

# Validate every retained top-level directory before creating any missing one.
# A symlink never gets followed by mkdir, chmod, chown or the controller.
for luci_directory in /luci-workspace/home /luci-workspace/state /luci-workspace/projects /luci-workspace/tmp; do
  if [ -e "$luci_directory" ] || [ -L "$luci_directory" ]; then
    private_directory "$luci_directory" || refuse
  fi
done
if [ -e /run/luci-engine ] || [ -L /run/luci-engine ]; then
  valid_directory /run/luci-engine || refuse
fi

if [ "$luci_new_workspace" -eq 1 ]; then
  mkdir -p /luci-workspace
  chown 1001:1001 /luci-workspace
  chmod 0700 /luci-workspace
fi
for luci_directory in /luci-workspace/home /luci-workspace/state /luci-workspace/projects /luci-workspace/tmp; do
  if [ ! -d "$luci_directory" ]; then
    mkdir -m 0700 "$luci_directory"
    chown 1001:1001 "$luci_directory"
  fi
done

# Only the socket directory is shared with the controller. dockerd creates its
# socket as root:luci (GID 1001), mode 0660, using its explicit --group argument.
mkdir -p /run/luci-engine
chown 0:1001 /run/luci-engine
chmod 0750 /run/luci-engine
