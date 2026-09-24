#!/usr/bin/env bash
# Case r6_short_sha: the yq row lost a digit of its hash, so nothing verifies the download.
# name platform version kind member sha256 url
read -r -d '' PINS <<'PINS' || true
gitleaks linux-amd64 8.30.1 tgz gitleaks 1111111111111111111111111111111111111111111111111111111111111111 https://example.invalid/gitleaks.tar.gz
yq       linux-amd64 4.53.6 bin yq       222222222222222222222222222222222222222222222222222222222222222 https://example.invalid/yq
PINS
echo "$PINS" > /dev/null
