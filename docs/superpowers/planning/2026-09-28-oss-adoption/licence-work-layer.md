# Licence audit — the work layer (Phase W)

**Auditor pass over `01-work-layer.md`. Adversarial: the job was to DISPROVE the licence claims, not confirm them.**

**Date of every fetch below: 2026-09-28.** Machine: KPMG Windows 11, behind the Netskope
TLS-intercepting proxy. Fetch method: `curl -s --ssl-no-revoke` through `claudezsh`, plus
`raw.githubusercontent.com` package-manifest reads. Where a fetch failed the row says so and the
verdict is `UNVERIFIED`, never inferred.

**No source file of any copyleft or source-available project was opened.** Only `LICENSE` files and
public repository pages, which is what the rule permits.

---

## Headline: where I disagree with the researcher

Four disagreements, in severity order. Detail in §3.

1. **TanStack DB ships Apache-2.0 code declared as MIT, and the researcher's licence gate would
   not catch it.** `@tanstack/db` depends on `@tanstack/db-ivm` (`workspace:*`, so it ships in the
   artifact). `db-ivm/package.json` declares `"license": "MIT"` and the package has **no LICENSE
   file** (`raw.githubusercontent.com/TanStack/db/main/packages/db-ivm/LICENSE` → **HTTP 404**).
   Its own README, read 2026-09-28, says: *"This is an implementation of differential dataflow used
   by TanStack DB, **forked from [@electric-sql/d2ts]**"*. And `electric-sql/d2ts` is
   **Apache-2.0** — root `LICENSE` is the Apache text and `packages/d2ts/package.json` declares
   `"license": "Apache-2.0"`, both read 2026-09-28. So a derivative work of Apache-2.0 code is
   being distributed to us under an MIT declaration with no Apache attribution and no NOTICE.
   Both licences are on our allowed list, so this is **not a blocker** — but it means the
   researcher's "MIT" is the wrong answer for what we would actually ship, and a gate that trusts
   `package.json` would print the wrong answer too. **Verdict: conditional.** Obligation in §3.
2. **Rocicorp Zero is NOT unverified. It is Apache-2.0, and the researcher looked in the wrong
   place.** They fetched `packages/zero/LICENSE` (404) and `npmjs.com` (403 through the proxy) and
   concluded "excluded because unread". The licence is declared in
   `raw.githubusercontent.com/rocicorp/mono/main/packages/zero/package.json`, read 2026-09-28:
   `"license": "Apache-2.0"` at `@rocicorp/zero@1.11.0`. That file was reachable from this machine.
   Exclude Zero on **maturity and architecture** grounds if you like; do not exclude it on licence.
3. **Apache-2.0 is not "permissive, ship it and forget it" — and React Aria is the one that bites.**
   `adobe/react-spectrum` has **no `NOTICE`** but it does have **`NOTICE.txt` (HTTP 200)**, read
   2026-09-28, listing Adobe sources whose code was modified into the codebase. Apache-2.0 §4(d)
   makes propagating that file's contents a condition of *any* redistribution, including the binary
   KPMG hands a customer. The researcher's document has no obligation column at all and treats
   Apache-2.0 as equivalent to MIT. It is not. `cel-go`, `oklog/ulid`, `bleve`, `Fuse.js` and
   `electric` were each checked for a NOTICE file: **all 404**, so those carry §4(a)/(b) only.
4. **"Vendoring" MIT code is a heavier obligation than adopting it, not a lighter one.** For cmdk,
   tinykeys and the three Gitea files, the MIT notice must travel with our *source and binary*, and
   the copied lines become ours to patch forever. The researcher treats vendor as the cautious
   option; legally it is the one that puts third-party text inside our repository, where a later
   refactor can strip the notice and nothing will fail.
5. **Gitea's root `LICENSE` does not, by itself, establish the licence of `dependency.go` — so I
   checked the three files directly.** All three carry an explicit per-file header, read
   2026-09-28 from `raw.githubusercontent.com/go-gitea/gitea/main/models/issues/<file>`:
   `// Copyright 2018|2017|2019 The Gitea Authors. All rights reserved.` /
   `// SPDX-License-Identifier: MIT`. **The vendoring claim survives, and now it has per-file
   evidence rather than a root-licence inference.** (Reading Gitea source is permitted: it is MIT,
   not copyleft.)

Everything else in the researcher's document that I could reach, I confirm. The two traps it found
independently — **Planka's relicence** and **Focalboard's multi-licence `LICENSE.txt`** — are real
and are the strongest work in the document.

---

## 2. Method, and what "verified" means in the table

For every `adopt` and `vendor` candidate I did all five checks the brief asks for:

1. **Fetched the actual licence file** at the actual repository over `raw.githubusercontent.com`
   and quoted its identifying line. Where the researcher relied on a GitHub **About sidebar** or a
   **GitHub API `spdx_id`** — TaskChampion, react-querybuilder, Bugzilla, vis-timeline,
   frappe-gantt, kbar, ninja-keys — **I went and read the file instead.** All seven held.
2. **Looked for a dual / changed licence**: a second licence file, an `-EE`/`-Enterprise` file, a
   BUSL change-date, a Commons Clause, or a separate commercial scope. Found in Meilisearch,
   Focalboard, BlockNote, Planka, Tiptap (separate npm scope), vis-timeline (dual, both allowed).
3. **Checked the sub-package we would actually install**, not just the monorepo root. This is where
   the two new findings came from. Tiptap: **all 60 directories under `packages/` enumerated** and
   every `package.json` `license` field read. TanStack DB, dnd-kit, TanStack Table/Virtual,
   React Aria, Electric, BlockNote: the installed package's own manifest read.
4. **Checked direct dependencies that ship in the artifact** for copyleft: `squirrel` →
   `lann/builder` (MIT), `lann/ps` (MIT); `cel-go` → `protobuf` (BSD-3), `cel.dev/expr`
   (Apache-2.0), `antlr4-go/antlr` (BSD-3, three-clause redistribution text confirmed),
   `golang.org/x/exp` (BSD-3); `@tanstack/db` → `@standard-schema/spec`, `@tanstack/pacer-lite`,
   `@tanstack/db-ivm` (**see §1 finding 1**).
5. **Checked for a NOTICE file** on every Apache-2.0 candidate, because §4(d) is the only
   Apache-2.0 obligation that most engineers miss. Only **React Aria has one** (`NOTICE.txt`).

**Reachability from this machine (Netskope proxy), 2026-09-28.** `raw.githubusercontent.com` — OK.
`registry.npmjs.org` — **OK** (this is what resolved Zero; the researcher tried `npmjs.com`, which
is a different host and does 403). `api.github.com` unauthenticated — **403 rate-limited after
~30 calls**; authenticated `gh api` works but takes ~15 s per call through the proxy.
`npmjs.com` — not retried.

---

## 3. The table — one row per candidate

Verdicts: **yes** = permissive, ship it. **conditional** = allowed, but we take on a stated
obligation. **no** = forbidden by the licence rule.

### 3.1 Researcher said ADOPT

| # | Candidate | Claimed | **Verified** | Source URL (read 2026-09-28) | Agree? | Verdict | Obligation we take on |
|---|---|---|---|---|---|---|---|
| 1 | TanStack Virtual | MIT | **MIT** — `MIT License / Copyright (c) 2021-present Tanner Linsley`. `packages/react-virtual/package.json` → `"license": "MIT"`. 7,123★, pushed 2026-09-21 | `raw.githubusercontent.com/TanStack/virtual/main/LICENSE` | yes | **yes** | Ship the MIT text + copyright line in `THIRD-PARTY.md` and in the desktop's about/licences view |
| 2 | TanStack Table | MIT | **MIT** — `Copyright (c) 2016 Tanner Linsley`. `packages/react-table/package.json` → MIT. 28,460★, pushed 2026-09-16 | `.../TanStack/table/main/LICENSE` | yes | **yes** | Same MIT notice |
| 3 | dnd-kit | MIT | **MIT** — `Copyright (c) 2021, Claudéric Demers`. All five packages we would install (`core`, `sortable`, `utilities`, `modifiers`, `accessibility`) declare `"license": "MIT"`. 17,679★, pushed 2026-09-12 | `.../clauderic/dnd-kit/master/LICENSE` | yes | **yes** | Same MIT notice |
| 4 | React Aria (react-spectrum) | Apache-2.0 | **Apache-2.0** — root `LICENSE` is the Apache 2.0 text; `packages/react-aria/package.json` and `packages/react-aria-components/package.json` both `"license": "Apache-2.0"`. **`NOTICE.txt` EXISTS (HTTP 200)** and lists modified Adobe sources. 15,894★, pushed 2026-09-27 | `.../adobe/react-spectrum/main/LICENSE` and `.../main/NOTICE.txt` | **partly — licence yes, "no obligation" no** | **conditional** | Apache-2.0 §4(a) ship the licence; §4(b) keep the copyright/attribution notices; **§4(d) reproduce the contents of `NOTICE.txt` in our own distributed NOTICE** — this binds the binary KPMG hands a customer, not just our repo |
| 5 | Radix Primitives | MIT | **MIT** — `MIT License / Copyright (c) 2022 WorkOS`. GitHub API `spdx_id` MIT, 19,339★, pushed 2026-08-08. **Caveat: the per-component `packages/react/<name>/package.json` manifests were NOT enumerated** (the repo has `packages/core` and `packages/react` subtrees) | `.../radix-ui/primitives/main/LICENSE` | yes | **yes**, with one check outstanding | MIT notice. **Before install, read the `license` field of each `@radix-ui/react-*` we take** — the same check that caught the TanStack DB issue |
| 6 | Tiptap core (`@tiptap/*`) | MIT core; `@tiptap-pro/*` proprietary | **MIT, and stronger than claimed.** Root `LICENSE.md`: `MIT License / Copyright (c) 2025, Tiptap GmbH`. **I enumerated all 60 directories under `packages/` and read every `license` field: every one is `MIT`** — including the extensions that used to be Pro (`ai-toolkit`, `extension-collaboration`, `extension-collaboration-caret`, `extension-table-of-contents`, `extension-unique-id`, `extension-drag-handle`, `extension-emoji`, `extension-file-handler`, `extension-find-and-replace`, `extension-invisible-characters`, `extension-mathematics`, `extension-node-range`, `markdown`, `html`). 38,556★, pushed 2026-09-25 | `.../ueberdosis/tiptap/main/LICENSE.md` + `.../main/packages/<pkg>/package.json` ×60 | **yes — and this closes the researcher's own "unverified item 4"** | **yes** | MIT notice. **The gate rule stands and is now precise: the Pro code is in a *different npm scope* (`@tiptap-pro/*`) that is not in this repository at all, so a `@tiptap-pro/` prefix denylist in `tools/licencegate` is exactly the right control.** Also deny `@hocuspocus/*` until someone verifies it separately — it was not checked here |
| 7 | ProseMirror | MIT | **MIT** — `Copyright (C) 2015-2017 by Marijn Haverbeke <marijn@haverbeke.berlin> and others`, MIT permission text. **Caveat: only `prosemirror-state` was read.** ProseMirror is ~12 separate repos (`-model`, `-view`, `-transform`, `-commands`, `-history`, `-keymap`, `-schema-list`, `-tables`, …) and each has its own `LICENSE` | `.../ProseMirror/prosemirror-state/master/LICENSE` | yes, with scope narrowed | **yes** | MIT notice **per module**. `@tiptap/pm` re-exports them, so the gate must walk the transitive tree, not stop at `@tiptap/pm` |
| 8 | uFuzzy | MIT | **MIT** — `MIT License / Copyright (c) 2022 Leon Sorokin`. 3,033★, pushed 2026-09-08 | `.../leeoniya/uFuzzy/main/LICENSE` | yes | **yes** | MIT notice |
| 9 | **TanStack DB** | MIT | **MIT at the root and in `packages/db`** (`MIT License / Copyright (c) 2025 Kyle Mathews`) — **but `@tanstack/db` depends on `@tanstack/db-ivm` (`workspace:*`), which declares MIT, has NO LICENSE file (404), and its README says it is "forked from `@electric-sql/d2ts`", which is Apache-2.0** (`electric-sql/d2ts` root `LICENSE` = Apache text; `packages/d2ts/package.json` = `"license": "Apache-2.0"`). 3,920★, pushed 2026-09-28, still BETA | `.../TanStack/db/main/LICENSE`; `.../main/packages/db/package.json`; `.../main/packages/db-ivm/package.json`; `.../main/packages/db-ivm/README.md`; `.../electric-sql/d2ts/main/LICENSE` | **NO — I disagree** | **conditional** | Treat the `db-ivm` portion as **Apache-2.0**: ship the Apache text and attribute ElectricSQL / the `d2ts` authors in `THIRD-PARTY.md`, and record that the upstream attribution was not carried by the fork. Doing both satisfies whichever licence actually governs. **Add a licence-gate rule that flags a package whose `license` field disagrees with its README/NOTICE provenance** — `package.json` alone would have printed "MIT" here |
| 10 | participle | MIT | **MIT** — `COPYING`: `Copyright (C) 2017-2022 Alec Thomas` + MIT permission text. GitHub API MIT. 3,891★, pushed 2026-09-28 | `.../alecthomas/participle/master/COPYING` | yes | **yes** | MIT notice. Note the file is `COPYING`, not `LICENSE` — a scanner keyed on `LICENSE*` will miss it |
| 11 | expr | MIT | **MIT** — `MIT License / Copyright (c) 2018 Anton Medvedev`. 8,030★, pushed 2026-07-07 | `.../expr-lang/expr/master/LICENSE` | yes | **yes** | MIT notice |
| 12 | cel-go | Apache-2.0 | **Apache-2.0** — root `LICENSE` is the Apache 2.0 text. **No NOTICE file (404).** Direct deps all permissive: `protobuf` BSD-3, `cel.dev/expr` Apache-2.0, `antlr4-go/antlr` BSD-3 (three-clause redistribution text confirmed), `golang.org/x/exp` BSD-3. Pushed 2026-09-10. **`go.mod` now also requires `cel.dev/cel-go v0.32.0` — the module appears to be migrating to a `cel.dev` path; re-verify the licence at the new path before pinning** | `.../google/cel-go/master/LICENSE` + `.../master/go.mod` | yes | **conditional** | Apache-2.0 §4(a)/(b): ship the licence text and retain attribution. No NOTICE to propagate. Plus the BSD-3 notices for antlr/protobuf/x/exp |
| 13 | squirrel | MIT (GitHub says NOASSERTION) | **MIT** — `MIT License / Squirrel: The Masterminds / Copyright (c) 2014-2015, Lann Martin. Copyright (C) 2015-2016, Google. Copyright (C) 2015, Matt Farina and Matt Butcher.` **GitHub API `spdx_id` = `NOASSERTION` confirmed** — the classifier trips on the project header above the MIT text. Deps `lann/builder` MIT, `lann/ps` MIT. 7,986★, **last push 2024-04-24 — 2 years 5 months quiet** | `.../Masterminds/squirrel/master/LICENSE` + `go.mod` | yes | **yes**, with a maintenance flag not a licence flag | MIT notice for squirrel **and for both `lann/*` modules** (they ship in the binary). The NOASSERTION means our own gate must whitelist this repo explicitly with the read-on-2026-09-28 evidence, or CI will fail it every time |
| 14 | oklog/ulid | Apache-2.0 | **Apache-2.0** — root `LICENSE` is the Apache 2.0 text. No NOTICE (404). GitHub API Apache-2.0. 5,049★, pushed 2026-07-23 | `.../oklog/ulid/main/LICENSE` | yes | **conditional** | Apache-2.0 §4(a)/(b). ~300 lines of code for a ULID; if the attribution paperwork is not worth it, `crypto/rand` + the ULID spec is a half-day and zero obligation |

### 3.2 Researcher said VENDOR (copy source into our tree)

Vendoring is the **higher**-obligation path, not the lower one: the third-party text lives in our
repository, the notice must survive every future refactor, and we own the maintenance.

| # | Candidate | Claimed | **Verified** | Source URL (read 2026-09-28) | Agree? | Verdict | Obligation we take on |
|---|---|---|---|---|---|---|---|
| 15 | cmdk | MIT | **MIT** — `MIT License / Copyright (c) 2022 Paco Coursey`. `cmdk/package.json` → `"license": "MIT"`. The `pacocoursey/cmdk` raw URLs still resolve (follow `-L`), so the org rename has not broken provenance. 12,991★, **last push 2025-10-29** | `raw.githubusercontent.com/pacocoursey/cmdk/main/LICENSE.md` + `.../main/cmdk/package.json` | yes | **yes** | Put the verbatim MIT text at the head of `desktop/src/vendor/cmdk/LICENSE`, keep the copyright line in every copied file, and add a `THIRD-PARTY.md` row naming the commit SHA copied from. **Record the SHA** — without it nobody can ever tell what we changed |
| 16 | tinykeys | MIT | **MIT** — `MIT License / Copyright (c) 2020 Jamie Kyle`. GitHub API MIT. 4,099★, pushed 2026-09-25 | `.../jamiebuilds/tinykeys/main/LICENSE` | yes | **yes** | Same as above. At ~650 bytes, copying the notice costs more lines than copying the code — which is the point: copy both |
| 17 | Gitea — `dependency.go`, `issue_index.go`, `issue_xref.go` | MIT | **MIT at the root AND per-file.** Root `LICENSE`: `Copyright (c) 2016 The Gitea Authors / Copyright (c) 2015 The Gogs Authors` + MIT text. Each of the three files opens with `// Copyright <2018\|2017\|2019> The Gitea Authors. All rights reserved.` and `// SPDX-License-Identifier: MIT`. 58,188★, pushed 2026-09-27 | `.../go-gitea/gitea/main/LICENSE` and `.../main/models/issues/<file>` | yes, and now with per-file evidence | **yes** | Keep **both** copyright lines (Gitea Authors **and** Gogs Authors — the root licence carries both, and Gogs-derived code is all over this tree) plus the per-file SPDX header, in every ported file. State in `THIRD-PARTY.md` that the logic was **ported, not copied verbatim**, and name the source commit. **Do not** carry Gitea's XORM session shape into our `pgx` code and then keep calling it a port |

### 3.3 Researcher said STUDY-ONLY or REJECT — licence checked anyway

A study-only verdict still needs a licence, because "read it for ideas" and "copy a data shape" are
one careless commit apart.

| # | Candidate | Claimed | **Verified** | Source URL (read 2026-09-28) | Agree? | Verdict | Obligation |
|---|---|---|---|---|---|---|---|
| 18 | TaskChampion | MIT (About sidebar) | **MIT** — `MIT License / Copyright (c) 2020 Dustin J. Mitchell`. **The researcher never read the file; I did.** | `raw.githubusercontent.com/GothenburgBitFactory/taskchampion/main/LICENSE` | yes, with better evidence | **yes** | MIT notice if any code is taken. Reading the sync spec for ideas carries none |
| 19 | react-querybuilder | MIT (GitHub API only) | **MIT** — `LICENSE.md`: `The MIT License (MIT)`. Note the filename: `LICENSE.md`, not `LICENSE` (raw `.../main/LICENSE` **404s**, which is how the researcher ended up on the API) | `.../react-querybuilder/react-querybuilder/main/LICENSE.md` | yes | **yes** (licence). Reject on design grounds stands | MIT notice if used. Copying only the `{combinator, rules[]}` **AST shape** is an idea, not code — no obligation |
| 20 | Bugzilla | MPL-2.0 (README/About) | **MPL-2.0** — `LICENSE` opens `Mozilla Public License Version 2.0`. **The researcher never read the file; I did.** | `.../bugzilla/bugzilla/master/LICENSE` | yes | **conditional** — but study-only means no obligation is incurred | MPL-2.0 is **file-level** copyleft: if we ever modify an MPL file we must publish *that file's* source under MPL. Our own files that merely call it stay proprietary. For Perl we will never link to: irrelevant. Keep it study-only and the obligation never arrives |
| 21 | Kanboard | MIT | **MIT** — `The MIT License (MIT) / Copyright (c) Frédéric Guillot` | `.../kanboard/kanboard/main/LICENSE` | yes | **yes** | MIT notice if used. **Relicence history NOT established this pass** — the current file is MIT and that is what governs a copy taken today; do not rely on an older checkout |
| 22 | ElectricSQL | Apache-2.0 | **Apache-2.0** — root `LICENSE` is the Apache text; `packages/typescript-client/package.json` → `"license": "Apache-2.0"`. No NOTICE (404) | `.../electric-sql/electric/main/LICENSE` | yes | **conditional** (licence fine) — **reject on architecture stands**, and it is the right call | Apache-2.0 §4(a)/(b) if ever used. It would also bypass the Hub's authorisation rows, which is the real reason not to |
| 23 | vis-timeline | `Apache-2.0 OR MIT` (About sidebar) | **Dual, confirmed from the file.** `LICENSE.md`: *"This work is dual-licensed under Apache-2.0 and MIT. You can choose between one of them if you use this work."* + `SPDX-License-Identifier: Apache-2.0 OR MIT`. Both `LICENSE.Apache-2.0.txt` and `LICENSE.MIT.txt` are present in the tree. GitHub API says `NOASSERTION` | `.../visjs/vis-timeline/master/LICENSE.md` | yes, with better evidence | **yes** | **Elect MIT in writing** in `THIRD-PARTY.md` — a dual licence is only simple if you record which arm you took. Then MIT notice only |
| 24 | frappe-gantt | MIT (About sidebar) | **MIT** — `license.txt` (lowercase, no extension match for `LICENSE*`): `The MIT License (MIT) / Copyright (c) 2024 Frappe Technologies Pvt. Ltd.` **The researcher never read the file; I did.** GitHub API `spdx_id` MIT | `.../frappe/gantt/master/license.txt` | yes | **yes** | MIT notice if used. The lowercase filename is a scanner trap worth adding to `tools/licencegate`'s candidate list |
| 25 | TanStack Hotkeys | MIT | **MIT** — `MIT License / Copyright (c) 2026 Tanner Linsley`. 729★, pushed 2026-09-27 | `.../TanStack/hotkeys/main/LICENSE` | yes | **yes** (licence). "Too new to depend on" is a maturity call and I agree with it | MIT notice |
| 26 | Bleve | Apache-2.0 | **Apache-2.0** — root `LICENSE` is the Apache text. **No `NOTICE.txt` (404)** | `.../blevesearch/bleve/master/LICENSE` | yes | **conditional** | Apache-2.0 §4(a)/(b). Its own `blevesearch/*` sub-modules were **not** individually checked — do that if Postgres FTS ever fails and Bleve gets promoted |
| 27 | downshift | MIT | **MIT** — `The MIT License (MIT) / Copyright (c) 2017 PayPal` | `.../downshift-js/downshift/master/LICENSE` | yes | **yes** | MIT notice |
| 28 | kbar | MIT (GitHub API only) | **MIT** — `LICENSE` opens `MIT License`. **File read, not the API.** | `.../timc1/kbar/main/LICENSE` | yes | **yes** (licence); reject on design stands | MIT notice |
| 29 | ninja-keys | MIT (GitHub API only) | **MIT** — `LICENSE` opens `MIT License`. **File read, not the API.** | `.../ssleptsov/ninja-keys/main/LICENSE` | yes | **yes** (licence); reject on staleness stands | MIT notice |
| 30 | Fuse.js | Apache-2.0 | **Apache-2.0** — `LICENSE` is the Apache text. No NOTICE (404) | `.../krisk/Fuse/main/LICENSE` | yes | **conditional** | Apache-2.0 §4(a)/(b) if used. Rejecting it in favour of MIT-licensed uFuzzy is *also* the cheaper licence outcome |
| 31 | **Rocicorp Zero** | UNVERIFIED (per-package licence unread) | **Apache-2.0 — verified twice.** `packages/zero/package.json` at `@rocicorp/zero@1.11.0`: `"license": "Apache-2.0"`. `registry.npmjs.org/@rocicorp/zero/latest` (the **registry** host, not `npmjs.com`): `"license":"Apache-2.0"`. Monorepo root `LICENSE` is Apache-2.0 and agrees | `raw.githubusercontent.com/rocicorp/mono/main/packages/zero/package.json`; `registry.npmjs.org/@rocicorp/zero/latest` | **NO — I disagree with "unverified"** | **conditional** | Apache-2.0 §4(a)/(b). No NOTICE checked — do that before adoption. **Exclude Zero on maturity/architecture if you wish; the licence objection is withdrawn.** The lesson: `npmjs.com` 403s through Netskope, `registry.npmjs.org` does not |

### 3.4 FORBIDDEN — verified, and the verdict holds

Every row here was checked at its own licence file. **No source file of any of these was opened.**

| # | Candidate | Claimed | **Verified** | Source URL (read 2026-09-28) | Agree? | Verdict | Note |
|---|---|---|---|---|---|---|---|
| 32 | **BlockNote** | MPL-2.0 core, GPL-3.0 `xl-*`, commercial offered | **Confirmed, and worse than claimed: there is NO root `LICENSE` file at all** (`.../BlockNote/main/LICENSE` → **HTTP 404**), which is why GitHub reports `NOASSERTION`. `packages/core/package.json` → `"license": "MPL-2.0"`. `packages/xl-multi-column/LICENSE` opens `GNU GENERAL PUBLIC LICENSE / Version 3, 29 June 2007` | `.../TypeCellOS/BlockNote/main/packages/core/package.json`; `.../main/packages/xl-multi-column/LICENSE` | yes | **no** | A monorepo with **no root licence** and a GPL-3.0 subtree is the exact "permissive core, copyleft add-on" trap. Rejecting the whole project rather than policing a prefix is correct |
| 33 | **Meilisearch** | `MIT AND BUSL-1.1` | **Confirmed.** `LICENSE`: *"Part of this work fall under the Meilisearch Enterprise Edition (EE) and are licensed under the Business Source License 1.1"*. `LICENSE-EE` reads *"Business Source License 1.1 – Adapted for Meili SAS"*, **"Additional Use Grant: You may use, modify, and distribute the Licensed Work for non-production purposes only"** and *"Production use … requires a commercial license agreement"* | `.../meilisearch/meilisearch/main/LICENSE` and `.../main/LICENSE-EE` | yes | **no** | BUSL-1.1 with a non-production-only grant is forbidden outright. The EE boundary is "any file explicitly marked EE … in `enterprise_editions` modules/folders" — a boundary that moves with every release |
| 34 | **Huly Platform** | EPL-2.0 | **EPL-2.0 confirmed** — `LICENSE` opens `Eclipse Public License - v 2.0`. **But I do NOT confirm the researcher's reasoning.** They wrote that "§3.2's secondary-licence clause can pull GPL-2.0 into scope". The Secondary-Licenses machinery (lines 34, 57, 100–103, 140–143, and `Exhibit A - Form of Secondary Licenses Notice` at line 262) is **standard EPL-2.0 boilerplate present in every copy of the licence**. It only bites if the initial contributor *elected* it by including the Exhibit A notice in the files. **Whether Huly elected it is UNVERIFIED** — establishing it would require reading Huly source headers, which the rule forbids | `.../hcengineering/platform/main/LICENSE` | **verdict yes, reasoning no** | **no** | The verdict needs no such argument. **EPL-2.0 is simply not on the allowed list.** That is sufficient and is the reason to record. Do not put a shaky GPL-contagion theory in a document a lawyer may read |
| 35 | **Planka** | PLANKA Community License, source-available; was MIT (≤v1.x) → AGPL-3.0 → bespoke at v2.0 | **Confirmed as far as it can be.** `raw.../master/LICENSE` → **HTTP 404**, so there is no root licence file; GitHub API `license.spdx_id` = **`NOASSERTION`**. 12,589★, pushed 2026-09-17 | `api.github.com/repos/plankanban/planka` (authenticated), 2026-09-28 | yes | **no** | `NOASSERTION` + no root licence + a self-description of "fair-code / source available" is enough to forbid. **The three-step relicence history is UNVERIFIED by me** — I did not open the in-tree `LICENSES/` directory |
| 36 | **Focalboard** | AGPL-3.0 source; MIT only for the compiled binary; Apache-2.0 only for admin tools | **Confirmed verbatim, and it is the best find in the researcher's document.** `LICENSE.txt`: *"You are licensed to use **compiled versions** of Focalboard produced by Mattermost, Inc. under an MIT LICENSE"*; *"You may be licensed to use **source code** … 1. Under the Free Software Foundation's **GNU AGPL v.3.0** … or 2. Under a commercial license"*; Apache-2.0 only for `webapp/html-templates/`, `app-config.json`, `config.json`, `webapp/i18n/`, `server/model/`, `plugin/`. **Plus a trademark clause requiring Mattermost's prior written approval for use of the mark** | `.../mattermost/focalboard/main/LICENSE.txt` | yes | **no** | An "MIT" badge here is true of the wrong artifact. The AGPL applies to exactly the thing we would want — the source |
| 37 | **Plane** | AGPL-3.0 | **Confirmed** — `LICENSE.txt` opens `GNU AFFERO GENERAL PUBLIC LICENSE / Version 3, 19 November 2007` | `.../makeplane/plane/master/LICENSE.txt` | yes | **no** | The researcher notes two search results calling Plane "MIT-licensed". Both are wrong. This is the single best argument for the read-the-file rule |

---

## 4. The obligations register — what adoption actually costs us

The researcher's document has no obligation column. This is that column, consolidated. **None of
this is optional and all of it binds the binary KPMG installs in a customer's cloud, not just our
repository.**

| Obligation | Applies to | What we must do |
|---|---|---|
| **MIT notice** | TanStack Virtual, TanStack Table, dnd-kit, Radix, Tiptap (+ every `@tiptap/*` we install), ProseMirror (per module), uFuzzy, TanStack DB (`packages/db`), participle, expr, squirrel + `lann/builder` + `lann/ps`, cmdk, tinykeys, Gitea (ported files), and any of downshift / vis-timeline (MIT arm) / frappe-gantt / TanStack Hotkeys we take | Reproduce the verbatim MIT text **and the copyright line** in a `THIRD-PARTY.md` shipped with the product and surfaced in the desktop's licences view. One combined file is fine; a link to GitHub is not |
| **Apache-2.0 §4(a)** | React Aria, cel-go, `oklog/ulid`, Bleve, Fuse.js, Electric, Zero, `db-ivm`'s d2ts-derived portion | Ship a copy of the Apache-2.0 licence text with the distribution |
| **Apache-2.0 §4(b)** | same | Retain all copyright, patent, trademark and attribution notices in the source we redistribute |
| **Apache-2.0 §4(d) — NOTICE** | **React Aria only** (the only candidate with a `NOTICE.txt`; all others 404) | Reproduce the contents of `adobe/react-spectrum`'s `NOTICE.txt` inside our own distributed NOTICE file. **This one is routinely missed and it is the one that is actually breached.** |
| **BSD-3-Clause notices** | `antlr4-go/antlr`, `protobuf`, `golang.org/x/exp` — transitive under `cel-go` | Same as MIT, plus the no-endorsement clause; carry the text |
| **Dual-licence election** | vis-timeline (`Apache-2.0 OR MIT`) | **Write down which arm we took.** Take MIT and the Apache obligations never arrive |
| **MPL-2.0 file-level copyleft** | Bugzilla (study-only today) | If we ever modify an MPL-2.0 file, that file's source must be published under MPL-2.0. Our own files are unaffected. Staying study-only means the obligation never attaches |
| **Vendored-code hygiene** | cmdk, tinykeys, the three Gitea files | Notice at the head of every copied file; source **commit SHA** in `THIRD-PARTY.md`; a CI check that the notice still exists, because a refactor will otherwise delete it silently |
| **Provenance-of-declaration** | `@tanstack/db-ivm` | Attribute **both** MIT (as declared) and Apache-2.0 / ElectricSQL (as the fork's own README states its origin). Cheap, and it makes the ambiguity someone else's problem rather than ours |

### Licence-gate changes this audit says are required

Beyond the researcher's `@tiptap-pro/*`, `@blocknote/xl-*` and Meilisearch-enterprise denylist:

1. **Do not trust `package.json` `license` alone.** It was wrong for `@tanstack/db-ivm`. The gate
   must also look for a package `LICENSE` file and flag **`license` field present + LICENSE file
   absent** as a manual-review case.
2. **Widen the licence-file filename match**: `COPYING` (participle), `LICENSE.md`
   (react-querybuilder, Tiptap, cmdk), `LICENSE.txt` (Focalboard, Plane), **`license.txt` lowercase**
   (frappe-gantt), `LICENSE-EE` (Meilisearch), `LICENSE.Apache-2.0.txt` / `LICENSE.MIT.txt`
   (vis-timeline). A gate keyed on `LICENSE` misses six of the projects in this document.
3. **Treat a missing root LICENSE as a hard fail, not an unknown.** BlockNote and Planka both have
   none, and both are forbidden.
4. **Allowlist `Masterminds/squirrel` explicitly** with this document as the evidence — GitHub's own
   classifier returns `NOASSERTION` and will fail the gate on every run otherwise.
5. **Require a NOTICE check for every Apache-2.0 dependency**, not just a licence check.

---

## 5. What I could not verify — do not read past this as settled

1. **Radix per-component manifests.** Root MIT confirmed; the individual `@radix-ui/react-*`
   `package.json` `license` fields were **not** enumerated. Do it before install — it is the same
   check that produced this audit's biggest finding.
2. **ProseMirror's other ~12 modules.** Only `prosemirror-state` was read. `-model`, `-view`,
   `-transform`, `-commands`, `-history`, `-keymap`, `-tables` etc. each have their own `LICENSE`
   and were **not** read.
3. **`@hocuspocus/*`** (Tiptap's collaboration server) — not checked at all. If collaboration is
   ever switched on, check it first.
4. **`cel.dev/cel-go`.** `google/cel-go`'s `go.mod` now requires a module at a `cel.dev` path. The
   licence at that path was **not** read. Verify before pinning.
5. **Bleve's `blevesearch/*` sub-modules** — not individually checked.
6. **Rocicorp Zero's NOTICE file** — not checked (the licence itself is now verified).
7. **Kanboard's relicence history** — the current file is MIT; whether it was ever AGPL was not
   established this pass. Irrelevant for a copy taken today, relevant for an old vendored copy.
8. **Whether Huly elected EPL-2.0 Secondary Licenses** — unverifiable without reading its source,
   which the rule forbids. Verdict does not depend on it.
9. **Planka's in-tree `LICENSES/` directory** — not opened. `NOASSERTION` + no root licence is
   enough to forbid without it.
10. **Gitea's *other* files.** The three named files are per-file MIT. Gitea is a large tree with
    vendored assets; if the port grows beyond those three, re-check each new file's header.
11. **Every day estimate in `01-work-layer.md`.** Out of scope for a licence audit, and still
    unmeasured.

## 6. One process finding

The researcher's own "§7 unverified" list named the Tiptap package-level check as outstanding —
and then the document's summary line said **adopt** anyway. Both of the errors this audit found
(TanStack DB's `db-ivm`, and Zero excluded on a licence that is actually fine) are **sub-package**
errors, found by reading one level below the repository root. **Make "read the manifest of the
package you will install, not the root of the repo it lives in" the standing rule**, and put it in
`tools/licencegate` so it is not a person's memory.

