# Open-source adoption research — Phase 3: git, worktrees, lanes and convergence

**Researched 2026-09-28** by the OSS-adoption workflow. Read-only pass over public sources; no repository
in this tree was modified and no code was run.

**The headline.** There is **no open-source library that can replace the git binary** for what phase 3
does. The single most load-bearing fact found: **go-git, the only mature pure-Go git implementation,
does not implement `git bundle`, does not implement `update-ref`, does not implement non-fast-forward
merge, and does not speak pack-protocol v2** — which is, precisely, the list of operations phase 3 is
built on (R4's bundle crossing, A5's compare-and-swap land, `merge --no-ff`, modern host transport).
The useful open source for phase 3 is therefore **not** a git replacement. It is, in order of value:

1. a **hardened `exec.Command` wrapper around the real git binary**, of which Gitea's is the best
   permissively-licensed example to learn from or vendor;
2. **diff/patch parsing and rendering** libraries (`bluekeyes/go-gitdiff`, `sergi/go-diff`,
   `difftastic`) for the Files lens and the conflict view, which are real, bounded savings;
3. **host API clients** for Azure DevOps and GitHub (`microsoft/azure-devops-go-api`,
   `google/go-github`) behind our own one-host interface — the boring, correct answer to `OD-4`;
4. **study-only prior art** for lane convergence (`jj`, `git-branchless`, `git-appraise`,
   `container-use`), where the design idea is worth more than the code.

Everything below cites the repository, the licence file actually read, and the date it was read.

---

## Candidate table

| # | Project | Licence (verified) | Lang | Replaces | Verdict |
|---|---|---|---|---|---|
| 1 | git binary + `update-ref --stdin` (baseline) | GPL-2.0 — *invoked, never linked* | C | — the reference the rest is judged against | **adopt (as a subprocess)** |
| 2 | `go-git/go-git` | Apache-2.0 | Go | narrow read-side helpers only | **study-only / narrow adopt** |
| 3 | Gitea `modules/git` | MIT | Go | gap-check on our existing `gitrunner.go` | **study-only (vendor a helper if it fits)** |
| 4 | `bluekeyes/go-gitdiff` | MIT | Go | unified-diff parse/apply for the Files lens | **adopt** |
| 5 | `sergi/go-diff` | MIT | Go | text diffing for conflict/preview rendering | **adopt** |
| 6 | `microsoft/azure-devops-go-api` | MIT | Go | half of `3-D8` / `OD-4`'s PR path | **adopt** |
| 7 | `google/go-github` | BSD-3-Clause | Go | the other half of `OD-4`'s PR path | **adopt** |
| 8 | `fluxcd/go-git-providers` | Apache-2.0 | Go | a ready-made multi-host abstraction | **reject (no ADO support)** |
| 9 | `google/git-appraise` | Apache-2.0 | Go | lane metadata stored in git refs | **study-only** |
| 10 | `jj` (Jujutsu) | Apache-2.0 | Rust | the convergence/undo model | **study-only** |
| 11 | `arxanas/git-branchless` | MIT OR Apache-2.0 | Rust | stacked-lane restack + `undo` | **study-only** |
| 12 | `dagger/container-use` | Apache-2.0 | Go | the whole agent-per-branch shape | **study-only** |
| 13 | `Wilfred/difftastic` | MIT | Rust | structural diff in the review lens | **study-only (later)** |
| 14 | `git2go` / `libgit2` | MIT binding over GPL-2.0-with-linking-exception core | Go/C | go-git alternative | **reject (licence + cgo)** |
| 15 | `aviator-co/av` | MIT (repo page) | Go | lane restack when the base moves (R13) | **study-only** |
| 16 | `ejoffe/spr` | MIT (repo page) | Go | commit→PR stack bookkeeping | **study-only** |
| 17 | `git-town/git-town` | MIT (repo page) | Go | undo of a multi-step git operation | **study-only** |
| 18 | `git-ecosystem/git-bundle-server` | MIT | Go | bundle lifecycle — **archived 2025-09-03** | **study-only** |
| 19 | `google/gitprotocolio` | Apache-2.0 | Go | git wire-protocol parsing — **archived 2023** | **study-only** |

Full arguments below. Copyleft and source-available projects that are relevant but excluded are
listed in their own section, by name only, with no source read.

---

## 1. The baseline you are already on: the real `git` binary

Phase 3's brief already assumes the git CLI (`merge-tree`, `commit-tree`, `update-ref <new> <old>`,
A5 in `scope-and-risk-phase-3-git-convergence.md` §0.3). **That assumption survives this research
intact**, and it is worth writing down why, because it is the reason most of the table below says
*study-only*.

- Git is **GPL-2.0**, but we **invoke** it as a subprocess and never link it. Invoking a GPL program
  creates no derivative work of it and imposes no obligation on PanaceaApp. This is the same posture
  every commercial IDE takes. The sandbox image ships git; the host ships git. No licence issue.
- Under **R4**, the host never runs git in a sandbox-writable directory, lane git runs **inside** the
  sandbox over RPC, and lane work crosses as a **bundle**. The binary is on both sides of that line
  already, so a library would have to beat git at *both* ends to be worth adopting.
- The operations phase 3 depends on — `git bundle create/verify`, `git update-ref --stdin` with a
  `transaction`/`prepare`/`commit` block and old-value compare-and-swap, `git merge-tree
  --write-tree` (conflict forecast with no worktree, git ≥ 2.38), `git merge --no-ff`, protocol v2
  fetch — are **exactly** the operations the Go ecosystem does not implement. See §2.

**Two concrete hardening recommendations that need no dependency at all**, both directly relevant to
R4 and worth more than any library on this list:

1. **Set `transfer.fsckObjects=true`, `fetch.fsckObjects=true` and `receive.fsckObjects=true`** on
   the host side before unbundling anything a sandbox produced. A bundle is a packfile from a
   process we do not trust; fsck on ingest is the cheap defence against a malformed or malicious
   pack. Also prefer `git index-pack --strict` in any manual ingest path.
2. **Isolate git config by environment, not by hope.** `GIT_CONFIG_GLOBAL=/dev/null`,
   `GIT_CONFIG_SYSTEM=/dev/null`, `GIT_CONFIG_NOSYSTEM=1`, plus `GIT_CONFIG_COUNT`/`GIT_CONFIG_KEY_n`
   /`GIT_CONFIG_VALUE_n` for the values we do want, gives a host resolver that reads **only**
   host-owned config — which is R4's requirement, stated as an env block rather than as code. Add
   `protocol.*.allow=never` for `ext`/`ftp`, `core.fsmonitor=false`, `core.hooksPath` pointed at an
   empty host-owned directory, and `--no-verify` where hooks must not run. There is **no library
   worth adopting for this**; it is about fifteen lines of `exec.Cmd.Env` and a test.

**Both are gaps in the tree today.** `control-plane/internal/project/gitrunner.go` already builds a
scrubbed environment — it sets `GIT_TERMINAL_PROMPT=0`, `GIT_OPTIONAL_LOCKS=0`, `GIT_PAGER=cat` and
`GIT_CEILING_DIRECTORIES` — but a read-only grep across `control-plane/internal/project` and
`sandboxd` on 2026-09-28 found **no `GIT_CONFIG_GLOBAL`/`GIT_CONFIG_SYSTEM`, no `fsckObjects`, no
`core.hooksPath`**. The user's own `~/.gitconfig` therefore still reaches a host git invocation. That
is a one-afternoon fix in a file that already exists, and it is worth more to R4 than every library
below.

Verdict: **adopt the binary, as you already planned.** Nothing below changes that.

---

## 2. `go-git/go-git` — the one that looks like the answer and is not

- Repository: https://github.com/go-git/go-git — 7.8k stars (read 2026-09-28).
- Licence: **Apache-2.0**, verified at https://raw.githubusercontent.com/go-git/go-git/master/LICENSE
  ("Apache License, Version 2.0", "Copyright 2018 Sourced Technologies, S.L."), read 2026-09-28.
- Maturity: **actively maintained**. `v5.19.2` published **2026-07-29** (pkg.go.dev, read
  2026-09-28). `v6.0.0-alpha.5` also 2026-07-29 — v6 has been in **alpha** for a long time and is
  explicitly not stable; its transport layer is a redesign.

**What it does:** a pure-Go git implementation, plumbing and porcelain, over a pluggable `Storer`
and `go-billy` filesystem, including in-memory repositories.

**What it cannot do**, verified in the project's own `COMPATIBILITY.md`
(https://raw.githubusercontent.com/go-git/go-git/main/COMPATIBILITY.md, read 2026-09-28):

| Phase 3 needs | go-git status |
|---|---|
| `git bundle` create/verify (R4's crossing) | **unsupported** |
| `update-ref` (A5's compare-and-swap land) | **unsupported** as a command |
| non-fast-forward `merge --no-ff` (3-D4) | **fast-forward only** |
| `rebase` | **unsupported** |
| `notes` (a candidate for lane metadata) | **unsupported** |
| `gc` (a clone per worktree needs housekeeping) | **unsupported** |
| pack-protocol **v2** (what ADO and GitHub negotiate) | **v1 only** |
| linked worktrees | partial, `x/plumbing/worktree`, "not all flags or subcommands" |
| partial clone | records the filter but **cannot fetch withheld objects on demand** |

That is not a list of gaps around the edges. **It is the phase 3 critical path, missing.** The
`update-ref` gap is the sharpest: go-git's storer exposes `CheckAndSetReference`, so a single-ref CAS
is expressible, but there is **no multi-ref atomic transaction** equivalent to `git update-ref --stdin`
with `transaction`/`prepare`/`commit`, and **no reftable backend**. A land that moves a lane ref and a
product branch together cannot be made atomic through it.

**Where go-git is still worth a dependency:** read-only, host-side, on a repository we own —
enumerating refs under `refs/panacea/lanes/…`, walking commits for the Files lens, reading blobs and
trees for a diff preview, and parsing `.git/config`
(`go-git/go-git/plumbing/format/config`) without shelling out per call. Those are hot, chatty
operations where a process per call is genuinely wasteful, and none of them mutate anything.

**Verdict: study-only for convergence; narrow adopt for host-side read paths only.** If it is
adopted, pin **v5**, never v6-alpha, and write the rule down: *go-git may read, git writes.* The
failure mode to avoid is the tempting one — a developer sees `Repository.Merge` exists, does not read
that it is fast-forward only, and ships a land that silently refuses or silently fast-forwards.

---

## 3. Gitea `modules/git` — the best permissively-licensed git-subprocess wrapper to learn from

- Repository: https://github.com/go-gitea/gitea, package `modules/git`.
- Licence: **MIT**, verified at https://raw.githubusercontent.com/go-gitea/gitea/main/LICENSE
  ("Copyright (c) 2016 The Gitea Authors"), read 2026-09-28. MIT covers the whole repo, so a file
  copied out carries only an attribution obligation.
- Maturity: one of the most-exercised Go git wrappers in production anywhere.

**What it does:** Gitea does not use go-git for the write path either. It runs the git binary, and
`modules/git` is the accumulated scar tissue of doing that safely at scale: a `Command` builder with
per-call timeouts and context cancellation, an argument allow-list that keeps user input out of flag
position, explicit `GIT_*` environment construction, stdout/stderr pipe handling that does not
deadlock on large output, and version detection that gates features on the installed git.

**What it would replace:** *not a greenfield.* `control-plane/internal/project/gitrunner.go` already
exists and already does the core of this — `exec.CommandContext` with a `GitTimeout`, a typed
`GitError` that splits stderr at the deadline, and a scrubbed environment. So Gitea is a **gap-check
against a battle-tested implementation**, not a replacement: read it for the cases our runner does
not yet cover (argument hygiene so a branch name can never land in flag position, large-output pipe
handling, git version gating, and the config isolation §1 found missing) as phase 3 adds
`merge-tree`, `commit-tree`, `update-ref` and `bundle` to the call surface.

**Integration cost:** do **not** import Gitea as a module — it drags a forge. Either (a) read it and
write our own ~300-line `gitcmd` package, or (b) vendor the two or three files that matter into
`control-plane/internal/git/vendor/gitea/` with the MIT notice intact and a `NOTICE` entry. Option
(b) is legitimate under MIT and is the cheaper of the two.

**Risks:** vendored code goes stale and nobody upgrades it; Gitea's wrapper assumes a server context
in places (it knows about its own settings package) so a straight copy needs pruning.

**Verdict: study-only, with vendoring open under MIT for a specific helper.** Because the runner
already exists, the honest saving is **1–2 days plus the bugs we do not ship**, not a week.

---

## 4. Diff and patch libraries — the clearest real saving in this phase

Phase 3 §2.7 has two lenses, and §2.6 wants each diff hunk linked back to the span that wrote it
(`3-D7`, permitted under `OD-8`). Both need a **structured representation of a diff**, not a blob of
text. That is a solved problem with permissive libraries.

### 4.1 `bluekeyes/go-gitdiff` — **adopt**

- Repository: https://github.com/bluekeyes/go-gitdiff — 156 stars, 30 forks (read 2026-09-28).
- Licence: **MIT**, verified at https://raw.githubusercontent.com/bluekeyes/go-gitdiff/master/LICENSE
  ("MIT License", "Copyright (c) 2019 Billy Keyes"), read 2026-09-28.
- What it does: parses the output of `git diff`, `git show` and `git format-patch` (and GNU unified
  diffs) into typed `File`/`TextFragment`/`Line` structures, **including git binary patches**, and
  can apply them in strict mode.
- **What it replaces:** the parser behind the Files lens and the hunk-to-provenance mapping. Its
  `TextFragment` already carries `OldPosition`/`NewPosition`/`LinesAdded`/`LinesDeleted`, which is
  precisely the coordinate system a "which agent turn wrote this hunk" back-link needs. Writing a
  correct unified-diff parser by hand — rename detection, mode changes, `\ No newline at end of
  file`, binary deltas — is a week nobody should spend.
- Integration cost: **low**. One small dependency, no transitive weight, no opinion about how you
  get the diff text (we get it from `git diff` under R4, inside the sandbox, and it crosses as text).
- Risks: small project, one principal maintainer, 156 stars. Mitigation is the same as its virtue —
  it is small enough to vendor if it is ever abandoned, under MIT. Its **apply** path is strict (no
  fuzzy matching, unlike `git apply`); we should not use it to apply anything. Parse only.
- **Verdict: adopt, for parsing only.** Saving: ~4–6 days plus the long tail of diff-format bugs.

### 4.2 `sergi/go-diff` — **adopt (small)**

- Repository: https://github.com/sergi/go-diff.
- Licence: **MIT**, verified at https://raw.githubusercontent.com/sergi/go-diff/master/LICENSE
  ("Copyright (c) 2012-2016 The go-diff Authors"), read 2026-09-28.
- Google's diff-match-patch ported to Go: character/line diffs with semantic cleanup. Used where we
  need a diff of text we hold in memory rather than a diff git produced — for example showing a
  conflict's two sides in the conflict view, or a before/after of a generated `CLAUDE.md`.
- Risk: the upstream diff-match-patch is itself in maintenance; the Go port is stable but slow-moving.
  It is 2k lines and MIT, so vendorable.
- **Verdict: adopt where needed; it is not architectural.**

### 4.3 Three-way merge in Go — **nothing adoptable exists; use `git merge-tree`**

I looked for a Go three-way merge/diff3 library good enough to compute a lane's conflict forecast
without git. There is no maintained, production-grade one: go-git's own merge is fast-forward only
(§2), and the small `diff3` ports on GitHub are hobby-scale with no releases. **`git merge-tree
--write-tree` (git ≥ 2.38) is the right tool and it is already in A5** — it computes a merge and
reports conflicts with no worktree and no index, which is exactly what a forecast needs and what
makes the forecast safe to run on the host against host-owned objects. **Do not replace it.**

`Wilfred/difftastic` (MIT, verified from the repository page 2026-09-28, 25.9k stars, Rust) is a
structural diff that understands syntax. It is interesting **later**, as a display mode in the review
lens for a reviewer drowning in a formatting-only diff. It is a CLI, not a library, and it would mean
shipping a Rust binary in the sandbox image. **Verdict: study-only, revisit after the lens ships.**

---

## 5. Host clients for `OD-4` (Azure DevOps **and** GitHub)

The decision is both hosts behind one interface. The interface is ours; the two clients underneath
should not be.

### 5.1 `microsoft/azure-devops-go-api` — **adopt, with a fallback plan**

- Repository: https://github.com/microsoft/azure-devops-go-api — 223 stars (read 2026-09-28).
- Licence: **MIT**, verified at
  https://raw.githubusercontent.com/microsoft/azure-devops-go-api/dev/LICENSE ("MIT License",
  Microsoft Corporation), read 2026-09-28.
- A generated, thin wrapper over the ADO REST APIs, import path `azuredevops/v7`.
- **What it replaces:** the ADO half of phase 3 §2.5 (push, create PR, read PR state).
- **Honest risk:** it is a *generated* client with a small following and **I could not verify its
  release cadence or an explicit maintenance statement** from the repository page. If it is stale,
  the fallback is cheap and should be designed for from day one: ADO is a plain REST API with PAT or
  bearer auth, and the two or three calls phase 3 needs (create PR, get PR, list refs) are ~150 lines
  of `net/http` behind our own host interface. **Keep the host interface thin enough that dropping
  this dependency is a one-file change.**

### 5.2 `google/go-github` — **adopt**

- Repository: https://github.com/google/go-github — 11.3k stars, current major **v92.0.0** (read
  2026-09-28).
- Licence: **BSD-3-Clause**, verified at https://raw.githubusercontent.com/google/go-github/master/LICENSE
  ("Copyright (c) 2013 The go-github AUTHORS"), read 2026-09-28. Permissive; attribution only.
- Mature, fast-moving (a major version per API sweep), GitHub App auth via companion packages.
- **What it replaces:** the GitHub half of §2.5. There is no reason to hand-roll this.
- Risk: the major-version churn is real — `v92` implies a rename every few months. Contain it behind
  our host interface so a bump is one package, not the codebase.

### 5.3 `fluxcd/go-git-providers` — **reject**

- Repository: https://github.com/fluxcd/go-git-providers — Apache-2.0, **72 stars** (read 2026-09-28).
- It is the "one interface over many hosts" library we would want, and it supports **GitHub, GitLab
  and Bitbucket Server — not Azure DevOps** (verified from the repository page, 2026-09-28).
- Since `OD-4` names ADO as a first-class host, adopting this would mean writing the ADO provider
  ourselves *and* conforming to somebody else's abstraction for the host we care most about. That is
  strictly worse than our own two-implementation interface.
- **Verdict: reject.** Worth reading its `gitprovider` interface for naming, nothing more.

---

## 6. Bundles — what exists, and what does not

R4 makes the bundle the crossing point, so this got its own search.

- **There is no Go library that creates or verifies git bundles.** go-git lists `bundle` as
  unsupported (§2). Searching the Go package registry surfaced nothing purpose-built (searched
  2026-09-28). The `git bundle create` / `git bundle verify` / `git fetch <file.bundle>` triad is the
  only implementation, and it is the one we should use.
- **`git-ecosystem/git-bundle-server`** (https://github.com/git-ecosystem/git-bundle-server) — Go,
  **MIT**, 68 stars — self-hosts bundles for git's bundle-URI feature. It is **archived, read-only
  since 2025-09-03** (read 2026-09-28), and its scope is serving bundles for clone acceleration, not
  verifying an untrusted bundle. **Verdict: study-only**, specifically its bundle lifecycle and
  naming conventions; do not depend on an archived project.
- **`google/gitprotocolio`** (https://github.com/google/gitprotocolio) — Go, **Apache-2.0**, 399
  stars, **archived 2023-01-10** (read 2026-09-28) — parses the git wire protocol. Relevant only if
  we ever decide the lane RPC should carry the smart protocol rather than a bundle file. It is
  archived and protocol-v2 coverage is unverified. **Verdict: study-only.**
- **The safety work is ours and is not a library.** A bundle from a sandbox is untrusted input:
  verify with `git bundle verify` against host-owned prerequisites, ingest with fsck on
  (`fetch.fsckObjects=true`), cap the file size, refuse a bundle whose ref names fall outside
  `refs/panacea/lanes/<lane-id>/…`, and never let the bundle's own ref names choose the destination.
  Those five rules are the real deliverable and no open-source project hands them to us.

---

## 7. Lane convergence — prior art worth reading, not importing

None of these is a Go library we can link. All four are **design capital**: someone has already made
the mistakes we are about to make, in public, under a licence that lets us read.

### 7.1 `jj` (Jujutsu) — **study-only, and the most valuable read on this list**

- Repository: https://github.com/jj-vcs/jj — 31.8k stars, Rust.
- Licence: **Apache-2.0**, verified at https://raw.githubusercontent.com/jj-vcs/jj/main/LICENSE
  (the Apache 2.0 template; no per-file copyright line in the LICENSE itself), read 2026-09-28.
  *Latest release date unverified* — the repository page reported v0.24 (November 2024), which looks
  stale against the project's known cadence; I did not get a second source, so treat the version as
  unknown and the project as unambiguously active (31.8k stars, Google-originated).
- **Why it matters to phase 3**: two model choices we are about to make by hand, jj has already made
  and shipped.
  1. **The operation log.** Every repository mutation is recorded as an operation, and `undo` moves
     the repo to a previous operation. Phase 3 wants "landed history with undo". jj's answer — *undo
     is a first-class log of operations over refs, not an inverse of each command* — is the design we
     should copy in shape. Writing our undo as "the inverse of a land" is the trap; recording the
     before-state of every ref we touched, keyed by a land id, is the way out.
  2. **Conflicts as first-class objects.** jj stores a conflicted state in a commit rather than as
     markers in a working file, so a conflict can be carried, rebased and resolved later. Phase 3's
     `3-D3` asks who resolves a conflict and whether anything is automatic. If a lane's conflict is a
     *recorded object* rather than a broken working tree, the cockpit can show it, route it and defer
     it — which is a better product than "your worktree is now in a conflicted state, good luck".
- **Integration**: none. Rust, its own storage backend, its own CLI. Adopting jj would replace git as
  the model of record, which contradicts `OD-2` and `OD-4`. **Do not adopt. Read the docs on the
  operation log and conflict model before the phase 3 convergence tasks are written.**

### 7.2 `arxanas/git-branchless` — **study-only**

- Repository: https://github.com/arxanas/git-branchless — 4.1k stars, Rust.
- Licence: **MIT OR Apache-2.0** (dual). MIT verified at
  https://raw.githubusercontent.com/arxanas/git-branchless/master/LICENSE-MIT ("Copyright (c) 2021
  Individual contributors"), read 2026-09-28. Permissive on either arm.
- `git undo` over a real git repository (it uses git's reflog plus its own event log), `git restack`
  to repair a stack after its base moves, `smartlog`.
- **What it maps to**: our "re-forecast after a land loses the compare-and-swap race" (R13) is
  exactly `restack`. Its event-log approach is the *git-compatible* version of jj's operation log,
  and therefore closer to what we can actually implement on top of `refs/panacea/lanes/…`.
- Self-described as **alpha**, with breaking changes expected. Rust, CLI-shaped.
- **Verdict: study-only.** Read how it materialises an event log alongside git without owning the
  repository format. That is the trick we need.

### 7.3 `google/git-appraise` — **study-only, high relevance to the provenance spine**

- Repository: https://github.com/google/git-appraise — 5.3k stars, **Go**, Apache-2.0 (as GitHub
  reports; read 2026-09-28 — *the LICENSE file itself was not separately fetched*).
- Stores review requests, comments, CI results and analysis output **inside the repository** as git
  notes under `refs/notes/devtools/reviews`, `…/ci`, `…/analyses`, `…/discuss`, one JSON object per
  line, merged with the `cat_sort_uniq` strategy so two clones can both write and converge.
- **Why this is the most directly relevant idea in the list**: it is a worked, production example of
  *carrying structured metadata alongside commits, in refs, across clones, without a server* — which
  is exactly the shape of `refs/panacea/lanes/…` plus the provenance thread. The `cat_sort_uniq`
  merge on append-only JSON lines is the reason two agents' notes never conflict. If our lane
  metadata is append-only JSON lines in a note ref, lane metadata convergence becomes a non-problem.
- **Caveat that decides the verdict**: it is Go and permissive, so *importing* it is legally fine,
  but it is old, slow-moving, and it models **human code review**, not agent lanes. `notes` is also
  the one thing go-git cannot do (§2), so a notes-based design commits us harder to the git binary.
- **Verdict: study-only.** Copy the *pattern* (append-only JSON-lines notes + `cat_sort_uniq`), not
  the package. Then decide deliberately whether lane metadata lives in notes (portable, survives a
  clone, visible to the customer's own git) or in the Hub's Postgres (queryable, `OD-3`). **My
  reading of `OD-3` plus the provenance spine says the Hub is the record and notes are, at most, a
  mirror** — but the question deserves an explicit ADR rather than drift.

### 7.4 `dagger/container-use` — **study-only, closest competitor shape**

- Repository: https://github.com/dagger/container-use — ~4.0k stars, Go.
- Licence: **Apache-2.0**, verified at
  https://raw.githubusercontent.com/dagger/container-use/main/LICENSE ("Apache License, Version
  2.0", "Copyright 2022 Dagger, Inc."), read 2026-09-28.
- An MCP server that gives each coding agent "a fresh container in its own git branch", with command
  history and logs, terminal drop-in, and a plain `git checkout` to review an agent's work.
- **This is phase 3's shape, built by someone else, permissively licensed.** That makes it the single
  best source of "what did they hit" for: how agent work returns to the user's repo, what the branch
  naming looks like, how the agent's environment is described, and how the MCP surface is framed.
- **Why not adopt**: it is Dagger-shaped end to end. Adopting it means adopting Dagger's engine as
  the isolation layer, which contradicts the locked Docker-local/Kata-cloud decision, and its branch
  model is not `OD-2`'s clone-per-worktree with `refs/panacea/lanes/…`. It also carries no
  provenance thread — the spine `CLAUDE.md` says any feature must not break.
- **Verdict: study-only.** Worth one focused read before the phase 3 plan is written; vendoring a
  file or two under Apache-2.0 with attribution is legally open if something specific fits.

### 7.5 `git-town/git-town` — **study-only (small, but cheap)**

- Repository: https://github.com/git-town/git-town — 3.4k stars, **Go**, **MIT** (as GitHub reports;
  read 2026-09-28 — LICENSE file not separately fetched).
- Branch-stack automation with a `git town undo` that undoes the most recent Git Town command.
- Relevance: it is **Go**, **MIT**, and it solves "undo a multi-step git operation safely" in the
  same language we are writing. If any project on this list is worth a targeted code read for
  implementation technique (recording a runnable "undo program" as it goes), it is this one.
- **Verdict: study-only**, with vendoring open under MIT if a specific helper fits.

### 7.6 Gerrit — **study-only, concept only**

Gerrit's `refs/for/<branch>` push target and `refs/changes/NN/…` storage is the canonical
"server-mediated lane" design and is worth knowing as the thing we are *not* doing. It is Java,
enormous, and needs its own server. **Its licence is Apache-2.0 by reputation but I could not verify
it from source: `gerrit.googlesource.com` returned HTTP 503 and the GitHub mirror's
`master/LICENSE` returned 404** (both 2026-09-28). Since the verdict is concept-only, no licence
decision rests on it.

### 7.7 Worktree managers for agents — **nothing to adopt**

The 2026 crop (Claude Squad, Conductor, Vibe Kanban, Parallel Code, treehouse-worktree, uzi,
crystal) all do "a worktree and a terminal per agent". **Phase 2 has already built this part of our
product**, so the saving would be zero even where the licence allows, and several are copyleft
(§8). The one structural idea worth noting is from `treehouse-worktree`: *lock a worktree to prevent
two agents entering it*, which is `3-D2` ("two agents in one worktree: allowed, warned, or
refused?") answered as "refused, by a lock". **Verdict: reject as dependencies; one idea recorded.**

### 7.8 `libgit2` / `git2go` — **reject**

- `git2go` (https://github.com/libgit2/git2go) is **MIT**, but it is a cgo binding over **libgit2**,
  which is **GPL-2.0 with a linking exception** (verified at
  https://raw.githubusercontent.com/libgit2/libgit2/main/COPYING, read 2026-09-28: *"the authors give
  you unlimited permission to link the compiled version of this library into combinations with other
  programs, and to distribute those combinations without any restriction"*).
- **The exception means linking is legally permitted** — this is not an AGPL-style trap, and I am
  saying so plainly rather than reflex-rejecting. The rejection is on other grounds:
  1. it forces **cgo** into a Go control plane that is currently pure Go and cross-compiles to
     Windows for the desktop side — a real, recurring build cost on a KPMG laptop behind a proxy;
  2. git2go's module versions track libgit2 releases (`v34` ≈ libgit2 1.2), so upgrades are a
     coordinated C-library bump, not a `go get`;
  3. libgit2 **also** has no `git bundle` and no `update-ref --stdin` transaction equivalent, so it
     does not solve §2's actual problem;
  4. it puts a GPL-2.0 artefact into a product KPMG ships to regulated customers, which means a
     legal conversation on every release even though the exception makes the answer "fine". After
     `OD-11`, this programme should not spend another licence conversation it does not have to.
- **Verdict: reject.**

### 7.9 `aviator-co/av` and `ejoffe/spr` — **study-only, but the closest usable code**

Two **Go**, **MIT** stacked-PR tools (both read 2026-09-28):

| Project | Repo | Licence | Stars | Shape |
|---|---|---|---|---|
| `av` | https://github.com/aviator-co/av | MIT (as GitHub reports) | 509 | tracks a stack of branches, each branch a PR, auto-restacks when a base moves |
| `git spr` | https://github.com/ejoffe/spr | MIT (as GitHub reports) | 1.3k | one commit → one PR, keeps the stack ordered and synced |

**Why they matter**: phase 3 §2.5 has to turn a landed lane into a PR on **two** hosts, and has to
cope with the base branch moving underneath a lane that is already open (R13). Both projects solve
the second problem in Go, in public, under MIT. `av`'s "remember the base commit the branch started
from, restack when it moves" is exactly the lane re-forecast.

**Why not adopt**: both are GitHub-only (no ADO, breaking `OD-4`), both are CLI-shaped with their own
state files and their own UX, and both model a *human's* stack of PRs rather than an agent's lane.
Importing either would mean fighting its model. **Verdict: study-only, with vendoring open under MIT
if one helper (the restack base-commit bookkeeping) fits cleanly.** *Caveat: for both, the licence is
as GitHub's repository page reports it; I did not fetch either LICENSE file directly. Verify before
any code is copied.*

---

## 8. Copyleft and source-available — the record that we looked, and did not read

Listed by name and licence only. **No source from any of these was opened, searched or copied**, per
`CLAUDE.md` and `OD-11`. Several are the *most interesting* projects in the space, which is precisely
why the record matters.

| Project | Licence (as reported) | What it is | Why excluded |
|---|---|---|---|
| **GitButler** | **FSL-1.1-MIT** (Functional Source License 1.1, MIT Future License) — verified at https://github.com/gitbutlerapp/gitbutler/blob/master/LICENSE.md, read 2026-09-28 | "Virtual branches" — multiple concurrent lines of work in one working directory, *conceptually the closest thing in existence to our lanes* | Source-available with a competing-use restriction, converting to MIT only on the second anniversary of each release. Falls squarely inside the forbidden list (BUSL-class). **Not read.** |
| **Sapling SCM** (Meta) | **GPL-2.0** (repo page, read 2026-09-28) | Git-compatible, scalable VCS with stacked-change workflows | GPL. **Not read.** |
| **`abhinav/git-spice`** | **GPL-3.0** (repo page, read 2026-09-28) | Go, stacked-branch manager, multi-forge | GPL-3.0, and it is **Go** — the most dangerous kind of exclusion, because a developer could copy a function without thinking. **Do not open it.** |
| **`smtg-ai/claude-squad`** | **AGPL-3.0** (repo page, read 2026-09-28) | Go + tmux + git worktrees; several coding agents in parallel — our phase 2/3 shape exactly | AGPL. This is the `Paperdesign` mistake waiting to happen again, in our own domain. **Do not open it.** |
| **`mergiraf`** | **GPL-3.0** (https://codeberg.org/mergiraf/mergiraf, read 2026-09-28) | Syntax-aware git merge driver — would materially reduce lane conflicts | GPL. **Not read.** If structured merge becomes valuable, the permissive route is our own tree-sitter-based driver. |
| **Gerrit Code Review** | Apache-2.0 *by reputation; unverified, see §9* | Server-mediated change refs (`refs/for/*`, `refs/changes/*`) | Not excluded on licence — excluded on fit (Java, own server). Concept only. |
| **Graphite CLI** | **unknown / unavailable** | Stacked-diff product | `https://github.com/withgraphite/graphite-cli` returned **HTTP 404** on 2026-09-28. Treat as **not open source / unavailable**; do not plan around it. |
| **git itself** | GPL-2.0 | the binary we invoke | **Not excluded**: invoked as a subprocess, never linked. See §1. |

---

## 9. What I could not verify, and what the network blocked

**Blocked or failed fetches (2026-09-28):**

| URL | Result |
|---|---|
| `https://gerrit.googlesource.com/gerrit/+/refs/heads/master/LICENSE` | **HTTP 503** — Gerrit's licence not verified from source |
| `https://raw.githubusercontent.com/GerritCodeReview/gerrit/master/LICENSE` | **HTTP 404** — mirror has no file at that path |
| `https://github.com/withgraphite/graphite-cli` | **HTTP 404** — repository absent or removed |

None of these was a TLS/proxy interception failure. GitHub, raw.githubusercontent.com, pkg.go.dev and
codeberg.org all resolved normally through the corporate proxy. **No Netskope block was observed in
this pass.**

**Verified from the actual LICENSE file** (highest confidence): go-git, go-gitdiff, go-diff,
go-github, azure-devops-go-api, Gitea, container-use, git-branchless (MIT arm), jj, libgit2,
GitButler.

**Licence taken from the repository page, not the LICENSE file** (verify before any code is copied):
git-appraise, git-town, `av`, `spr`, git2go, Sapling, git-spice, claude-squad, go-git-providers,
difftastic, git-bundle-server, gitprotocolio, mergiraf.

**Facts I could not pin down:**
- **Release dates are unreliable from repository pages.** go-git's I confirmed against pkg.go.dev
  (v5.19.2, 2026-07-29; v6.0.0-alpha.5 the same day). For jj the page reported *v0.24, November
  2024*, which is almost certainly stale — **treat jj's current version as unknown**.
  `container-use`, `go-gitdiff`, `av`, `spr`, `git-town` and `azure-devops-go-api` release dates are
  **unverified**.
- **`azure-devops-go-api`'s maintenance status is unverified** and it is the thinnest link in the
  `OD-4` chain. Design the host interface so it can be dropped for direct REST in a day.
- **Transitive dependency licences were not audited** for any candidate. Before any `go get`, run a
  licence gate (`go-licenses`) over the resulting module graph — the gate `OD-11` already requires
  for phase 5 should cover the whole `go.work`. **This is a real gap in this research**: a permissive
  top-level licence says nothing about the tree beneath it.
- The in-tree observations in §1 and §3 were made on the working tree of `feat/phase-2-cockpit` on
  2026-09-28 while **other workflows were actively editing those files** (`gitrunner.go` and
  `project.go` were both modified-uncommitted at the time). Re-check them against whatever lands.
- I did **not** read `COMPATIBILITY.md` for go-git v6 separately; the compatibility table cited in §2
  is from `main`, which tracks v6 development. If v6 later ships bundle or `update-ref` support, §2's
  verdict should be revisited — **that is the one finding here with a plausible expiry date.**

---

## 10. What I would actually do, in order

1. **Change nothing about the architecture.** Phase 3's A5 plumbing plan (`merge-tree`,
   `commit-tree`, `update-ref` with compare-and-swap) is correct and is the *only* implementation
   route the ecosystem supports. This research confirms it rather than replacing it.
2. **Adopt four dependencies, all permissive, all small**: `bluekeyes/go-gitdiff` (MIT) for diff
   parsing, `sergi/go-diff` (MIT) for in-memory text diffs, `google/go-github` (BSD-3) and
   `microsoft/azure-devops-go-api` (MIT) behind our own one-host interface. Combined realistic
   saving: **8–12 focused days**, concentrated where hand-written code is most bug-prone.
3. **Close the two R4 gaps in `control-plane/internal/project/gitrunner.go` first** — `GIT_CONFIG_*`
   isolation and fsck-on-ingest (§1) — then read Gitea `modules/git` (MIT) for what else our runner
   is missing before phase 3 widens its call surface. **1–2 days.** This is the highest-value item
   on the list and it is not an adoption at all.
4. **Spend half a day reading three things before the phase 3 convergence tasks are written**: jj's
   operation log and conflict model, git-appraise's append-only JSON-lines notes with `cat_sort_uniq`,
   and `av`'s base-commit restack bookkeeping. These change the *design*, which is worth more than
   the code.
5. **Write the licence gate into CI now** (`go-licenses` over `go.work`; deny AGPL/GPL/LGPL/SSPL/
   BUSL/FSL/Elastic/Commons-Clause), so the next `OD-11` never happens. Half a day.

**The honest summary for the owner:** phase 3 is a phase where open source buys **days, not weeks**.
The 24 planned tasks are mostly product decisions (`3-D1`–`3-D10`), security structure (R4) and
provenance wiring — none of which anyone else has written for us, because nobody else has our
constraints. The one place open source *would* have saved weeks — a pure-Go git that can bundle and
transact refs — **does not exist**.

