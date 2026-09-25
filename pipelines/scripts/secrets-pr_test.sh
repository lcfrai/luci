#!/usr/bin/env bash
# pipelines/scripts/secrets-pr_test.sh — secrets-pr.sh against a throw-away git repository,
# offline (task 28 fix round 1, review I3). The repository has the shape an Azure Pipelines PR
# checkout leaves (fetchDepth: 0, persistCredentials: false): HEAD detached at the PR's merge
# commit (target tip first, PR tip second), the target branch at refs/remotes/origin/<target>, and
# an origin nothing can be fetched from, as a private repository cannot be once the checkout has
# removed its credential. A stub gitleaks, first on PATH, prints the arguments it was given and
# exits with STUB_GITLEAKS_EXIT (default 0). secrets.yml runs this before the scan.
#
#   bash pipelines/scripts/secrets-pr_test.sh
#
# Output: one line per check, then   secrets-pr_test: <k> of <n> checks passed   exit 0 iff k == n
set -uo pipefail
HERE="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
SCRIPT="${PANACEA_APP_SECRETS_PR:-$HERE/secrets-pr.sh}"
WORK="$(mktemp -d)" || { echo "secrets-pr_test: mktemp failed"; exit 2; }
trap 'rm -rf "$WORK"' EXIT
REPO="$WORK/repo"

g() { git -C "$REPO" -c user.name=fixture -c user.email=fixture@example.invalid -c commit.gpgsign=false "$@"; }
build() {
  mkdir -p "$WORK/bin" &&
    printf '%s\n' '#!/bin/sh' 'echo "stub gitleaks: $*"' 'exit "${STUB_GITLEAKS_EXIT:-0}"' > "$WORK/bin/gitleaks" &&
    chmod +x "$WORK/bin/gitleaks" &&
    git init -q "$REPO" &&
    T="$(g hash-object -t tree -w --stdin < /dev/null)" &&
    A="$(g commit-tree "$T" -m A)" &&
    B="$(g commit-tree "$T" -p "$A" -m 'B, the target tip')" &&
    C="$(g commit-tree "$T" -p "$A" -m 'C, the pull request')" &&
    M="$(g commit-tree "$T" -p "$B" -p "$C" -m 'Merge C into B, refs/pull/1/merge')" &&
    g update-ref refs/remotes/origin/main "$B" &&
    g update-ref refs/remotes/origin/feature "$C" &&
    g update-ref --no-deref HEAD "$M" &&
    g remote add origin "$WORK/no-such-remote"
}
build || { echo "secrets-pr_test: cannot build the fixture repository"; exit 2; }

pass=0 n=0
# check <name> <target, or -unset-> <gitleaks exit> <want status> <want line> [<text no line may hold>]
check() {
  local out st
  n=$((n + 1))
  out="$(
    cd "$REPO" || exit 99
    if [ "$2" = -unset- ]; then unset SYSTEM_PULLREQUEST_TARGETBRANCH
    else export SYSTEM_PULLREQUEST_TARGETBRANCH="$2"; fi
    PATH="$WORK/bin:$PATH" STUB_GITLEAKS_EXIT="$3" GIT_TERMINAL_PROMPT=0 bash "$SCRIPT" 2>&1
  )"
  st=$?
  if [ "$st" != "$4" ]; then
    echo "FAIL $1: exit $st, want $4"
  elif [ -n "${6:-}" ] && [[ "$out" == *"$6"* ]]; then
    echo "FAIL $1: the output holds \"$6\""
  else
    case $'\n'"$out"$'\n' in
      *$'\n'"$5"$'\n'*) echo "ok   $1"; pass=$((pass + 1)); return ;;
      *) echo "FAIL $1: no line \"$5\"" ;;
    esac
  fi
  printf '    %s\n' "$out"
}

SCAN="stub gitleaks: git --no-banner --redact --config .gitleaks.toml --log-opts=$B..HEAD ."
# The checkout fetched the target with the job's token; the scan must not fetch again.
check azure_repos_target refs/heads/main 0 0 "$SCAN"
check github_target main 0 0 "$SCAN"
check range_is_announced refs/heads/main 0 0 "secrets-pr: scanning $B..HEAD"
# A finding fails the job: gitleaks' status is the script's.
check finding_fails refs/heads/main 1 1 "$SCAN"
# A target the checkout did not fetch: fail closed, and never scan a guessed range.
check target_not_fetched refs/heads/release 0 2 \
  "secrets-pr: refs/remotes/origin/release is not in the checkout: secrets.yml's checkout must be fetchDepth: 0 (task 31 step 3)" \
  "stub gitleaks"
check not_a_pull_request -unset- 0 2 \
  "secrets-pr: SYSTEM_PULLREQUEST_TARGETBRANCH is empty: not a pull-request build" "stub gitleaks"

echo "secrets-pr_test: $pass of $n checks passed"
[ "$pass" -eq "$n" ]
