#!/usr/bin/env bash
# pipelines/scripts/lint.sh — the rules every pipeline file follows, as a script (task 28).
#
#   bash pipelines/scripts/lint.sh [<root>]      <root> defaults to the repository root
#
# Pipeline files: <root>/azure-pipelines.yml and <root>/pipelines/*.yml. Templates:
# <root>/pipelines/templates/*.yml. Every file under <root>/pipelines/ except
# pipelines/scripts/testdata/ is read for R3. Comments are stripped before any rule looks.
#
#   R1 triggers                     every pipeline file has trigger: and pr:, each with a non-empty
#                                   paths.include; "none" only in the manual pipelines,
#                                   infra-plan.yml and deploy.yml (task 30)
#   R2 tools                        no tool is installed outside templates/tools.yml: no curl, wget,
#                                   Invoke-WebRequest, go/pip/apt/choco/brew/cargo/gem/dotnet/winget
#                                   install, global npm install, helm plugin install, tflint --init,
#                                   rustup, or *Installer@ task, and install-tool.sh is called only by
#                                   the template (GoTool@0 and NodeTool@0 are the two runtimes allowed)
#   R3 no-drift                     no "latest", no "*" or "N.x" version, and no download piped
#                                   into a shell, anywhere under pipelines/ or in azure-pipelines.yml
#   R4 connections-are-variables    every service-connection input (azureSubscription,
#                                   connectedServiceName*, awsCredentials, kubernetesServiceConnection,
#                                   dockerRegistryServiceConnection, containerRegistry, …) is $(NAME)
#   R5 estate-guard-first           infra-plan.yml and deploy.yml (task 30), if present: the first
#                                   step that is not checkout, download or the tools template runs
#                                   pipelines/scripts/estate-guard.sh
#   R6 pins-have-hashes             every pin in install-tool.sh has seven fields and a 64-digit
#                                   lower-case SHA-256, and every tool a pipeline asks the template
#                                   for has a linux-amd64 (or "any") pin
#   R7 scripts-exist                every "bash <path>" (and filePath:/scriptPath:) names a file in
#                                   the tree, by a literal path
#   R8 own-connections              each file names only its own connections (task 30's table):
#                                   infra-plan.yml PLAN; images.yml PUSH; deploy.yml INFRA in stage
#                                   apply, CLUSTER in stages cluster and smoke, PLAN in the job named
#                                   outputs and in every other stage; every other file none. A
#                                   connection is PANACEA_APP_<ENV>_<KIND>_SERVICE_CONNECTION
#
# LIKE EVERY RULE HERE, THIS IS HYGIENE, NOT A BOUNDARY (review C3): a pull request can edit this
# script. The ADO branch control on each write connection is what stops a hostile change.
#
# Output: "R<n> <name>: ok", or one "R<n> <name>: FAIL <file>[:<line>]: <why>" per finding;
# then    lint: <k> of 8 rules pass over <n> pipeline files       exit 0 iff k == 8
#         lint: no pipeline files                                  exit 1
#         lint: scanner failed (awk exit <n>)                      exit 2
set -uo pipefail

HERE="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
ROOT="${1:-$(git -C "$HERE" rev-parse --show-toplevel)}" || exit 2
cd "$ROOT" 2> /dev/null || { echo "lint: $ROOT is not a directory"; exit 2; }

NAMES=("" triggers tools no-drift connections-are-variables estate-guard-first \
  pins-have-hashes scripts-exist own-connections)

# Every file, with the kind that says which rules read it.
ARGS=() NPIPE=0
add() { ARGS+=("kind=$1" "$2"); }
[ -f azure-pipelines.yml ] && { add pipe azure-pipelines.yml; NPIPE=$((NPIPE + 1)); }
shopt -s nullglob
for f in pipelines/*.yml; do add pipe "$f"; NPIPE=$((NPIPE + 1)); done
for f in pipelines/templates/*.yml; do add tmpl "$f"; done
[ "$NPIPE" -gt 0 ] || { echo "lint: no pipeline files"; exit 1; }
if [ -d pipelines ]; then
  LIST="$(find pipelines -path pipelines/scripts/testdata -prune -o -type f -print)" ||
    { echo "lint: inventory failed (find exit $?)"; exit 2; }
  while IFS= read -r f; do
    case "$f" in   # a case pattern's * crosses "/", so the deeper paths come first
      '') ;;
      pipelines/scripts/install-tool.sh) add pins "$f" ;;
      pipelines/templates/*/*) add other "$f" ;;
      pipelines/templates/*.yml) ;;                    # added above as a template
      pipelines/*/*) add other "$f" ;;
      pipelines/*.yml) ;;                              # added above as a pipeline file
      *) add other "$f" ;;
    esac
  done <<< "$LIST"
fi

# --- the scanner: one awk pass over every file, one "R<n><TAB>finding" line per finding --------
read -r -d '' PROG <<'AWK' || true
function base(p) { sub(/.*\//, "", p); return p }
function ind(s) { match(s, /^[ \t]*/); return RLENGTH }
function trim(s) { sub(/^[ \t]+/, "", s); sub(/[ \t]+$/, "", s); return s }
function unq(s) { s = trim(s); gsub(/^["']|["']$/, "", s); return s }
function emit(r, m) { print r "\t" m }
function reset() {
  split("", seen); split("", val); split("", hasp); blk = ""; inp = 0; ininc = 0
  g5 = (kind == "pipe" && (base(FILENAME) == "infra-plan.yml" || base(FILENAME) == "deploy.yml"))
  coll = 0; done5 = 0; first5 = 0; text5 = ""; intools = 0; tlist = 0; st = ""; job = ""
  inpins = 0
}
function endfile(   b, i, k, manual) {
  if (cur == "") return
  b = base(cur)
  if (curkind == "pipe") {
    manual = (b == "infra-plan.yml" || b == "deploy.yml")
    split("trigger pr", k, " ")
    for (i = 1; i <= 2; i++) {
      if (!(k[i] in seen)) emit("R1", cur ": no " k[i] ": block")
      else if (val[k[i]] == "none") { if (!manual) emit("R1", cur ": " k[i] ": none outside a manual pipeline") }
      else if (!(k[i] in hasp)) emit("R1", cur ": " k[i] ": has no paths.include")
    }
  }
  if (g5) {
    if (first5 == 0) emit("R5", cur ": no step runs pipelines/scripts/estate-guard.sh")
    else if (index(text5, "pipelines/scripts/estate-guard.sh") == 0)
      emit("R5", cur ":" first5 ": the first step does not run pipelines/scripts/estate-guard.sh")
  }
  cur = ""
}
function r7(p,   t, ok) {
  if (p ~ /\$/) { emit("R7", cur ": " p " is not a literal path"); return }
  if (!(p in exists)) { ok = (getline t < p); exists[p] = (ok >= 0); if (ok >= 0) close(p) }
  if (!exists[p]) emit("R7", cur ": " p " does not exist")
}
function r8(k,   b, ok) {
  b = base(cur)
  if (curkind == "pipe" && b == "infra-plan.yml") ok = (k == "PLAN")
  else if (curkind == "pipe" && b == "images.yml") ok = (k == "PUSH")
  else if (curkind == "pipe" && b == "deploy.yml") {
    if (job == "outputs") ok = (k == "PLAN")
    else if (st == "apply") ok = (k == "INFRA")
    else if (st == "cluster" || st == "smoke") ok = (k == "CLUSTER")
    else if (st != "") ok = (k == "PLAN")
    else ok = 0
    if (!ok) emit("R8", cur ": stage " (st == "" ? "(none)" : st) " names " k " (job " (job == "" ? "(none)" : job) ", line " FNR ")")
    return
  } else ok = 0
  if (!ok) emit("R8", cur ":" FNR ": names " k "; " (b == "infra-plan.yml" ? "infra-plan.yml may name only PLAN" : b == "images.yml" ? "images.yml may name only PUSH" : "only infra-plan.yml, images.yml and deploy.yml may name a connection"))
}
BEGIN {
  LATEST = "(^|[^A-Za-z0-9_])" "lat" "est" "([^A-Za-z0-9_]|$)"
  PIPESH = "\\|[ \t]*(sudo[ \t]+)?(ba|z|da)?sh([ \t;]|$)"
  PIPEIEX = "\\|[ \t]*(i" "ex|Invoke-Ex" "pression)"
  STEP = "^[ \t]*-[ \t]+(script|bash|pwsh|powershell|task|template|checkout|download|downloadBuild|getPackage|publish|reviewApp)[ \t]*:"
  CONN = "^[ \t-]*(azureSubscription|azureResourceManagerConnection|connectedServiceName|connectedServiceNameARM|ConnectedServiceName|ConnectedServiceNameARM|awsCredentials|kubernetesServiceConnection|kubernetesServiceEndpoint|dockerRegistryServiceConnection|dockerRegistryEndpoint|containerRegistry|serviceConnection|externalEndpoints)[ \t]*:"
  W = "(^|[^A-Za-z0-9_-])"; E = "([^A-Za-z0-9_-]|$)"
}
FNR == 1 { endfile(); cur = FILENAME; curkind = kind; reset() }
{
  line = $0; sub(/\r$/, "", line)
  code = line
  if (code ~ /^[ \t]*#/) code = ""; else sub(/[ \t]+#.*$/, "", code)
  yaml = (curkind == "pipe" || curkind == "tmpl")

  # R3: every file
  if (code != "") {
    if (code ~ LATEST) emit("R3", cur ":" FNR ": a moving version (" trim(code) ")")
    if (code ~ /[Vv]ersion[A-Za-z]*[ \t]*:[ \t]*["']?(\*|[0-9.]*x)["']?[ \t]*$/ || code ~ /@\*/)
      emit("R3", cur ":" FNR ": a wildcard version (" trim(code) ")")
    if (code ~ PIPESH || code ~ PIPEIEX) emit("R3", cur ":" FNR ": a download piped into a shell (" trim(code) ")")
  }

  # R6: the pin table
  if (curkind == "pins") {
    if (line ~ /<<[ \t]*'PINS'/) inpins = 1
    else if (inpins && line ~ /^PINS[ \t]*$/) inpins = 0
    else if (inpins && line !~ /^[ \t]*(#.*)?$/) {
      npins++
      n = split(trim(line), f, /[ \t]+/)
      if (n != 7) emit("R6", cur ":" FNR ": a pin has " n " fields, not 7")
      else if (length(f[6]) != 64 || f[6] ~ /[^0-9a-f]/) emit("R6", cur ":" FNR ": " f[1] " " f[2] " has no SHA-256")
      else pinned[f[1] SUBSEP f[2]] = 1
    }
    pinsfile = 1
  }
  if (!yaml || code ~ /^[ \t]*$/) next

  # R1: trigger: and pr:, top level, with paths.include
  if (curkind == "pipe") {
    if (code ~ /^[A-Za-z_][A-Za-z0-9_-]*[ \t]*:/) {
      key = code; sub(/[ \t]*:.*/, "", key)
      rest = code; sub(/^[^:]*:[ \t]*/, "", rest); rest = trim(rest)
      blk = ""
      if (key == "trigger" || key == "pr") {
        blk = key; seen[blk] = 1; inp = 0; ininc = 0
        val[blk] = (rest == "" ? "" : rest == "none" ? "none" : "inline")
      }
    } else if (blk != "") {
      i = ind(code); t = trim(code)
      if (inp && i <= pind && t !~ /^-/) { inp = 0; ininc = 0 }
      if (ininc && i <= iind && t !~ /^-/) ininc = 0
      if (t ~ /^paths[ \t]*:/) { inp = 1; pind = i; ininc = 0 }
      else if (inp && t ~ /^include[ \t]*:/) {
        r = t; sub(/^include[ \t]*:[ \t]*/, "", r)
        if (r ~ /^\[[ \t]*[^] \t]/) hasp[blk] = 1
        else if (r == "") { ininc = 1; iind = i }
      }
      else if (inp && t ~ /^exclude[ \t]*:/) ininc = 0
      else if (ininc && t ~ /^-[ \t]*[^ \t]/) hasp[blk] = 1
    }
  }

  # R2: no tool installed outside the tools template
  if (cur !~ /(^|\/)pipelines\/templates\/tools\.yml$/) {
    what = ""
    if (code ~ (W "(curl|wget|iwr|Invoke-WebRequest|Start-BitsTransfer|rustup)" E)) what = "a download or toolchain install"
    else if (code ~ (W "(go|pip|pip3|apt|apt-get|choco|brew|cargo|gem|winget|snap)[ \t]+(install|get)" E)) what = "a package-manager install"
    else if (code ~ (W "dotnet[ \t]+tool[ \t]+install") || code ~ (W "(npm|pnpm|yarn)[ \t]+(install|i|add)[ \t].*(-g|--global)" E) || code ~ (W "yarn[ \t]+global[ \t]+add")) what = "a global install"
    else if (code ~ /helm[ \t]+plugin[ \t]+install/ || code ~ /tflint[ \t]+--init/) what = "a plugin install"
    else if (code ~ /task[ \t]*:[ \t]*[A-Za-z]*Installer@/) what = "an installer task"
    else if (code ~ /install-tool\.sh/ && code !~ /^[ \t]*-[ \t]+["']?[^ \t"']+["']?[ \t]*$/)
      what = "a call to install-tool.sh"   # a bare path in a list (a trigger path) is not a call
    if (what != "") emit("R2", cur ":" FNR ": " what " outside templates/tools.yml (" trim(code) ")")
  }

  # R5: in infra-plan.yml and deploy.yml, the first real step is the estate guard
  isstep = (code ~ STEP)
  if (g5 && !done5) {
    if (coll && (isstep || code ~ /^[ \t]*-[ \t]+(job|deployment|stage)[ \t]*:/ || code ~ /^[A-Za-z]/)) { coll = 0; done5 = 1 }
    else if (coll) text5 = text5 "\n" code
    else if (isstep) {
      sk = code; sub(/^[ \t]*-[ \t]+/, "", sk); sv = sk
      sub(/[ \t]*:.*/, "", sk); sub(/^[^:]*:[ \t]*/, "", sv)
      if (!(sk == "checkout" || sk ~ /^download/ || (sk == "template" && unq(sv) ~ /templates\/tools\.yml$/))) {
        coll = 1; first5 = FNR; text5 = code
      }
    }
  }

  # R6: the tools each pipeline asks the template for
  if (code ~ /template[ \t]*:[ \t]*["']?[^ ]*templates\/tools\.yml/) { intools = 1; tlist = 0 }
  else if (intools) {
    if (isstep || code ~ /^[ \t]*-[ \t]+(job|deployment|stage)[ \t]*:/ || code ~ /^[A-Za-z]/) intools = 0
    else {
      t = trim(code)
      if (t ~ /^tools[ \t]*:/) {
        r = t; sub(/^tools[ \t]*:[ \t]*/, "", r)
        if (r ~ /^\[/) {
          gsub(/[][ \t"']/, "", r); n = split(r, a, ",")
          for (j = 1; j <= n; j++) if (a[j] != "") req[a[j]] = cur
        } else { tlist = 1; tind = ind(code) }
      } else if (tlist && t ~ /^-[ \t]*[^ \t]/ && ind(code) >= tind) {
        r = t; sub(/^-[ \t]*/, "", r); req[unq(r)] = cur
      } else if (tlist && ind(code) <= tind) tlist = 0
    }
  }

  # R4 and R8: service connections
  isconn = (code ~ CONN)
  if (isconn) {
    k = code; sub(/^[ \t-]*/, "", k); sub(/[ \t]*:.*/, "", k)
    v = code; sub(/^[^:]*:[ \t]*/, "", v); v = unq(v); vv = v
    gsub(/\$\{\{[^}]*\}\}/, "X", vv)
    if (vv !~ /^\$\([A-Za-z_][A-Za-z0-9_.]*\)$/) emit("R4", cur ":" FNR ": " k " is " v ", not a $(variable)")
  }
  if (curkind == "pipe" && base(cur) == "deploy.yml") {
    if (code ~ /^[ \t]*-[ \t]+stage[ \t]*:/) { st = code; sub(/^[^:]*:[ \t]*/, "", st); st = unq(st); job = "" }
    if (code ~ /^[ \t]*-[ \t]+(job|deployment)[ \t]*:/) { job = code; sub(/^[^:]*:[ \t]*/, "", job); job = unq(job) }
  }
  s = code; nsc = gsub(/SERVICE_CONNECTION/, "&", s)
  if (nsc > 0 || isconn) {
    known = 0; s = code
    while (match(s, /_(PLAN|INFRA|CLUSTER|PUSH)_SERVICE_CONNECTION/)) {
      r8(substr(s, RSTART + 1, RLENGTH - 20)); known++; s = substr(s, RSTART + RLENGTH)
    }
    if (nsc > known || (isconn && known == 0)) r8("an unnamed connection")
  }

  # R7: every script a step runs exists
  s = code
  while (match(s, /(^|[^A-Za-z0-9_-])bash[ \t]+[^ \t'";|&)]+/)) {
    m = substr(s, RSTART, RLENGTH); s = substr(s, RSTART + RLENGTH)
    sub(/^.*bash[ \t]+/, "", m)
    if (m ~ /^-/ || (m !~ /\// && m !~ /\.sh$/)) continue
    r7(m)
  }
  if (code ~ /(filePath|scriptPath)[ \t]*:/) { v = code; sub(/^[^:]*:[ \t]*/, "", v); r7(unq(v)) }
}
END {
  endfile()
  for (t in req) {
    if (!pinsfile) { emit("R6", req[t] ": asks for " t ", and pipelines/scripts/install-tool.sh is missing"); continue }
    if (!((t SUBSEP "linux-amd64") in pinned) && !((t SUBSEP "any") in pinned))
      emit("R6", req[t] ": tool " t " has no linux-amd64 pin")
  }
  if (pinsfile && npins == 0) emit("R6", "pipelines/scripts/install-tool.sh: no pins")
}
AWK

# A read that fails must never pass for "nothing found" (task 01 review I1): capture, then test.
OUT="$(awk "$PROG" "${ARGS[@]}")"
st=$?
[ "$st" -eq 0 ] || { echo "lint: scanner failed (awk exit $st)"; exit 2; }
for f in azure-pipelines.yml pipelines/*.yml; do
  [ -f "$f" ] && [ ! -s "$f" ] && OUT+=$'\n'"R1"$'\t'"$f: the file is empty"
done

declare -A FOUND=()
while IFS=$'\t' read -r r msg; do
  [ -n "$r" ] || continue
  FOUND[$r]+="${NAMES[${r#R}]}"$'\t'"$msg"$'\n'
done <<< "$OUT"

pass=0
for n in 1 2 3 4 5 6 7 8; do
  if [ -z "${FOUND[R$n]:-}" ]; then
    note=""
    [ "$n" = 5 ] && [ ! -f pipelines/infra-plan.yml ] && [ ! -f pipelines/deploy.yml ] &&
      note=" (no infra-plan.yml or deploy.yml yet)"
    echo "R$n ${NAMES[n]}: ok$note"
    pass=$((pass + 1))
  else
    while IFS=$'\t' read -r name msg; do
      [ -n "$name" ] && echo "R$n $name: FAIL $msg"
    done <<< "${FOUND[R$n]}"
  fi
done
echo "lint: $pass of 8 rules pass over $NPIPE pipeline files"
[ "$pass" -eq 8 ]
