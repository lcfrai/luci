#!/usr/bin/env bash
# infra/scripts/guards.sh — the ten repository rules for infra/, as named checks (task 01).
#
#   bash infra/scripts/guards.sh              checks the infra/ tree this script sits in
#   INFRA_ROOT=<dir> bash guards.sh           checks <dir> instead (guards_test.sh's fixtures)
#
# Each check prints   guard G<n>: ok
#                or   guard G<n>: FAIL <path>: <why>      (one line per offending path)
# Last line:          guards: <k> of 10 passed            exit 0 iff k == 10
# A failed read:      guards: inventory failed (find | sort exit <n>)       exit 2, no rule
#                     guards: scanner failed (awk exit <n>)                  reported
#
# THESE GUARDS ARE HYGIENE, NOT A SECURITY BOUNDARY (review C3). They run from the branch
# being built, so a pull request can edit or delete them. They catch mistakes in reviewed code.
# What stops a hostile pull request is outside the repository: the read-only plan identity and
# the branch control on the write connections (task 30).
#
# The rules read HCL text, and are fail-closed: a comment that spells a forbidden attribute
# fails too, and is reworded. Paths are searched under INFRA_ROOT only; .terraform/ (init
# caches, git-ignored) is skipped. One find, one git and one awk pass feed every rule:
# process start-up costs about 300 ms on the build laptop, so the rules share the reads.
# Each read is captured whole and its exit status tested before any rule reports: a read
# that failed must never be taken for "nothing found" (task 01 review I1, where an awk that
# failed left seven rules printing ok). A failed find or awk exits 2; a failed git fails
# G1 and G7, the two rules that need it.
set -uo pipefail   # not -e: every guard runs and reports, whatever an earlier one found

case "${BASH_SOURCE[0]}" in */*) HERE="${BASH_SOURCE[0]%/*}" ;; *) HERE=. ;; esac
cd "${INFRA_ROOT:-$HERE/..}" 2> /dev/null || { echo "guards: INFRA_ROOT is not a directory"; exit 2; }
BASE="${PWD##*/}"   # every path printed starts with this, e.g. infra/…
cd .. || exit 2
STACKS="$BASE/terraform/stacks"
ALLOWED="hashicorp/azurerm hashicorp/azuread azure/azapi"

# --- inventory, once ---------------------------------------------------------------------------
# read_failed <what> <command> <status>: a read that feeds the rules failed; stop before any rule.
read_failed() { echo "guards: $1 failed ($2 exit $3)"; exit 2; }
# The pipeline's status under pipefail covers find and sort alike; find's own errors stay on
# stderr, where the caller sees them.
LIST="$(find "$BASE" -name .terraform -prune -o -type f -print | LC_ALL=C sort)" ||
  read_failed inventory "find | sort" "$?"
# *.tf.json and *.tfvars.json are not read by any rule: G1 refuses them (task 01 review M3).
FILES=() TFTEST=() TFALL=() STACK_NAMES=()
declare -A IS_STACK=()
while IFS= read -r f; do
  [ -n "$f" ] || continue
  FILES+=("$f")
  case "$f" in
    *.tf)                         TFALL+=("$f") ;;
    *.tftest.hcl)  TFTEST+=("$f"); TFALL+=("$f") ;;
    *.tfvars|*.hcl)               TFALL+=("$f") ;;
  esac
  case "$f" in
    "$STACKS"/*/*) s="${f#"$STACKS"/}"; s="${s%%/*}"
                   if [ -z "${IS_STACK[$s]:-}" ]; then IS_STACK[$s]=1; STACK_NAMES+=("$s"); fi ;;
  esac
done <<< "$LIST"

# Tracked paths, once. Not a git work tree -> G1 and G7 cannot be checked, and fail closed.
declare -A TRACKED=()
GIT_OK=1
LIST="$(git -c core.quotepath=off ls-files -- "$BASE" 2> /dev/null)" || GIT_OK=0
while IFS= read -r f; do
  [ -n "$f" ] && TRACKED[$f]=1
done <<< "$LIST"

# --- reporting ---------------------------------------------------------------------------------
PASSED=0 G="" BAD=0
begin() { G="$1"; BAD=0; }
fail()  { echo "guard G$G: FAIL $1: $2"; BAD=1; }
end()   { if [ "$BAD" = 0 ]; then echo "guard G$G: ok"; PASSED=$((PASSED + 1)); fi; }

# --- the one awk pass --------------------------------------------------------------------------
# Tagged lines, TAB-separated, that the rules below consume:
#   RP <file> <name> <source>   an entry of a required_providers block (<source> may be empty)
#   GUID <file>                 a tfvars file holding a GUID                          (G2)
#   MOCK <file> <name>          a mock_provider in a test file                        (G3)
#   K8S <file>                  names the kubernetes or helm provider                  (G4)
#   BODY <file> <text>          a backend.tf with comments and whitespace removed     (G5)
#   PROV <file>                 a local-exec or remote-exec provisioner                (G6)
#   SECRET <file> <attr>        a password or client-secret attribute                  (G8)
#   LOCK <file> <address>       a provider in a lock file                              (G9)
#   TFVKEY <file> <key>         a top-level key of a tfvars file                       (G10)
#   TTKEY <file> <key>          a key of a test file's file-level variables block      (G10)
#   TTONE <file>                a test file with a one-line file-level variables block (G10)
# required_providers is read by a character scanner, not a line grep: fmt leaves a
# one-argument block on one line, so `required_providers { x = { source = "…" } }` is as legal
# as the spread-out form. No {n} intervals: mawk, Ubuntu's default awk, may lack them.
# shellcheck disable=SC2016  # an awk program: every $ in it is awk's, not the shell's (task 28)
SCAN='
function strip(s,   out, i, n, c, q) {         # drop # // and /* */ comments outside strings
  n = length(s); out = ""; q = 0
  for (i = 1; i <= n; i++) {
    c = substr(s, i, 1)
    if (q) { out = out c; if (c == "\\") { out = out substr(s, i + 1, 1); i++ } else if (c == "\"") q = 0; continue }
    if (c == "\"") { q = 1; out = out c; continue }
    if (c == "#" || (c == "/" && substr(s, i + 1, 1) == "/")) { while (i <= n && substr(s, i, 1) != "\n") i++; out = out "\n"; continue }
    if (c == "/" && substr(s, i + 1, 1) == "*") { i += 2; while (i <= n && substr(s, i, 2) != "*/") i++; i++; continue }
    out = out c
  }
  return out
}
function scan(file, s,   t, n, p, k, i, j, m, c, depth, ident, name, want, str, cnt, order, src) {
  if (index(s, "required_providers") == 0) return
  t = strip(s); n = length(t); p = 1
  while ((k = index(substr(t, p), "required_providers")) > 0) {
    i = p + k - 1
    if (i > 1 && substr(t, i - 1, 1) ~ /[A-Za-z0-9_-]/) { p = i + 1; continue }
    i += length("required_providers")
    while (i <= n && substr(t, i, 1) ~ /[ \t\r\n]/) i++
    if (substr(t, i, 1) != "{") { p = i; continue }
    depth = 1; i++; name = ""; want = 0; cnt = 0; split("", order); split("", src)
    while (i <= n && depth > 0) {
      c = substr(t, i, 1)
      if (c == "\"") {
        j = i + 1; str = ""
        while (j <= n && substr(t, j, 1) != "\"") {
          if (substr(t, j, 1) == "\\") { str = str substr(t, j + 1, 1); j += 2 } else { str = str substr(t, j, 1); j++ }
        }
        if (want && name != "") src[name] = str
        want = 0; i = j + 1; continue
      }
      if (c == "{") { depth++; i++; continue }
      if (c == "}") { depth--; i++; continue }
      if (c ~ /[A-Za-z_]/) {
        j = i; while (j <= n && substr(t, j, 1) ~ /[A-Za-z0-9_-]/) j++
        ident = substr(t, i, j - i)
        m = j; while (m <= n && substr(t, m, 1) ~ /[ \t]/) m++
        if (substr(t, m, 1) == "=" && substr(t, m + 1, 1) != "=") {
          if (depth == 1) { name = ident; order[++cnt] = ident; src[ident] = "" }
          else if (depth == 2 && ident == "source") want = 1
          i = m + 1; continue
        }
        i = j; continue
      }
      i++
    }
    for (j = 1; j <= cnt; j++) printf "RP\t%s\t%s\t%s\n", file, order[j], src[order[j]]
    p = i
  }
}
function flush() {
  if (cur == "") return
  if (is_tf) scan(cur, buf)
  if (is_backend) printf "BODY\t%s\t%s\n", cur, body
  buf = ""; body = ""
}
BEGIN {
  h = "[0-9a-fA-F]"; h4 = h h h h
  guid = h4 h4 "-" h4 "-" h4 "-" h4 "-" h4 h4 h4
  nsec = split("administrator_password_wo administrator_password client_secret admin_password", secret, " ")
}
FNR == 1 {
  flush(); cur = FILENAME; base = cur; sub(/.*\//, "", base)
  is_tf = (cur ~ /\.tf$/); is_test = (cur ~ /\.tftest\.hcl$/); is_vars = (cur ~ /\.tfvars$/)
  is_lock = (base == ".terraform.lock.hcl"); is_backend = (base == "backend.tf")
  in_vars = 0; guid_seen = 0; k8s_seen = 0; prov_seen = 0; split("", sec_seen)
}
{ sub(/\r$/, "") }
is_tf { buf = buf $0 "\n" }
is_backend { l = $0; sub(/(#|\/\/).*/, "", l); gsub(/[ \t]/, "", l); body = body l }
is_vars && !guid_seen && $0 ~ guid { guid_seen = 1; print "GUID\t" cur }
!k8s_seen {
  l = tolower($0)
  if (l ~ /(^|[^a-z0-9_])provider[ \t]+"(kubernetes|helm)"/ || l ~ /hashicorp\/(kubernetes|helm)([^a-z0-9_-]|$)/) {
    k8s_seen = 1; print "K8S\t" cur
  }
}
is_tf && !prov_seen && $0 ~ /provisioner[ \t]+"(local|remote)-exec"/ { prov_seen = 1; print "PROV\t" cur }
is_tf {
  for (i = 1; i <= nsec; i++)
    if (!(secret[i] in sec_seen) && $0 ~ ("(^|[^A-Za-z0-9_])" secret[i] "[ \t]*=([^=]|$)")) {
      sec_seen[secret[i]] = 1; print "SECRET\t" cur "\t" secret[i]
    }
}
is_lock && /^[ \t]*provider[ \t]+"/ {
  a = $0; sub(/^[ \t]*provider[ \t]+"/, "", a); sub(/".*/, "", a); print "LOCK\t" cur "\t" a
}
is_vars && /^[A-Za-z_][A-Za-z0-9_-]*[ \t]*=/ { k = $0; sub(/[ \t]*=.*/, "", k); print "TFVKEY\t" cur "\t" k }
is_test && /^[ \t]*mock_provider[ \t]+"/ {
  n = $0; sub(/^[ \t]*mock_provider[ \t]+"/, "", n); sub(/".*/, "", n); print "MOCK\t" cur "\t" n
}
is_test {
  if (in_vars && /^}/) in_vars = 0
  else if (in_vars && /^  [A-Za-z_][A-Za-z0-9_-]*[ \t]*=/) {
    k = $0; sub(/^  /, "", k); sub(/[ \t]*=.*/, "", k); print "TTKEY\t" cur "\t" k
  }
  else if (/^variables[ \t]*\{.*=/) print "TTONE\t" cur
  else if (/^variables[ \t]*\{/) in_vars = 1
}
END { flush() }
'
RP=() GUIDS=() K8S=() PROVS=() SECRETS=() LOCKED=() TFVKEYS=() TTKEYS=() TTONES=()
declare -A MOCK=() BODY=()
TAB=$'\t'
LIST=""
if [ "${#TFALL[@]}" -gt 0 ]; then
  LIST="$(awk "$SCAN" "${TFALL[@]}")" || read_failed scanner awk "$?"
fi
while IFS= read -r l; do
  tag="${l%%"$TAB"*}" rest="${l#*"$TAB"}"
  case "$tag" in
    RP) RP+=("$rest") ;;          GUID) GUIDS+=("$rest") ;;      K8S) K8S+=("$rest") ;;
    PROV) PROVS+=("$rest") ;;     SECRET) SECRETS+=("$rest") ;;  LOCK) LOCKED+=("$rest") ;;
    TFVKEY) TFVKEYS+=("$rest") ;; TTKEY) TTKEYS+=("$rest") ;;    TTONE) TTONES+=("$rest") ;;
    MOCK) MOCK[${rest%%"$TAB"*}]+=" ${rest#*"$TAB"} " ;;
    BODY) BODY[${rest%%"$TAB"*}]="${rest#*"$TAB"}" ;;
  esac
done <<< "$LIST"

# --- G1: no auto-loaded tfvars anywhere; no override file tracked by git ------------------------
# terraform.tfvars and *.auto.tfvars are how a bare apply hit Panacea's production (survey §1.1).
# An override file replaces the block it names, backend included; runbook §2 keeps one on disk,
# untracked, for the single local-state bootstrap apply, so only a TRACKED one fails.
# JSON configuration is refused outright: G3, G5, G9 and G10 read HCL only, so a *.tf.json
# would slip past four rules (task 01 review M3). The two JSON override names are left to the
# tracked check. Other JSON (task 30's infra/rbac/*.json) is not OpenTofu configuration, and passes.
begin 1
for f in "${FILES[@]}"; do
  case "${f##*/}" in
    terraform.tfvars|terraform.tfvars.json|*.auto.tfvars|*.auto.tfvars.json)
      fail "$f" "auto-loaded by tofu; name it <env>.tfvars and pass -var-file" ;;
    override.tf.json|*_override.tf.json) ;;
    *.tf.json|*.tfvars.json)
      fail "$f" "JSON configuration, which the other rules do not read; write it as HCL (.tf, .tfvars)" ;;
  esac
done
if [ "$GIT_OK" = 1 ]; then
  for f in "${!TRACKED[@]}"; do
    case "${f##*/}" in
      override.tf|override.tf.json|*_override.tf|*_override.tf.json)
        fail "$f" "override file tracked by git; it silently replaces the block it names" ;;
    esac
  done
else
  fail "$BASE" "not inside a git work tree, so tracked override files cannot be checked"
fi
end

# --- G2: no GUID in any tfvars ------------------------------------------------------------------
# Subscription, tenant and object ids arrive as TF_VAR_* / ARM_*, never in source.
begin 2
for f in "${GUIDS[@]}"; do fail "$f" "holds a GUID; ids arrive as TF_VAR_* or ARM_*, never in source"; done
end

# --- G3: every *.tftest.hcl mocks every provider its directory requires ------------------------
# "Its directory": the module or stack the test runs against. tests/<f> belongs to the parent;
# a root-level <f> to its own directory. Every *.tf there is read, not only versions.tf.
begin 3
declare -A REQ=()
for l in "${RP[@]}"; do
  file="${l%%"$TAB"*}" r="${l#*"$TAB"}"
  REQ[${file%/*}]+=" ${r%%"$TAB"*}"
done
for t in "${TFTEST[@]}"; do
  d="${t%/*}"
  [ "${d##*/}" = tests ] && d="${d%/*}"
  declare -A said=()
  for name in ${REQ[$d]:-}; do
    case "${MOCK[$t]:-}" in *" $name "*) continue ;; esac
    [ -n "${said[$name]:-}" ] && continue
    said[$name]=1
    fail "$t" "no mock_provider \"$name\", which $d requires; the test would call Azure"
  done
  unset said
done
end

# --- G4: no kubernetes or helm provider, as a block or a source (P8-D3) -------------------------
# Every OpenTofu-language file: *.tf, *.tfvars, *.hcl (tests and lock files too).
begin 4
for f in "${K8S[@]}"; do fail "$f" "names the kubernetes or helm provider; charts deploy the cluster (P8-D3)"; done
end

# --- G5: every stack's backend.tf is exactly `backend "azurerm" {}` ------------------------------
# Partial configuration only: the storage account arrives with -backend-config, never in source.
begin 5
for s in "${STACK_NAMES[@]}"; do
  f="$STACKS/$s/backend.tf"
  if [ ! -f "$f" ]; then
    fail "$f" "missing; every stack holds exactly terraform { backend \"azurerm\" {} }"
  elif [ "${BODY[$f]:-}" != 'terraform{backend"azurerm"{}}' ]; then
    fail "$f" "must hold exactly terraform { backend \"azurerm\" {} }: partial config, no body"
  fi
done
end

# --- G6: no local-exec or remote-exec provisioner -----------------------------------------------
begin 6
for f in "${PROVS[@]}"; do fail "$f" "a local-exec or remote-exec provisioner is a script tofu cannot plan"; done
end

# --- G7: every stack has a .terraform.lock.hcl tracked by git ------------------------------------
begin 7
for s in "${STACK_NAMES[@]}"; do
  f="$STACKS/$s/.terraform.lock.hcl"
  if [ "$GIT_OK" != 1 ]; then
    fail "$f" "not inside a git work tree, so tracking cannot be checked"
  elif [ ! -f "$f" ]; then
    fail "$f" "missing; create it with tofu providers lock (README) and commit it"
  elif [ -z "${TRACKED[$f]:-}" ]; then
    fail "$f" "exists but is not tracked by git; commit it so every init verifies the same hashes"
  fi
done
end

# --- G8: no password or client-secret attribute (P8-D10, P8-D11: there is no password to set) ----
begin 8
for l in "${SECRETS[@]}"; do
  fail "${l%%"$TAB"*}" "sets ${l#*"$TAB"}; identities are Entra-only, with no password or secret"
done
end

# --- G9: every provider source, declared or locked, is on the allowlist ------------------------
# A provider is a program that runs at plan time with the pipeline's token. An entry without a
# source is inferred as hashicorp/<name>; a resource type with no entry at all shows up only in
# the lock file, so both are read (review C3; task 30's init -lockfile=readonly is the partner).
# OD-32: `tofu` writes registry.opentofu.org/… into a lock file where the old engine wrote
# registry.terraform.io/…. Both hosts are stripped, because the tracked lock files still carry
# the old host until someone regenerates them with `tofu providers lock` (README, blocker 2).
# Narrow this to registry.opentofu.org once they have been, and a Terraform-written lock file
# then fails G9 by itself.
allowed() {
  local src="${1,,}"
  src="${src#registry.opentofu.org/}"
  src="${src#registry.terraform.io/}"
  case " $ALLOWED " in *" $src "*) return 0 ;; esac
  return 1
}
begin 9
for l in "${RP[@]}"; do
  file="${l%%"$TAB"*}" r="${l#*"$TAB"}"
  name="${r%%"$TAB"*}" source="${r#*"$TAB"}"
  src="${source:-hashicorp/$name}"
  allowed "$src" || fail "$file" "provider \"$name\" has source $src, not one of: $ALLOWED"
done
for l in "${LOCKED[@]}"; do
  allowed "${l#*"$TAB"}" || fail "${l%%"$TAB"*}" "locks provider ${l#*"$TAB"}, not one of: $ALLOWED"
done
end

# --- G10: a stack test's file-level variables block holds no key any of its tfvars sets ---------
# A file-level `variables` block overrides -var-file (Terraform 1.15.8 probe, review I1), so a
# file-level `environment` would turn validate.sh's prod.tfvars pass into a second dev pass.
# fmt writes a file-level block at column 0 and its keys at column 2, aligning every `=` (so
# `^  <key> *=`, any run of spaces). fmt also leaves a one-argument block on one line, with the
# key on the opener (b04-2 I1), so a one-line file-level block fails outright: multi-line is
# the only layout this rule reads. Every *.tftest.hcl under the stack counts, at any depth,
# root-level included (b04-2 M2): tofu test runs both.
stack_of() { local r="${1#"$STACKS"/}"; S="${r%%/*}"; [ "$1" != "$r" ]; }
begin 10
declare -A SETBY=()
for l in "${TFVKEYS[@]}"; do
  f="${l%%"$TAB"*}" key="${l#*"$TAB"}"
  stack_of "$f" || continue
  [ "$f" = "$STACKS/$S/${f##*/}" ] || continue            # <stack>/<env>.tfvars only
  [ -n "${SETBY[$S|$key]:-}" ] || SETBY[$S|$key]="$f"
done
for f in "${TTONES[@]}"; do
  stack_of "$f" && fail "$f" "one-line file-level variables block; write it multi-line, one key per line"
done
for l in "${TTKEYS[@]}"; do
  f="${l%%"$TAB"*}" key="${l#*"$TAB"}"
  stack_of "$f" || continue
  by="${SETBY[$S|$key]:-}"
  [ -z "$by" ] || fail "$f" "file-level variables key $key is set by $by, and overrides it; set it in each run's own variables block"
done
end

echo "guards: $PASSED of 10 passed"
[ "$PASSED" -eq 10 ]
