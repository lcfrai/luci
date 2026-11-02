#!/usr/bin/env bash
set -euo pipefail
mode=${1:?backend or frontend required}
case "$mode" in backend|frontend) ;; *) exit 64 ;; esac
repo=$(pwd -P)
mkdir -p artifacts/isolated-regression
stage=$(mktemp -d "$RUNNER_TEMP/luci-regression-XXXXXXXX")
mkdir "$stage/home"
unit="luci-regression-${GITHUB_RUN_ID}-${GITHUB_RUN_ATTEMPT}-${mode}"
quota=$((20 * $(nproc)))
# Dependencies were fetched above; tests receive no GitHub/provider environment
# and cannot route outside their own loopback-only network namespace.
sudo systemd-run --quiet --wait --pipe --collect --unit="$unit"   --property="CPUQuota=${quota}%" --property=MemoryMax=2147483648   --property=MemorySwapMax=0 --property=TasksMax=512   --property=RuntimeMaxSec=1320 --property=TimeoutStopSec=30   --property=KillMode=control-group --property=PrivateMounts=yes --property="InaccessiblePaths=-/run/docker.sock -/var/run/docker.sock -/run/containerd -/run/user"   /usr/bin/unshare --net /bin/bash -c '
    set -euo pipefail
    ip link set lo up
    uid=$1; gid=$2; repo=$3; stage=$4; path=$5; gomodcache=$6; mode=$7
    exec setpriv --reuid="$uid" --regid="$gid" --clear-groups --no-new-privs --bounding-set=-all --inh-caps=-all --ambient-caps=-all       env -i PATH="$path" HOME="$stage/home" TMPDIR="$stage"       LANG=C.UTF-8 CI=true CGO_ENABLED=0 GOMAXPROCS=1 GOMEMLIMIT=768MiB       GOTOOLCHAIN=local GOPROXY=off GOSUMDB=off GOVCS="*:off" GOAUTH=off GOENV=off       GOMODCACHE="$gomodcache" GOCACHE="$stage/go-build"       NODE_OPTIONS=--max-old-space-size=1024 UV_THREADPOOL_SIZE=1       RAYON_NUM_THREADS=1 ROLLDOWN_WORKER_THREADS=1 VITE_LUCI_API=       /bin/bash "$repo/.github/luci-regression/run-body.sh" "$repo" "$mode"
  ' _ "$(id -u)" "$(id -g)" "$repo" "$stage" "$PATH" "$(go env GOMODCACHE 2>/dev/null || printf /nonexistent)" "$mode"
