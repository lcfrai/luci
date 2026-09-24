#!/usr/bin/env bash
# Fixture: only the pin block matters to lint.sh (rule R6). The hashes are made up.
# name platform version kind member sha256 url
read -r -d '' PINS <<'PINS' || true
gitleaks linux-amd64 8.30.1 tgz gitleaks 1111111111111111111111111111111111111111111111111111111111111111 https://example.invalid/gitleaks.tar.gz
yq       linux-amd64 4.53.6 bin yq       2222222222222222222222222222222222222222222222222222222222222222 https://example.invalid/yq
yq       windows-amd64 4.53.6 bin yq.exe 3333333333333333333333333333333333333333333333333333333333333333 https://example.invalid/yq.exe
PINS
echo "$PINS" > /dev/null
