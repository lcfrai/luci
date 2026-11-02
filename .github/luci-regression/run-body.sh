#!/usr/bin/env bash
set -euo pipefail
repo=${1:?}; mode=${2:?}
cd "$repo"
case "$mode" in
  backend)
    set +e
    timeout --signal=TERM --kill-after=30s 20m \
      python3 scripts/ci-go-tests.py --name 'Whole control-plane regression' \
      --output artifacts/isolated-regression/go -- \
      go test -json -p 1 -parallel 1 -count 1 -timeout=240s ./control-plane/...
    go_status=$?
    timeout --signal=TERM --kill-after=10s 60s \
      python3 -I -B -m unittest discover -s scripts/azure-setup -p test_operator_transport.py -v \
      > artifacts/isolated-regression/operator-envelope.log 2>&1
    operator_status=$?
    set -e
    printf 'Go regression exit: %s\nOperator-envelope exit: %s\n' "$go_status" "$operator_status"
    tail -c 16384 artifacts/isolated-regression/operator-envelope.log
    printf '\n'
    if [ "$go_status" -ne 0 ] || [ "$operator_status" -ne 0 ]; then exit 1; fi
    ;;
  frontend)
    cd desktop
    timeout --signal=TERM --kill-after=30s 20m       npm test -- --maxWorkers=1 --no-file-parallelism --no-cache       --reporter=default --reporter=json       --outputFile="$repo/artifacts/isolated-regression/vitest.json"
    ;;
  *) exit 64 ;;
esac
