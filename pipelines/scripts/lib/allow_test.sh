#!/usr/bin/env bash
# pipelines/scripts/lib/allow_test.sh — the SPDX evaluator, exceptions and the denied globs of
# lib/allow.sh, with the tables set in memory (no yq, no files). Task 28.
#
#   bash pipelines/scripts/lib/allow_test.sh
#
# Output: one "FAIL" line per wrong answer, then   allow_test: <k> of <n> cases passed
set -uo pipefail
HERE="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
# shellcheck source=pipelines/scripts/lib/allow.sh
source "${PANACEA_APP_ALLOW_LIB:-$HERE/allow.sh}"

ALLOWED=(MIT Apache-2.0 BSD-2-Clause BSD-3-Clause ISC)
DENIED=(tldraw "@tldraw/*")
EXC=("npm"$'\t'"mpl-thing"$'\t'"MPL-2.0")

pass=0 n=0
want() { # want <0|1> <description> <command…>
  local w="$1" d="$2" got; shift 2
  n=$((n + 1))
  "$@"; got=$?
  [ "$got" -ne 0 ] && got=1
  if [ "$got" = "$w" ]; then pass=$((pass + 1)); else echo "FAIL $d: got $got, want $w"; fi
}

want 0 "one allowed id"                  spdx_ok "MIT"
want 1 "one id outside the list"         spdx_ok "GPL-3.0-only"
want 0 "OR with one allowed side"        spdx_ok "GPL-3.0-only OR MIT"
want 0 "OR, both allowed"                spdx_ok "MIT OR Apache-2.0"
want 1 "AND with one side outside"       spdx_ok "MIT AND Zlib"
want 0 "AND, both allowed"               spdx_ok "MIT AND ISC"
want 1 "grouped OR AND an outsider"      spdx_ok "(MIT OR Apache-2.0) AND Unicode-3.0"
want 0 "grouped OR AND allowed"          spdx_ok "(MIT OR Apache-2.0) AND BSD-3-Clause"
want 0 "AND binds tighter than OR"       spdx_ok "Zlib AND GPL-3.0-only OR MIT"
want 1 "WITH is not its base id"         spdx_ok "Apache-2.0 WITH LLVM-exception"
want 1 "an unclosed group"               spdx_ok "(MIT"
want 1 "a dangling operator"             spdx_ok "MIT OR"
want 1 "nothing at all"                  spdx_ok ""
want 1 "two ids with no operator"        spdx_ok "MIT Apache-2.0"
want 1 "a lower-case operator"           spdx_ok "GPL-3.0-only or MIT"
want 1 "not SPDX (npm's old free text)"  spdx_ok "Apache 2.0"
want 0 "an exception, exact"             licence_ok npm mpl-thing "MPL-2.0"
want 1 "an exception, other ecosystem"   licence_ok cargo mpl-thing "MPL-2.0"
want 1 "an exception, relicensed"        licence_ok npm mpl-thing "GPL-3.0-only"
want 1 "an exception, other package"     licence_ok npm other "MPL-2.0"
want 0 "denied by name"                  denied tldraw
want 0 "denied by glob"                  denied "@tldraw/tldraw"
want 1 "a name that only contains it"    denied "not-tldraw"

echo "allow_test: $pass of $n cases passed"
[ "$pass" -eq "$n" ]
