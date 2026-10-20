#!/bin/sh
set -eu
umask 077

# These bytes are part of the verified, immutable release image. A Compose
# environment override cannot substitute a different document runtime pin.
export LUCI_DOCUMENT_PROFILE_SHA256='' LUCI_DOCUMENT_TOOL_DOMAIN_SHA256='' LUCI_DOCUMENT_IMAGE_MANIFEST_SHA256=''
luci_document_pin_file=/opt/luci/document-image-manifest-pin
luci_document_root=/opt/luci/runtime/document-image
luci_document_manifest="$luci_document_root/document-runtime-manifest-v1.json"
if [ -L "$luci_document_pin_file" ] || [ ! -f "$luci_document_pin_file" ] ||
   [ "$(wc -c < "$luci_document_pin_file")" -ne 65 ] ||
   [ -L "$luci_document_root" ] || [ ! -d "$luci_document_root" ] ||
   [ -L "$luci_document_manifest" ] || [ ! -f "$luci_document_manifest" ]; then
  printf '%s\n' 'The Luci image is missing its verified document runtime companion.' >&2
  exit 1
fi
IFS= read -r luci_document_pin < "$luci_document_pin_file"
case "$luci_document_pin" in
  ''|*[!0-9a-f]*) printf '%s\n' 'Invalid document runtime identity.' >&2; exit 1 ;;
esac
if [ "${#luci_document_pin}" -ne 64 ] ||
   ! printf '%s  %s\n' "$luci_document_pin" "$luci_document_manifest" | sha256sum --check --status; then
  printf '%s\n' 'The document runtime manifest does not match this Luci release.' >&2
  exit 1
fi
export LUCI_DOCUMENT_IMAGE_MANIFEST_SHA256="$luci_document_pin"

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
