#!/usr/bin/env bash
# pipelines/scripts/secrets-pr.sh — gitleaks over a pull request's own commits (task 28;
# secrets.yml's pull_request job runs it after a full-depth checkout, secrets-pr_test.sh first).
#
#   bash pipelines/scripts/secrets-pr.sh        SYSTEM_PULLREQUEST_TARGETBRANCH names the target
#                                               (refs/heads/<b> on Azure Repos, <b> on GitHub)
#
# The range is <merge base of origin/<target> and HEAD>..HEAD, HEAD being the PR's merge commit.
# It makes NO network call (fix round 1, review I3). "checkout: self" keeps no credential
# (persistCredentials defaults to false), so a second "git fetch" of a private repository fails
# and would turn the secret gate red on every pull request for no reason. A full-depth checkout
# (fetchDepth: 0) already fetches +refs/heads/*:refs/remotes/origin/* with the job's token
# (training knowledge, UNVERIFIED: task 31 step 3 checks it on the first run). If the target is
# not there, this fails closed: it never scans a guessed range, and it leaves no token behind.
#
# Output: "secrets-pr: scanning <base>..HEAD", then gitleaks' own; exit gitleaks' status, or
#         secrets-pr: <why>                                                   exit 2
set -euo pipefail

target="${SYSTEM_PULLREQUEST_TARGETBRANCH:-}"
target="${target#refs/heads/}"
[ -n "$target" ] || { echo "secrets-pr: SYSTEM_PULLREQUEST_TARGETBRANCH is empty: not a pull-request build"; exit 2; }
ref="refs/remotes/origin/$target"
if ! git rev-parse --verify --quiet "$ref^{commit}" > /dev/null; then
  echo "secrets-pr: $ref is not in the checkout: secrets.yml's checkout must be fetchDepth: 0 (task 31 step 3)"
  exit 2
fi
base="$(git merge-base "$ref" HEAD)" || { echo "secrets-pr: $ref and HEAD share no commit"; exit 2; }
echo "secrets-pr: scanning $base..HEAD"
gitleaks git --no-banner --redact --config .gitleaks.toml --log-opts="$base..HEAD" .
