#!/usr/bin/env bash
# The half of task 11's values contract helm-unittest cannot assert.
#
# values.schema.json rejects an empty CIDR before any template renders, so helm-unittest
# reports it as a suite error rather than a failed template and no assertion can catch it
# (measured against helm-unittest 1.1.2; a suite-level `skipSchemaValidation` key is
# silently ignored). That is why `helm unittest` runs with --skip-schema-validation and
# why this script exists: it requires `helm template` to FAIL on each empty CIDR, naming
# that key. Without it, "empty fails the render" would be an unproved claim.
#
# Offline only. Renders locally, talks to no cluster, plans nothing against Azure.
# Usage: bash tests/schema-gate.sh [path-to-chart]   (default: the chart this lives in)
set -u

chart="${1:-$(cd "$(dirname "$0")/.." && pwd)}"
values="${chart}/tests/values-test.yaml"
helm="${HELM:-helm}"
rc=0

# The good case: plausible foundation CIDRs render and satisfy the schema.
if ! "$helm" template gate "$chart" -f "$values" >/dev/null 2>&1; then
  echo "FAIL: the chart does not render with tests/values-test.yaml"
  rc=1
else
  echo "ok: renders with plausible foundation CIDRs"
fi

# The gate: each of the four CIDRs, emptied one at a time, must stop the render and the
# message must name the key, so whoever hits it knows which foundation output is missing.
for key in vnetCidr podCidr serviceCidr privateEndpointsCidr; do
  out="$("$helm" template gate "$chart" -f "$values" --set "network.${key}=" 2>&1)"
  status=$?
  if [ "$status" -eq 0 ]; then
    echo "FAIL: network.${key} empty still rendered"
    rc=1
  elif ! printf '%s' "$out" | grep -q "${key}"; then
    echo "FAIL: network.${key} empty stopped the render but the message does not name it"
    echo "$out"
    rc=1
  else
    echo "ok: network.${key} empty stops the render, and says so"
  fi
done

if [ "$rc" -eq 0 ]; then echo "schema-gate: PASS"; else echo "schema-gate: FAIL"; fi
exit "$rc"
