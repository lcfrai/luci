# Licence audit — Phase 3 candidates (git, worktrees, lanes, convergence)

**Audited 2026-09-28** by an adversarial licence auditor against
`docs/superpowers/planning/2026-09-28-oss-adoption/04-git-lanes.md`.
Method: fetch the actual `LICENSE`/`COPYING` blob from the actual repository over HTTPS
(`curl --ssl-no-revoke -L`), read the identifying line, then check for a relicence, a dual
licence, a commercial edition, sub-package divergence, and copyleft in the direct dependencies
that would ship in our artifact. Nothing here is taken from a GitHub badge, a blog or memory.

**Scope note.** No source from any AGPL/GPL project was opened. `git` and `libgit2` were read
**only at their `COPYING` file**, which is the licence text itself — no program source.

---

## Verdict table

| # | Candidate | Claimed | Verified (SPDX) | Licence blob read | Verdict | Obligation if we take it |
|---|---|---|---|---|---|---|
| 1 | git binary (subprocess) | GPL-2.0, "imposes no obligation" | **GPL-2.0-only** | `git/git@master/COPYING` | **conditional** — *auditor disagrees with "no obligation"* | We **distribute** git inside the sandbox image and (probably) the desktop bundle. GPLv2 §3 written offer / source availability for git itself. No effect on PanaceaApp source. |
| 2 | go-git/go-git | Apache-2.0 | **Apache-2.0** | `go-git/go-git@master/LICENSE` | **yes** | Apache §4: keep LICENSE + NOTICE, state changes. |
| 3 | Gitea `modules/git` | MIT | **MIT** (repo root) | `go-gitea/gitea@main/LICENSE` | **conditional** | MIT notice per vendored file; *and* Gitea root MIT does **not** automatically cover every file in the tree — per-file header must be checked for the exact files vendored. |
| 4 | bluekeyes/go-gitdiff | MIT | **MIT** | `bluekeyes/go-gitdiff@master/LICENSE` | **yes** | MIT notice. |
| 5 | sergi/go-diff | MIT | **MIT** (grant text; no "MIT" title line) | `sergi/go-diff@master/LICENSE` | **yes** | MIT notice; carry the upstream diff-match-patch attribution. |
| 6 | microsoft/azure-devops-go-api | MIT | **MIT** | `microsoft/azure-devops-go-api@dev/LICENSE` | **yes** | MIT notice. Maturity is the risk, not the licence. |
| 7 | google/go-github | BSD-3-Clause | **BSD-3-Clause** | `google/go-github@master/LICENSE` | **yes** | BSD-3 notice + no-endorsement clause. |
| 8 | fluxcd/go-git-providers | Apache-2.0 | **Apache-2.0** | `fluxcd/go-git-providers@main/LICENSE` | **yes** (licence); reject on fit | Apache §4 if adopted. |
| 9 | google/git-appraise | Apache-2.0 (unfetched) | **Apache-2.0** | `google/git-appraise@master/LICENSE` | **yes** | Apache §4. |
| 10 | jj (Jujutsu) | Apache-2.0 | **Apache-2.0** | `jj-vcs/jj@main/LICENSE` | **yes** (licence); do not adopt on fit | Apache §4. |
| 11 | arxanas/git-branchless | MIT OR Apache-2.0 | **MIT** verified; dual claim only half-verified | `arxanas/git-branchless@master/LICENSE-MIT` | **conditional** | Its *binary* links libgit2 (GPL-2.0-with-linking-exception). Reading it is free; shipping it is not the same question. |
| 12 | dagger/container-use | Apache-2.0 | **Apache-2.0** | `dagger/container-use@main/LICENSE` | **yes** (licence); study-only on fit | Apache §4. |
| 13 | aviator-co/av | MIT (unfetched) | **MIT**, "Copyright (c) 2022 Aviator Technologies, Inc" | `aviator-co/av@master/LICENSE` | **yes** | MIT notice. Single-vendor project — relicence risk is non-zero; pin a commit. |
| 14 | ejoffe/spr | MIT (unfetched) | **MIT**, "Copyright (c) 2022 InigoLabs" | `ejoffe/spr@master/LICENSE` | **yes** | MIT notice. |
| 15 | git-town/git-town | MIT (unfetched) | **MIT (MIT/Expat)** | `git-town/git-town@main/LICENSE` | **yes** | MIT notice. |
| 16 | git-ecosystem/git-bundle-server | MIT | **MIT** | `git-ecosystem/git-bundle-server@main/LICENSE` | **yes** | MIT notice. |
| 17 | google/gitprotocolio | Apache-2.0 | **Apache-2.0** | `google/gitprotocolio@master/LICENSE` | **yes** | Apache §4. |
| 18 | Wilfred/difftastic | MIT | **MIT** at repo root, "Copyright (c) 2021-2025 Wilfred Hughes" | `Wilfred/difftastic@master/LICENSE` | **conditional** | Root MIT covers Wilfred's code only; the shipped binary embeds ~50 vendored tree-sitter grammars with their own licences. Per-grammar audit required before shipping the binary. |
| 19 | git2go / libgit2 | git2go MIT; libgit2 GPL-2.0 + linking exception | **git2go MIT**; **libgit2 GPL-2.0-only WITH a linking exception** | both `COPYING`/`LICENSE` read | **no** (agrees with researcher) | Exception permits linking a compiled build; *modifying* libgit2 or shipping it unlinked stays GPL-2.0. Not worth the argument. |

All URLs, quotes and per-candidate reasoning are below.

---

## 1. git binary — GPL-2.0-only — **CONDITIONAL, and I disagree with the researcher**

- Blob read: `https://raw.githubusercontent.com/git/git/master/COPYING`, HTTP 200, 2026-09-28, 360 lines.
- Identifying text, verbatim: *"Note that the only valid version of the GPL as far as this project is
  concerned is _this_ particular version of the license (ie v2, not v2.2 or v3.x or whatever), unless
  explicitly otherwise stated."* followed by the full GNU General Public License Version 2, June 1991.
  SPDX: **GPL-2.0-only** (not `-or-later`; the preamble explicitly refuses the "or later" reading).
- **The researcher's claim is "invoked, never linked; imposes no obligation on PanaceaApp" and
  "No licence issue." The first half is right. The second half is wrong.**
  - *Invocation* across a process boundary with `exec` does not make PanaceaApp a derivative work.
    That part of the analysis holds and is the same conclusion every commercial IDE reaches.
  - *Distribution* is a separate obligation and we will be doing it. `OD-1` puts a git client inside
    the Kata sandbox image; the desktop cockpit ships a git for local worktrees (`OD-2`, `OD-20`).
    Shipping the image to a customer's Azure subscription is conveying a GPL-2.0 binary. **GPLv2 §3
    then requires the complete corresponding source, or a written offer valid for three years, for
    the exact git version in the image.** It attaches to git, not to our code.
  - On Windows the cockpit would bundle Git for Windows, which is git plus an MSYS2/MinGW runtime
    carrying further GPL and LGPL components — a bigger notice set than upstream git alone.
- **Obligation we take on, concretely:** (a) a NOTICE/attribution entry naming git and its version;
  (b) a source mirror or written offer pinned per release; (c) a rule that nobody modifies git and
  statically links it into a Go binary. (a)–(c) are cheap and normal. Calling them "no licence issue"
  is what gets a regulated-customer release stopped in legal review.
- **Verdict: conditional — adopt as a subprocess, and add the distribution obligation to the
  release checklist now, while it costs nothing.**

---

## 2. go-git/go-git — Apache-2.0 — **YES (agrees)**

- Blob read: `https://raw.githubusercontent.com/go-git/go-git/master/LICENSE`, HTTP 200, 2026-09-28,
  201 lines. Verbatim Apache-2.0 text; line 189, inside the standard appendix:
  `Copyright 2018 Sourced Technologies, S.L.`
  **The researcher's quote checks out.** GitHub API `spdx_id` agrees: `Apache-2.0`.
- **No `NOTICE` file** — `master/NOTICE` returns HTTP 404. Apache-2.0 §4(d) only bites when the work
  carries a NOTICE; there is nothing to propagate. Our obligation is §4(a)+(b): ship the LICENSE and
  mark modified files if we fork.
- **Relicence check:** the project moved from `src-d/go-git` when source{d} folded, and kept
  Apache-2.0; the current LICENSE still carries the src-d copyright, which is consistent with a
  transfer, not a relicence. No CLA, no commercial edition, no change-date clause found.
- **Direct dependencies at v5.19.2** (`go.mod` read at the v5.19.2 tag, 2026-09-28) — none copyleft:
  `dario.cat/mergo` BSD-3 · `ProtonMail/go-crypto` BSD-3 (Go Authors) · `emirpasic/gods` BSD-2 ·
  `gliderlabs/ssh` BSD-3 · `go-git/gcfg` BSD-3 · `go-git/go-billy` Apache-2.0 ·
  `golang/groupcache` Apache-2.0 · `jbenet/go-context` MIT · `kevinburke/ssh_config` MIT ·
  `pjbgf/sha1cd` Apache-2.0 · `sergi/go-diff` MIT · `skeema/knownhosts` Apache-2.0 ·
  `xanzy/ssh-agent` Apache-2.0 · `cloudflare/circl` BSD-3 · `Microsoft/go-winio` MIT ·
  `cyphar/filepath-securejoin` BSD-3 (file is `LICENSE.BSD`, not `LICENSE`) ·
  `gopkg.in/warnings.v0` BSD-3 · `golang.org/x/*` BSD-3. **Clean.**
- Maturity: 7,782 stars, last push 2026-09-27, latest release **v5.19.2, 2026-07-29**. Active.
- **Verdict: yes.** The researcher's "go-git may read, git writes" rule is a correctness rule, not a
  licence rule, and I have no licence basis to overturn their study-only framing.

---

## 3. Gitea `modules/git` — MIT — **YES on licence; the vendoring plan is stale**

- Blob read: `https://raw.githubusercontent.com/go-gitea/gitea/main/LICENSE`, HTTP 200, 2026-09-28.
  Opens `Copyright (c) 2016 The Gitea Authors` / `Copyright (c) 2015 The Gogs Authors`, then the
  verbatim MIT grant. GitHub `spdx_id`: `MIT`.
- **Sub-package check — this is the one that mattered, and it passes.** A root MIT in a 58k-star
  monorepo does not automatically cover every file, so I read the headers of the files the researcher
  proposed vendoring. Each carries a per-file SPDX tag, for example
  `modules/git/repo.go`: `// Copyright 2015 The Gogs Authors...` / `// SPDX-License-Identifier: MIT`;
  `modules/git/blame.go`: `// Copyright 2025 The Gitea Authors...` / `// SPDX-License-Identifier: MIT`;
  `modules/git/repo_commit.go`: same. **Per-file MIT confirmed, not inferred from the root.**
- **But the vendoring target has moved.** `modules/git/command.go` returns **HTTP 404 at `main`**
  (2026-09-28) while `modules/git/git.go` returns 200. The researcher's plan to "vendor the two or
  three files that matter" names a file that no longer exists upstream. Re-identify the files against
  a pinned tag (`v1.27.3`, released 2026-08-29) before writing the plan, and pin that tag in the
  vendor comment — this is exactly the "vendored code goes stale" risk they flagged, arriving before
  we vendored anything.
- **Obligation:** MIT notice retained in each vendored file plus a NOTICE entry naming Gitea, Gogs
  and the commit. Do not copy Gitea files that lack an MIT SPDX header without re-checking.
- Maturity: 58,188 stars, last push 2026-09-27, latest release v1.27.3 (2026-08-29).
- **Verdict: conditional — MIT is real and per-file, but the file list must be rebuilt at a pinned tag.**

---

## 4. bluekeyes/go-gitdiff — MIT — **YES (agrees). Cleanest candidate in the set.**

- Blob read: `https://raw.githubusercontent.com/bluekeyes/go-gitdiff/master/LICENSE`, HTTP 200,
  2026-09-28. Line 1: `MIT License`; line 3: `Copyright (c) 2019 Billy Keyes`. GitHub `spdx_id`: `MIT`.
- **Dependency check: `go.mod` has no `require` block at all** — module path, `go 1.21`, nothing else.
  Zero direct dependencies means zero transitive licence surface. Nothing else here comes close.
- No CLA, no second edition, no change-date clause.
- Maturity: 156 stars (small), last push 2026-08-08, latest release **v0.9.0, 2026-07-18**.
  Low star count, but a release inside the last three months and a single maintainer whose entire
  surface is vendorable under MIT if he stops.
- **Verdict: yes.** Obligation: one MIT notice.

---

## 5. sergi/go-diff — MIT — **YES on licence; flag the maintenance, not the licence**

- Blob read: `https://raw.githubusercontent.com/sergi/go-diff/master/LICENSE`, HTTP 200, 2026-09-28.
  **Note for the record: the file has no "MIT License" title line.** It opens
  `Copyright (c) 2012-2016 The go-diff Authors. All rights reserved.` and then carries the verbatim
  MIT grant ("Permission is hereby granted, free of charge... subject to the following conditions").
  That is MIT/Expat by text; GitHub's `spdx_id` agrees: `MIT`.
- Derivation: it is a Go port of Neil Fraser's `diff-match-patch` (Apache-2.0). The repo carries an
  `AUTHORS` file ("the official list of go-diff authors for copyright purposes") and a `CONTRIBUTORS`
  file. **Obligation: ship LICENSE + AUTHORS, and keep the upstream diff-match-patch attribution.**
- Dependencies: test-only (`testify`, `check.v1`, `go-spew`, `yaml.v2`) — none ship.
- **Maturity is the real finding: no GitHub release has ever been tagged** (`releases/latest` empty)
  and the **last push was 2025-06-05 — about 15 months ago**, outside the 12-month bar this research
  set for itself. The researcher's "adopt where needed" is legally sound and practically fine only
  because go-git already pins it transitively (`v1.3.2-0.20230802210424-...`), so adopting it adds no
  new supply-chain surface — but it should be recorded as an unmaintained dependency, and it is 2k
  lines of MIT that we can vendor the day it breaks.
- **Verdict: yes (licence). Record the staleness.**

---

## 6. microsoft/azure-devops-go-api — MIT — **YES on licence; two findings the researcher missed**

- Blob read: `https://raw.githubusercontent.com/microsoft/azure-devops-go-api/dev/LICENSE`, HTTP 200,
  2026-09-28. Indented, opens `MIT License` / `Copyright (c) Microsoft Corporation. All rights
  reserved.` then the verbatim MIT grant. GitHub `spdx_id`: `MIT`. Claim confirmed.
- **Finding 1 — sub-package divergence, in our favour but worth writing down.** The Go module we
  would import is `github.com/microsoft/azure-devops-go-api/azuredevops/v7`, a *subdirectory* module
  (its own `go.mod`, read 2026-09-28). **There is no LICENSE file in `azuredevops/` or in
  `azuredevops/v7/` — both return HTTP 404.** The grant still reaches us (the Go module zip carries
  licence files from ancestor directories, and MIT at the repo root covers the tree), but **any SBOM
  or `go-licenses`-style scanner that looks inside the module directory will report "no licence
  found"**. We must add the attribution to NOTICE by hand and pre-empt the scanner finding, or it
  will surface as an unlicensed dependency in a customer's supply-chain review.
- **Finding 2 — the maturity claim is weaker than "adopt with a fallback plan" suggests.** The
  latest GitHub release is **`azuredevops/v7.1.0`, published 2023-04-17 — three years and five
  months old** as of today. The repo was pushed 2026-06-10, so it is not abandoned, but it has 223
  stars and no tagged release in over three years. Against a research bar of "a release in the last
  12 months", this fails. The fallback (hand-rolled REST against the documented ADO API) should be
  costed as the *likely* path, not the contingency, because `OD-4` makes ADO first-class.
- Dependencies: `github.com/google/uuid v1.1.1` only — BSD-3-Clause, verified at
  `google/uuid@master/LICENSE`. Clean.
- **Verdict: yes on licence (MIT, obligation = one notice we must add manually). Downgrade the
  confidence on maturity.**

---

## 7. google/go-github — BSD-3-Clause — **YES (agrees)**

- Blob read: `https://raw.githubusercontent.com/google/go-github/master/LICENSE`, HTTP 200,
  2026-09-28. Opens `Copyright (c) 2013 The go-github AUTHORS. All rights reserved.` and carries the
  three BSD clauses including *"Neither the name of Google Inc. nor the names of its contributors may
  be used to endorse or promote products derived from this software without specific prior written
  permission."* SPDX: **BSD-3-Clause**. GitHub `spdx_id` agrees.
- **Obligation, and it is not just a notice:** clause 3 is a no-endorsement term. Marketing copy for
  PanaceaApp must not imply Google endorses it because we use go-github. Clauses 1–2 require the
  copyright notice in source and in binary documentation.
- Dependencies (`go.mod` at master, module `github.com/google/go-github/v92`, read 2026-09-28):
  `google/go-cmp` (BSD-3) and `google/go-querystring` (BSD-3, verified at `master/LICENSE`).
  Two deps, both BSD-3. Clean.
- Maturity: 11,309 stars, last push 2026-09-24, module at **v92** (they version by GitHub API
  generation rather than GitHub Releases, so `releases/latest` is empty — not a staleness signal).
- **Verdict: yes.**

---

## 8. fluxcd/go-git-providers — Apache-2.0 — **YES on licence; reject on fit (agrees)**

- Blob read: `https://raw.githubusercontent.com/fluxcd/go-git-providers/main/LICENSE`, HTTP 200,
  2026-09-28. Verbatim Apache-2.0. The researcher cited only the repo URL, not the licence file;
  it now checks out. CNCF project, so an Apache-2.0 + DCO posture is expected and found.
- Maturity: **UNVERIFIED** — the GitHub API rate-limited this session before reaching it (see
  "What I could not verify").
- **Verdict: yes on licence, reject on fit** — it has no Azure DevOps provider, and `OD-4` makes ADO
  first-class, so adopting it means writing the half we actually need anyway. I agree with the
  researcher's reject; the licence was simply never the blocker.

---

## 9. google/git-appraise — Apache-2.0 — **YES (upgrades the researcher's "as GitHub reports")**

- Blob read: `https://raw.githubusercontent.com/google/git-appraise/master/LICENSE`, HTTP 200,
  2026-09-28. Verbatim Apache-2.0 template. The researcher explicitly flagged this one as
  "LICENSE file not separately fetched" — **now fetched; the claim holds.**
- Obligation if we ever import rather than imitate: Apache §4(a)+(b), plus the §3 patent grant we
  receive (a positive, not a cost).
- Maturity: **UNVERIFIED** (rate limit). Treat as dormant pending a check.
- **Verdict: yes on licence.** Copying its *pattern* (append-only notes under a `refs/notes/...`
  namespace with `cat_sort_uniq`) carries no licence obligation at all, since a data layout is not
  copyrightable expression — so the researcher's "study-only" costs us nothing either way.

---

## 10. jj (Jujutsu) — Apache-2.0 — **YES on licence; do not adopt (agrees)**

- Blob read: `https://raw.githubusercontent.com/jj-vcs/jj/main/LICENSE`, HTTP 200, 2026-09-28.
  Verbatim Apache-2.0, no filled-in appendix copyright line — consistent with the researcher's note.
- **Google-origin project: contributions require the Google CLA.** That is an obligation on
  *contributors*, not on consumers. It matters only if we upstream a patch — at which point a KPMG
  CLA sign-off is required, which is a process cost worth knowing before someone volunteers a fix.
- Maturity: **UNVERIFIED** (rate limit), though jj is visibly active in 2026.
- **Verdict: yes on licence, do not adopt on architecture** — adopting jj replaces git as the model
  of record, contradicting `OD-2` and `OD-4`. Agrees with the researcher.

---

## 11. arxanas/git-branchless — MIT OR Apache-2.0 — **CONDITIONAL, and the dual claim needed both halves**

- Blobs read, 2026-09-28: `master/LICENSE-MIT` HTTP 200 (`Copyright (c) 2021 Individual
  contributors` + MIT grant) and **`master/LICENSE-APACHE` HTTP 200** (verbatim Apache-2.0). The
  researcher cited only the MIT half. Both halves now exist, and
  `git-branchless-lib/Cargo.toml` line 6 states `license = "MIT OR Apache-2.0"` — **dual licence
  confirmed from package metadata, not from a badge.** We would take the MIT half.
- **The finding the researcher did not make: `git-branchless-lib/Cargo.toml` line 58 declares
  `git2 = { workspace = true }`.** The `git2` crate is a binding over **libgit2, which is
  GPL-2.0-only with a linking exception** (see §19). So:
  - *Reading* git-branchless source: free, MIT.
  - *Importing* its Rust crates into something we ship: inherits the libgit2 link, and the exception
    permits the link but the combined artifact's obligations need real advice.
  - *Shipping the `git-branchless` binary* inside the Kata sandbox image: that is distributing a
    libgit2-linked GPL-2.0 work, with the same §3 source-offer obligation as §1 above.
  "Study-only" must therefore be read as **read the source, do not put the binary in the image**.
- Maturity: **UNVERIFIED** (rate limit).
- **Verdict: conditional.** Fine to read; not fine to ship without a decision.

---

## 12. dagger/container-use — Apache-2.0 — **YES (agrees)**

- Blob read: `https://raw.githubusercontent.com/dagger/container-use/main/LICENSE`, HTTP 200,
  2026-09-28. Verbatim Apache-2.0.
- **Single-vendor relicence risk, stated plainly:** container-use is a Dagger Inc project. Dagger's
  engine is also Apache-2.0 and Dagger uses DCO rather than a CLA, which *reduces* the practical
  ability to relicence (no copyright assignment). No BUSL/Elastic change-date clause, no Commons
  Clause, no "enterprise edition" file found in the repo root. Even so: a grant already made under
  Apache-2.0 for a given commit is irrevocable, so **pin a commit** and the risk is bounded.
- Maturity: **UNVERIFIED** (rate limit).
- **Verdict: yes on licence, study-only on fit.** Agrees with the researcher. Its shape is phase 3's
  shape; its engine dependency is the reason not to adopt it wholesale, and that is an architecture
  call, not a licence one.

---

## 13. aviator-co/av — MIT — **YES (upgrades "as GitHub reports")**

- Blob read: `https://raw.githubusercontent.com/aviator-co/av/master/LICENSE`, HTTP 200, 2026-09-28.
  `MIT License` / `Copyright (c) 2022 Aviator Technologies, Inc`. The researcher did not fetch it;
  **it checks out.**
- **Relicence check — this was the highest-risk name in the list, because Aviator is a commercial
  company selling a merge-queue product and `av` is the free CLI in front of it.** Searched Aviator's
  docs, changelog and GitHub org (2026-09-28): the CLI is described as free and open source, usable
  without an Aviator account, and I found **no** licence change, Commons Clause, BUSL change-date or
  separate commercial edition of the CLI repo. Nothing in the repo root indicates a dual edition.
  Residual risk is future-only and neutralised by pinning a commit.
- Maturity: **UNVERIFIED** (rate limit).
- **Verdict: yes on licence, study-only on fit** (GitHub-only, breaking `OD-4`). Agrees.

---

## 14. ejoffe/spr — MIT — **YES (upgrades "as GitHub reports")**

- Blob read: `https://raw.githubusercontent.com/ejoffe/spr/master/LICENSE`, HTTP 200, 2026-09-28.
  `MIT License` / `Copyright (c) 2022 InigoLabs`. The researcher did not fetch it; **it checks out.**
- Note the copyright holder is a company (InigoLabs), not the individual in the repo path — normal,
  but it means the NOTICE entry must name InigoLabs, not "ejoffe".
- Maturity: **UNVERIFIED** (rate limit).
- **Verdict: yes on licence, study-only on fit.** Agrees.

---

## 15. git-town/git-town — MIT — **YES (upgrades "as GitHub reports")**

- Blob read: `https://raw.githubusercontent.com/git-town/git-town/main/LICENSE`, HTTP 200,
  2026-09-28. `The MIT License (MIT)` / `Copyright (c) 2014-2024 the Git Town creators`.
  The researcher did not fetch it; **it checks out.**
- No CLA, no paid edition found in the repo root. Community-owned copyright ("the Git Town
  creators") makes a unilateral relicence harder than a single-vendor project.
- Maturity: **UNVERIFIED** (rate limit).
- **Verdict: yes on licence.**

---

## 16. git-ecosystem/git-bundle-server — MIT — **YES (agrees)**

- Blob read: `https://raw.githubusercontent.com/git-ecosystem/git-bundle-server/main/LICENSE`,
  HTTP 200, 2026-09-28. `Git Bundle Server` / `Copyright © Git Bundle Server contributors` /
  `MIT License` + verbatim grant.
- **Worth knowing: this is a Microsoft-originated project in the `git-ecosystem` org, and it is MIT,
  not GPL — i.e. it is a separate work from git itself and carries none of §1's obligations.** That
  is a genuinely useful distinction if we ever ship bundle serving.
- Maturity: **UNVERIFIED** (rate limit).
- **Verdict: yes on licence, study-only on fit.** Agrees.

---

## 17. google/gitprotocolio — Apache-2.0 — **YES (agrees)**

- Blob read: `https://raw.githubusercontent.com/google/gitprotocolio/master/LICENSE`, HTTP 200,
  2026-09-28. Verbatim Apache-2.0.
- The researcher recorded it as archived with unverified protocol-v2 coverage. I did not re-check the
  archive flag (rate limit) and do not dispute it. Archived ≠ unlicensed: the Apache-2.0 grant stands
  for the last commit regardless.
- **Verdict: yes on licence, study-only on fit.** Agrees.

---

## 18. Wilfred/difftastic — MIT at the root — **CONDITIONAL, and I disagree with a flat "MIT"**

- Blobs read, 2026-09-28: `master/LICENSE` HTTP 200 — `MIT License` /
  `Copyright (c) 2021-2025 Wilfred Hughes`; and `master/Cargo.toml` line 6 — `license = "MIT"`,
  `version = "0.72.0"`, binary name `difft`. **Both the licence file and the package metadata say
  MIT, so the researcher's claim is correct for Wilfred's own code.**
- **The claim is incomplete, and the gap is the one that matters for a shipped product.** `difft`
  is a Rust binary that statically links a large set of tree-sitter grammars. Four are vendored
  in-tree at `vendored_parsers/` (listing read 2026-09-28):
  - `tree-sitter-kotlin/LICENSE` — `The MIT License (MIT)`, `Copyright (c) 2019 fwcd` ✔
  - `tree-sitter-latex/LICENSE` — `MIT License`, `Copyright (c) 2021 Patrick Förster` ✔
  - `tree-sitter-smali/LICENSE` — `The MIT License (MIT)`, `Copyright (c) 2023 Yotam Nachum, Amaan Qureshi` ✔
  - **`tree-sitter-janet-simple` — no licence file found.** `LICENSE`, `LICENSE.md`, `LICENSE.txt`
    and `COPYING` all returned HTTP 404 at that path. **UNRESOLVED.**
  The remaining ~45 grammars arrive as Cargo dependencies, each with its own licence, and were
  **not** audited here.
- **Consequence:** reading difftastic's source and copying its *ideas* is free under MIT. Putting the
  `difft` binary into the Kata sandbox image is an aggregation of ~50 separately-licensed works, at
  least one of which has no discoverable licence grant. That is exactly the shape of the Paperdesign
  incident — a permissive headline over an unaudited interior.
- **Verdict: conditional.** Study-only is safe. **Before `difft` ever ships in an image, run a full
  `cargo-deny`/`cargo-about` licence sweep of the grammar crates and resolve `tree-sitter-janet-simple`.**
- Maturity: **UNVERIFIED** (rate limit); `Cargo.toml` reports v0.72.0 at master.

---

## 19. git2go / libgit2 — MIT binding over GPL-2.0-only-with-linking-exception — **NO (agrees)**

- Blobs read, 2026-09-28:
  - `https://raw.githubusercontent.com/libgit2/git2go/main/LICENSE` HTTP 200 — `The MIT License` /
    `Copyright (c) 2013 The git2go contributors`. **git2go itself is MIT. Confirmed.**
  - `https://raw.githubusercontent.com/libgit2/libgit2/main/COPYING` HTTP 200 — the core is
    **GPL-2.0-only** (`the only valid version of the GPL ... is _this_ particular version ... (ie v2,
    not v2.2 or v3.x or whatever)`), preceded by a section headed `LINKING EXCEPTION`, verbatim:
    > *"In addition to the permissions in the GNU General Public License, the authors give you
    > unlimited permission to link the compiled version of this library into combinations with other
    > programs, and to distribute those combinations without any restriction coming from the use of
    > this file. (The General Public License restrictions do apply in other respects; for example,
    > they cover modification of the file, and distribution when not linked into a combined
    > executable.)"*
  **The researcher's characterisation is exactly right.** SPDX: `GPL-2.0-only WITH
  GCC-exception-2.0`-style linking exception (libgit2's own text, not a registered SPDX exception id).
- **What the exception does and does not buy us:** linking a *compiled, unmodified* libgit2 into
  PanaceaApp is permitted and does not make our code GPL. It does **not** cover (a) modifying libgit2
  and distributing the modification, or (b) distributing libgit2 *not* linked into a combined
  executable — e.g. dropping `libgit2.so`/`git2.dll` into a sandbox image as a standalone file, which
  is precisely what a container build does. That second carve-out is the trap.
- **Bundled third-party code inside libgit2's own COPYING** (read at the same blob): `deps/zlib`
  under the Zlib licence; `deps/pcre2` under the PCRE2 "BSD" licence (with the JIT support note).
  Both permissive — but each is a further notice obligation.
- Additional non-licence cost: cgo, a vendored C build, and a cross-platform toolchain across a
  Windows desktop and an AzureLinux microVM.
- **Verdict: no. Agrees with the researcher, for the same reasons plus the unlinked-distribution
  carve-out they did not name.**

---

## Where I disagree with the researcher

1. **git is not obligation-free (§1).** "Invoked, never linked; imposes no obligation on PanaceaApp"
   and "No licence issue" conflate *derivation* with *distribution*. We ship git in the sandbox image
   and the desktop bundle, so GPLv2 §3 attaches to git itself: source availability or a written offer,
   per release, per version. It does not touch our code. Put it on the release checklist now.
2. **difftastic's MIT is a root licence over an unaudited interior (§18).** Root `LICENSE` and
   `Cargo.toml` both say MIT and that is correct for Wilfred's code, but `difft` links ~50 grammars
   and **`vendored_parsers/tree-sitter-janet-simple` has no licence file at any of the four standard
   names**. Reading is safe; shipping the binary needs a `cargo-deny` sweep first.
3. **azure-devops-go-api has no licence file inside the module we would import (§6)**, and its last
   tagged release is **2023-04-17**. The grant is fine; the SBOM story and the maturity story are not.
   The hand-rolled REST fallback should be the baseline plan, not the contingency.
4. **The Gitea vendoring target has already moved (§3).** `modules/git/command.go` returns 404 at
   `main`. Per-file MIT SPDX headers are confirmed on the files that do exist, so the licence claim
   stands; the *plan* needs rebuilding against pinned tag `v1.27.3`.
5. **git-branchless links libgit2 (§11).** `git-branchless-lib/Cargo.toml` declares `git2`, so the
   dual MIT/Apache-2.0 headline covers the source but not the shipped binary. "Study-only" has to
   mean *read it*, not *put it in the image*.
6. **sergi/go-diff has never had a tagged release and was last pushed 2025-06-05 (§5)** — outside the
   research's own 12-month bar. Adopting it is still right, because go-git already pins it, but it
   should be listed as an unmaintained dependency with a vendor-if-abandoned note.

Where the researcher marked something adopt or vendor and I found nothing wrong, I have said so
plainly: **go-git, go-gitdiff, go-github and the Gitea licence itself all survived the audit
unchanged**, and four candidates the researcher flagged as "licence not separately fetched"
(`git-appraise`, `av`, `spr`, `git-town`) were fetched and **all four claims held**.

## What I could not verify

- **Stars, last-push and latest-release for 12 of the 19 candidates.** The unauthenticated GitHub
  API rate-limited this session after six repositories (2026-09-28). Verified metadata exists for
  go-git, Gitea, go-gitdiff, sergi/go-diff, azure-devops-go-api and go-github only. Every other
  maturity statement above is marked **UNVERIFIED** and must not be quoted as evidence.
- **The ~45 tree-sitter grammar crates difftastic pulls via Cargo** — not audited. See §18.
- **Per-file headers across the whole Gitea tree** — only the four `modules/git` files named by the
  researcher were checked. Any additional file vendored later must be re-checked individually.
- **git's own sub-component licences** (e.g. LGPL-licensed pieces historically present in
  `compat/`) — not enumerated. `COPYING` is GPL-2.0-only for the project as a whole; a full
  component sweep belongs in the release-engineering task, not here.

## Standing rules this audit suggests

1. Pin a commit or tag for every adopted dependency, and record it in NOTICE with the licence text.
   An irrevocable grant at a pinned commit is what neutralises single-vendor relicence risk.
2. Any binary placed in the sandbox image goes through a licence sweep of its *linked* dependencies,
   not just its root LICENSE. That rule would have caught both §1 and §18.
3. A module in a subdirectory needs its attribution added by hand (§6), because scanners look in the
   module directory and Go's zip semantics are not obvious to a reviewer.

