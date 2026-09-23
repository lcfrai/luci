# Survey — phase 3: git, worktrees, coordination and convergence

**Date:** 2026-09-23. **Method:** read the code, not the plans. Every claim below cites a file and a
line range I opened. Where I am inferring rather than reading, the sentence says so.

**Scope note.** `desktop/`, `sandboxd/`, `control-plane/`, `docs/superpowers/contracts/` and
`docs/superpowers/plans/` were read-only for this pass, and other agents were editing some of them
while I read. Line numbers are a snapshot of the tree at roughly 2026-09-23 17:00.
**`control-plane/internal/api/sessions.go` moved by about a hundred lines between two of my reads**
(the resume fix is landing in it), so this survey cites that file by function name, not line. I
ran no build, no test and no git command that writes; the only commands were reads (`git grep`,
`git worktree list`, `git --version`, `git diff --stat`).

**Why this survey exists.** `docs/PROGRAM.md` puts phase 3 next in the adopted order, and the
phases 4/5/7 planning pass placed nine obligations on it
(`completeness-critique-2.md` §6.3) while it had no plan, no survey and no brief. This document is
where those nine land. §9 takes them one at a time.

---

## 0. The headline, in six sentences

Phase 2 built **exactly three read-only git calls and a guard that forbids a fourth**
(`project.go:1-6`, `project_test.go:427-464`), so phase 3 inherits enumeration and nothing that
writes. **Phase 2's worktree model is a host directory bind-mounted into a Docker sandbox**
(`handleSessionCreate` in `api/sessions.go`), **the Kata backend the architecture locks refuses
every mount** (`katabe/kata.go:84-85`), and — inferred, not run — **git inside a sandbox fails on
any linked worktree and has no commit identity on any worktree** (§4.2, §4.3); so the first thing
phase 3 must settle is *where the repository lives and how a sandbox reaches it*, in a form that
survives phase 8. The concurrency problem at the heart of the phase has two halves: **between
worktrees, agents cannot collide while they write and collide only when their work converges** —
so a file claim there is a *forecast of a merge conflict*, which `git merge-tree` computes exactly
— while **inside one worktree, which `FUNCTIONAL-SPEC.md:18` says several agents routinely share,
there is no isolation at all** and no claim can be enforced without wrapping the CLI. Phase 2's rule
that agent CLIs are never wrapped or parsed leaves one honest place to stamp a session id on an
agent's commit — **a git hook enabled by environment the control plane sets on the sandbox** — and
it is advisory, because the agent can bypass it. Files and Merge are two `LensPlaceholder
phase={3}` arms today, and **no document says what Files shows**; §8 proposes an answer drawn only
from facts phase 3 would own.

---

## 1. What phase 2 already provides

### 1.1 Enumeration — three git reads, guarded

`control-plane/internal/project/project.go` is the whole of phase 2's git surface.

| Call | Where | What it yields |
|---|---|---|
| `for-each-ref --format=%(refname:short) refs/heads` | `project.go:241` | local branch names only — **no remote refs, no upstream, no tip SHA** |
| `rev-parse --abbrev-ref HEAD` | `project.go:248` | which branch the primary worktree has checked out (`IsHead`) |
| `worktree list --porcelain` | `project.go:282`, parsed `:294-331` | worktree path, branch, detached/bare — **the `HEAD <sha>` line is read past and discarded** |

The package comment is explicit about the boundary: *"It is READ-ONLY by design: docs/PROGRAM.md
phase 3 owns worktree lifecycle, and a Phase 2 that could create or delete one would be building
Phase 3 badly"* (`project.go:1-6`). `project_test.go:427-464` enforces it mechanically: every git
invocation the scanner makes is checked against a whitelist and anything else fails with *"git %s is
not on the read-only whitelist; Phase 3 owns anything that writes"* (`:464`).

**Consequence for phase 3.** Phase 3 does not extend `project.Scanner`; the whitelist test would
fail on its first write, correctly. It adds a sibling package with its own runner discipline and
leaves the scanner's guard intact. (Recommendation, not a finding.)

### 1.2 The one git process launcher

`control-plane/internal/project/gitrunner.go` — `ExecRunner` is *"the only place in the product that
starts a git process"* (`:19-20`). What it already gets right, and phase 3 must copy rather than
re-derive:

- **Stdout only on success** (`:32-45`). git writes non-fatal warnings to stderr and exits 0; a
  parser fed combined output grows a branch named *"warning: ignoring broken ref…"*.
- **Headless environment** (`:56-63`): `GIT_TERMINAL_PROMPT=0`, `GIT_OPTIONAL_LOCKS=0`,
  `GIT_PAGER=cat`, `GCM_INTERACTIVE=never`. A credential prompt on Windows otherwise pops a window.
- **A 10-second timeout per call** (`GitTimeout`, `:13-17`), because the rail polls it.

**Two of those are wrong for phase 3's writes, and that is a design point, not a defect.**
`GIT_OPTIONAL_LOCKS=0` exists so a *read* does not take `index.lock` behind a user's back; a merge
or a `worktree add` must take its locks. And ten seconds is right for `for-each-ref` behind a 2 s
poll and wrong for a merge of a large branch or a `push` over a corporate proxy. Phase 3's runner
needs its own timeout class and must never run behind the poll.

### 1.3 Identity — ids derived from paths and names, with no storage

| Id | Derivation | Where | Survives |
|---|---|---|---|
| `p-…` project | first 8 hex of sha256(lowercased absolute path) | `project.go:92-102` | moving the directory: **no** |
| `b-…` branch | project hash + slug(name) + first 8 hex of sha256(raw name) | `project.go:118-121` | **rename: no** — a renamed branch is a new id |
| `w-…` worktree | first 8 hex of sha256(lowercased absolute path) | `project.go:105` | moving the worktree: **no** |

**This matters more to phase 3 than to any other phase.** Phase 3 is the phase that renames,
deletes and moves these things. A branch rename mints a new `b-…` id, so every store keyed by branch
id — the phase 4 brief's `branchId`, phase 7's per-branch deploy view, the desktop's persisted
`pinned` and `branchDefaults` — sees a deletion and a creation. The provenance contract already
protects the brief itself (`provenance-contract-draft.md:91-92`: *"`briefId`… never changes… including
across branch rename"*), but nothing re-points the branch-keyed stores. §6.1 takes this up.

### 1.4 How a session reaches its worktree today

- The control plane resolves `worktreeId → path` through `Scanner.Path`, which verifies the
  directory exists and creates nothing (`project.go:333-362`).
- Session create **bind-mounts that host path into the sandbox** at `sessionWorkDir`
  (`api/sessions.go`: `sessionWorkDir`, and the `Mounts` line of `handleSessionCreate`'s `backend.Spec`); restart rebuilds the spec and re-resolves the mount
  (`handleSessionRestart`, same file).
- `backend.Mount` is **Docker only** (`backend/backend.go:124-146`). The Kata backend **refuses any
  mount**: *"%d mount(s) requested but a host path on an AKS node is meaningless"*
  (`backend/katabe/kata.go:84-85`), pinned by `kata_test.go:207`.

**So under Kata there is, today, no worktree in the sandbox at all.** Phase 2 built the
Docker-on-laptop path end to end and left the microVM path's filesystem unsolved, which is honest —
it is a phase 3 and phase 8 question — but it means "worktree lifecycle against real git" has no
agreed location. §4 lays out the options; it is decision `3-D1` in the brief.

### 1.5 Worktree flags — the one thing phase 2 writes about a worktree

`registry.FileStore.SetWorktreeReady` (`registry/filestore.go:422`) and
`WorktreeFlagsSnapshot` (`:408`) persist a per-worktree `ready` flag in the same document as sessions
(`filestore_test.go:355`). `PATCH /worktrees/{id}` sets it (`api/catalog.go:384`) and rejects a body
that sets anything else (`catalog_test.go:374`). **It is a flag in our registry, not a git fact.**
Phase 3 will be tempted to hang merge state off the same record; that is a choice, not an inheritance
(§6.3).

### 1.6 Seams phase 2 has already named for phase 3 — planned, not landed

`docs/superpowers/plans/phase-2-parts/task-26.md` is an **untracked, unexecuted** phase 2 plan
(`git status` at the start of this pass lists it as `??`). It names the seams later phases plug
into. The rows addressed to phase 3, quoted from its seams table (`task-26.md:940-954`):

| Seam | Where | Phase 3 does | Phase 3 must not |
|---|---|---|---|
| SX.1 | `components/lens/{Files,Merge}Lens.tsx`; `AppShell.tsx` closed | replace the body of its own lens file | edit `AppShell.tsx`; add a `Lens` member |
| SX.4 | `api/api.go:172`, the route recorder | add routes to the table and both audit maps | register on the raw mux; add a side-effecting GET |
| SX.5 | `statepath.Dir(sub)` | take one subdirectory | re-derive the state ladder |
| SX.6 | `palette/commands.ts` | append commands | put a destructive verb one keystroke from a filter |
| SX.9 | `tokens.css:75,77` — `--drift` | use the token the day the status can be derived | invent a second purple |
| S3.1 | `internal/project`, read-only | put every git write in a new package | widen the whitelist |
| S3.2 | `worktree.Provider.Path` | make every worktree it creates resolve through it | resolve paths a second way |
| S3.3 | `PATCH …/worktrees/{id}`, `Worktree.ready?` | gate merge on `ready` | derive readiness any other way (known defect 9) |
| S3.4 | `lib/immutable.ts` `moveWithinVisible`, `idAtVisiblePosition` | reorder the merge queue through them | pass an index across a filter boundary (known defect 10) |
| S3.5 | `FilesLens.tsx`, `MergeLens.tsx` | fill both | treat Files' ownership as settled |

**Today, in the tree, none of the lens files exists.** `desktop/src/components/lens/` holds only
`LensPlaceholder.tsx`, and `AppShell.tsx:44-58` still routes `files` and `merge` to
`LensPlaceholder phase={3}`. Files carries the line *"The worktree's files, with what each session
changed"* (`AppShell.tsx:49`). SX.1 is what makes phase 3 not collide with phases 4, 5 and 7 in
`AppShell.tsx` (`survey-phase-2-substrate.md` §4.4); **phase 3 should not start its desktop half
until task 26 has landed**, or it re-opens that collision.

Two more items phase 2 routed to "phase 3", found in
`.superpowers/sdd/2026-09-18-phase-2-backend-and-cockpit/group-d-open-carries.md`:

- **Carry 8 — Authority.** `TopBar.tsx:29` renders a disabled *Authority* button titled *"Authority
  needs the agent-runtime signal (docs/AGENTIC-UX.md §7). It arrives in Phase 3."* `PROGRAM.md`'s
  phase 3 does not mention authority. The carry says *"one product decision settles all three"* —
  Files, Merge and Authority. This survey treats Authority as **not phase 3** (§10) and asks the
  owner to confirm (`3-D10` in the brief).
- **Carry 11 — "`observe` drops `dirty` before the write. Minor 9 from Group C → Phase 3."** It is
  about the output tracker's throttle in `api/attach.go:277-296`, which has nothing to do with git.
  **Probably a pre-renumber phase number**: `PROGRAM.md:77-83` records the renumbering and
  `PROGRAM.md:129` records items it missed. Received and returned to the orchestrator, not absorbed.

---

## 2. What the other documents already assume phase 3 does

Nine obligations, counted by `completeness-critique-2.md` §6.3 (`:491-503`), plus three more this
survey found by reading the same documents.

| # | Obligation | Placed by | Exact words |
|---|---|---|---|
| 1 | Criterion evaluator over `(brief key, commit)` | contract §2.3 | *"it takes `(brief key, commit)` and returns states, it never takes a branch id"* (`provenance-contract-draft.md:204-206`) |
| 2 | Write the merge link; choose ledger or trailer | contract §2.2 | *"**phase 3 decides**: ledger or a git trailer"* (`:184`) |
| 3 | Decide, with the owner, whether `CLAUDE.md` is committed | contract §3.4, Q4 | *"This is phase 3's to build and yours to decide"* (`:616-619`) |
| 4 | Branch deletion archives a head, never deletes a revision | contract P13, §8.3 | *"Branch deletion may archive the head; revisions stay"* (`:423`) |
| 5 | Stamp a session id on every agent commit | `scope-and-risk-phase-7-deploy.md` D3 option b | *"a trailer, e.g. `Panacea-Session: <id>`"* (`:60`) |
| 6 | Ahead/behind, so Ship can say what will not ship | `scope-and-risk-phase-7-deploy.md` §4.2 | *"needs phase 3's git API to report ahead/behind for the branch"* (`:237-238`) |
| 7 | Decide whether the design handoff bundle is committed | `scope-and-risk-phase-5-design.md` §2.3 | *"follows the same answer as contract open question 4"* (`:107`) |
| 8 | Mount `DefinitionOfDone` above the merge list | `scope-and-risk-phase-4-discover.md` S10 | *"Phase 3 mounts the first and phase 7 mounts the second"* (`:88`) |
| 9 | Own the Files and Merge lenses | `AppShell.tsx:44-58`; task 26 S3.5 | *"Fill both"* |
| 10 | **Push** — get a cockpit branch onto a remote Panacea can clone | `dependency-graph-and-order.md:101`, `:275` | *"Getting worktree → pushed remote is Phase 3's PR/push work"*; *"Phase 7b cannot precede Phase 3"* |
| 11 | **Worktree creation** — Discover's only need is `Path`, and phase 3 replaces phase 2 as the creator | `scope-and-risk-phase-4-discover.md:95` | *"Phase 2 today (`project.Scanner`), phase 3 later"* |
| 12 | **The real worktree layout** | `scope-and-risk-phase-4-discover.md:261-262` | *"Phase 3 has not fixed the real layout"* |

Rows 10–12 are not in the critique's count. Row 10 is the one with a schedule consequence: phase
7b (Ship) waits on phase 3's push, per `dependency-graph-and-order.md:275`.

**One thing the documents assume that phase 3 should refuse.** The phase 4 plan once assigned the
*"CLAUDE.md-before-PTY"* guarantee to phase 3; `dependency-graph-and-order.md:99` already calls that
*"wrong against the code, because Phase 3 does not own session start"*. Nothing in this survey
takes it back.

---

## 3. The concurrency problem — several agents in one repository

This is the heart of the phase, and the documents so far describe it in the language of the
prototype (*"2 agents converging on services/api/routes.go"*, `AGENTIC-UX.md:92`) rather than of
git. Stated in git, there are **three different concurrency situations**, and file claims mean
something different in each.

### 3.1 The object model promises something git does not allow

`docs/FUNCTIONAL-SPEC.md:13` — *"**Worktree** — many per branch; an isolated checkout."* The
prototype's rail draws exactly that: branch `evidence-upload` holding worktrees `upload`, `test`,
`docs` (`AGENTIC-UX.md:98-102`), and its Merge lens is *"Merge into ${branch}"* — worktrees land
**into their branch** (`prototype/app.html:1147`, `mergeOne` at `:718-724`).

**git refuses to check out one branch in two worktrees** (`git worktree add` refuses a branch
another worktree already has checked out, unless forced — git's documented behaviour, not run here). Phase 2's scanner reports each worktree under the branch it has
checked out (`project.go:319-322`), so against real git **every branch has at most one worktree**
and the prototype's hierarchy cannot occur. Forcing it is worse than refusing it: two worktrees
whose `HEAD` is the same branch see each other's commits as reversed staged changes.

So "many worktrees per branch" has to mean **each worktree has its own git branch — a lane — whose
parent is the product-level branch**, and Merge lands lanes into the parent. git has a native way
to say "this branch's parent is that local branch": set the lane's upstream to it
(`branch.<lane>.remote = .`, `branch.<lane>.merge = refs/heads/<parent>`). Then
`git rev-list --left-right --count <lane>...<lane>@{upstream}` is ahead/behind **against the
parent** with no metadata of our own. That is a proposal, not a finding; it is option (a) of `3-D1` in the brief,
and it changes phase 2's enumeration, because every lane is a local head and
`for-each-ref refs/heads` (`project.go:241`) would list each lane as a branch in the rail.

**Option (b) of `3-D1` removes the problem instead of naming around it** (§4.2): if each worktree is
its own local clone, each clone can check out a branch literally named `evidence-upload`, because
clones have separate ref namespaces. The agent inside sees the branch the rail shows it under; the
control plane fetches each clone's work into the project repository under
`refs/panacea/lanes/<worktree-id>`, which `for-each-ref refs/heads` never lists, so a terminal's
`git branch` stays clean too. "Lane" in the rest of this survey means *a worktree's line of work*,
whichever option carries it.

### 3.2 Situation A — two sessions in one worktree

`FUNCTIONAL-SPEC.md:18`: *"you routinely want a shell open beside an agent, or two agents on
different parts of the same checkout."* Each session is its own sandbox with **the same host
directory bind-mounted** (`handleSessionCreate` in `api/sessions.go`). So they share one filesystem, one index and one
`HEAD`. Isolation between them is zero, and git makes the sharing sharper:

- **One index.** Agent A's `git add -A` stages agent B's half-finished edit; A's commit then carries
  B's work. Any per-commit attribution (obligation 5) names A for B's lines.
- **One `HEAD`.** An agent that runs `git checkout`, `git stash` or `git reset --hard` destroys the
  other's uncommitted work. This programme's own orchestration has hit exactly this: its working
  rule for parallel fix agents is *"never stash/reset shared tree; use worktrees"* (the
  orchestrator's notes, not a repository file — cited as experience, not as evidence).
- **One lock.** Two `git commit`s race on `index.lock`; the loser fails with a message the agent
  may retry, or may "fix" by deleting the lock.

**File claims here would be a real lock**, and a lock needs enforcement. Enforcement on an
unmodified CLI means intercepting its writes, which phase 2's rule forbids
(`2026-09-18-phase-2-backend-and-cockpit.md:49`: *"Agent CLIs are never modified, wrapped or
parsed"*). What phase 3 can do without breaking it is in §3.5.

### 3.3 Situation B — sessions in different worktrees (lanes) of one branch

Separate directories, separate indexes, separate `HEAD`s. **No write can collide.** The collision
is deferred to convergence: two lanes that both changed `services/api/routes.go` conflict when the
second one lands. Here a "file claim" is **not a lock — it is a forecast of a merge conflict**, and
git can compute it with no agent's co-operation:

| Fact | git | Cost |
|---|---|---|
| Files a lane has committed since it forked | `git diff --name-only <merge-base>..<lane>` | cheap |
| Files a lane has touched and not committed | `git status --porcelain=v2` in that worktree | cheap; takes no lock with `GIT_OPTIONAL_LOCKS=0` |
| **Whether two lanes actually conflict, and in which files** | `git merge-tree --write-tree --name-only <a> <b>` — a real three-way merge in memory, no worktree, no index | one call per pair; needs git ≥ 2.38 (this machine: **2.53.0**) |

The third row is the important one: overlap of touched paths is a **suspicion**, `merge-tree` is a
**fact**. The Merge lens can say *"lands clean"* or *"conflicts with `test` in 2 files"* for every
lane in the queue, and recompute after each land, from git alone. Uncommitted edits cannot be
merge-tree'd; they are shown as overlap — suspicion — and labelled that way.

### 3.4 Situation C — sessions on different product branches

Two branches that never merge into each other do not converge in the cockpit; they converge at the
remote, through PRs (obligation 10 and PR creation). Phase 3 can forecast it the same way —
`merge-tree` of each branch against the target — but cannot land it: a PR is merged by the remote's
server, which writes its own merge commit. §7.3 takes the consequence for the merge link.

### 3.5 What "file-claim coordination" can honestly mean

| Kind of claim | Source | Honest in situation | Enforceable? |
|---|---|---|---|
| **Observed** — the paths a lane or worktree has changed | git, as in §3.3 | A, B, C | no — it is a reading, not a lock |
| **Forecast** — lanes that will conflict on landing | `git merge-tree` | B, C | no — but it is exact for committed work |
| **Declared** — an agent says "I am taking `routes.go`" | the agent, over some channel | A | no — the agent can ignore it |
| **Enforced** — a write outside a claim is refused | a filesystem or CLI interception | A | only by wrapping the CLI or the filesystem — **forbidden** |

**Recommendation for the brief:** phase 3 builds observed and forecast claims — both mechanical,
both `AGENTIC-UX.md` §8.1-compliant (*"an index built exclusively from mechanically derivable
facts"*) — and builds **no declared or enforced claim** in v1. Situation A is handled by making the
hazard visible (*"2 agent sessions share this worktree; their commits share an index"*) and by the
owner choosing whether a second agent in one worktree is allowed, warned or refused (`3-D2`).
A declared-claim channel would need an MCP tool or a harness hook; both are exposed to the managed
policy described in §5.3.

---

## 4. Where the repository lives — the question under every other one

### 4.1 Two backends, two answers, and only one is built

`FUNCTIONAL-SPEC.md:11` — *"**Project** — many per machine; = one git repo."* `types.ts:53` —
`Project.path` is *"Absolute path on the machine."* So **the repository lives on the machine**,
and a session's sandbox is a short-lived thing that reaches into it.

| Machine | Where the repo is | How a sandbox reaches it | State |
|---|---|---|---|
| **Local** (Docker) | the laptop's disk, under the projects root | host bind-mount at `/home/dev/work` (`sessionWorkDir` and `handleSessionCreate` in `api/sessions.go`) | **built** |
| **Cloud** (Kata) | not decided — a pod's home is an `emptyDir` (`task-26.md:1013-1015`, citing `docs/verification/2026-09-17-phase-1-results.md:67`) | **nothing** — `katabe.Create` refuses mounts and `WorkDir` (`kata.go:75-89`) | **every create fails** (`task-26.md:963`, seam S8.3) |

Task 26 assigns the Kata half to phase 8 as *"its first problem"* (`task-26.md:963`, `:984-994`).
That is right for the pod plumbing. **It is not right for the git model**, which phase 3 fixes and
phase 8 inherits. If phase 3 designs convergence as "the control plane runs `git` in a directory it
can see", phase 8 discovers that the directory is in a pod on AKS and the control plane is a process
on a laptop.

### 4.2 A defect in the Docker path that phase 3 inherits the moment it creates a worktree

**Inferred from git's documented behaviour, not run.** A *linked* worktree's `.git` is a file
containing `gitdir: <absolute host path>/.git/worktrees/<name>` (`project.go:207` and
`project_test.go:107-113` both note that `.git` is a file there). Session create mounts **only the
worktree directory** (`handleSessionCreate` in `api/sessions.go`). Inside the Linux sandbox, a Windows path such as
`C:/Users/…/.git/worktrees/x` does not resolve, and neither would a macOS host path, so **every
`git` command an agent runs in a linked worktree fails with "not a git repository"**. The primary
worktree works, because its `.git` is a directory inside the mount.

No test covers it: `gitrunner_test.go:104-135` builds a real linked worktree but runs git on the
host. Phase 2 has never needed git inside the sandbox, because it creates no worktrees. Phase 3
creates them, so it owns the fix. Three shapes, in `3-D1`:

| | Mechanism | Blast radius of an agent | Works under Kata? |
|---|---|---|---|
| **a. Worktree + common-dir mount** | also mount `<repo>/.git` and set `GIT_DIR`/`GIT_WORK_TREE` in the sandbox | **the whole repository's refs** — an agent can `git branch -f main` or delete another lane | no — needs a shared filesystem |
| **b. Lane clone** | a plain local `git clone` on the machine — git hard-links the object files on the same disk, so it is cheap and, unlike `--shared`, cannot be broken by a `gc` in the source repo; the clone checks out the product branch under its own name; the control plane converges by fetching from it | **its own clone** — refs elsewhere are unreachable | **yes** — a pod gets a clone; the control plane fetches from it |
| **c. (a) now, (b) in phase 8** | both | changes between phases | only the second half |

Option (b) is the only one whose isolation matches `AGENTIC-UX.md:51`'s *"blast radius: own
worktree only"* and the only one that survives phase 8. Its cost is real: a lane clone is **not** a
`git worktree`, so `git worktree list` (`project.go:282`) no longer enumerates lanes, and "worktree"
becomes a product word rather than a git word. The users *"know the term"*
(`FUNCTIONAL-SPEC.md:13`) and will look for `git worktree list` to agree with the rail. That is the
trade the owner must see.

### 4.3 Git inside the sandbox has no identity either

**Inferred, not run.** The sandbox image installs `git` (`images/sandbox/Dockerfile:10-12`) and
sets no `user.name` or `user.email`. A user's identity usually lives in the host's global
`~/.gitconfig`, which is not mounted. So an agent's first `git commit` should fail with git's
"Author identity unknown". Nothing in phase 2 has tried it.

The fix is the same mechanism obligation 5 needs (§5): **environment the control plane sets on the
sandbox**. `backend.Spec.Env` exists, both backends carry it identically
(`backend/backend.go:175-205`, pinned by `kata_test.go:357-372`), and only `PANACEA_APP_*` keys are
reserved (`backend.go:60`, `:228-233`). Session create passes **no** `Env` today
(`handleSessionCreate`'s `backend.Spec` literal). `GIT_AUTHOR_NAME`, `GIT_AUTHOR_EMAIL`, `GIT_COMMITTER_NAME` and
`GIT_COMMITTER_EMAIL`, read by the control plane from the host's own git config, give every commit
the user's real identity. `CLAUDE.md`: *"Users authenticate as themselves"* — commits should too.

### 4.4 A git API that can run where the repository is

`ExecRunner` runs git in the control plane's own process (`gitrunner.go:46-78`). For a local
machine that is where the repository is. For a cloud machine it is not. **Recommendation:** phase
3's git operations sit behind an interface keyed by machine, with one implementation — local — in
phase 3, exactly as `PROGRAM.md:27-43` did for sandboxes (*"the sandbox backend becomes an interface
with two implementations"*). Phase 8 adds the second. This costs phase 3 an interface and nothing
else, and it is the difference between phase 8 swapping an implementation and rewriting phase 3.

---

## 5. Stamping a session id on an agent's commit — obligation 5

### 5.1 The constraint

`2026-09-18-phase-2-backend-and-cockpit.md:49`: *"Agent CLIs are never modified, wrapped or parsed.
No feature in this phase may read PTY bytes to derive meaning."* `PROGRAM.md:116`: *"Harnesses run
unmodified… Any feature requiring us to parse or replace agent UI is wrong."* The agent runs
`git commit` itself, inside its sandbox, through its own tool. We are not in that call.

### 5.2 Mechanisms, judged against it

| Mechanism | Wraps or parses the CLI? | Verdict |
|---|---|---|
| **A git hook in the image, enabled by environment the control plane sets** — `GIT_CONFIG_COUNT`/`GIT_CONFIG_KEY_0=core.hooksPath`/`GIT_CONFIG_VALUE_0=<image path>` plus a session variable; a `prepare-commit-msg` hook runs `git interpret-trailers --if-exists doNothing --trailer "Panacea-Session: <id>"` | **No.** git runs the hook; the CLI never knows. The control plane sets environment at a boundary it already owns — `backend.Spec` | **Recommended** |
| A trailer the control plane adds when it lands a lane | No | Right for the **merge** commit (§7.2), wrong for this: it attributes the land, not the commits inside it |
| Session id in the committer identity (`GIT_COMMITTER_EMAIL=s-3f2a…@…`) | No | **Rejected.** Pushed commits would carry a fabricated committer; remotes that verify committer email reject them, and `CLAUDE.md` wants users to act as themselves |
| Attribute afterwards by time — commits made while session S was live in that worktree | No | **Rejected.** `FUNCTIONAL-SPEC.md:18` puts many sessions in one worktree; time cannot tell them apart |
| A harness hook (Claude Code `PostToolUse`) reporting each commit | Arguably configuration, not modification — but harness-specific | **Not for this.** Claude Code only, and exposed to the managed policy in §5.3 |

**The session id variable.** `backend.Spec` already carries `SessionID` (`handleSessionCreate`, and again in `handleSessionRestart`).
Exporting it into the sandbox is either a new reserved `PANACEA_APP_*` variable written by
`EnvEntries` — a phase 2 contract amendment, in `backend.go` — or a non-reserved key the session
handler puts in `Spec.Env`, which needs no backend change. Either way the value comes from the
control plane, never from the agent or the client. Recommendation: the reserved variable, because
the backend already owns the three that exist (`backend.go:186-190`) and a session's identity
belongs with them. Note `backend.go:138`: *"SessionID … for the label only; a backend never reads it
back"* — exporting it changes that comment's truth, which is why it is an amendment.

### 5.3 Its limits — state them on screen, not only here

1. **Advisory.** An agent can unset the variable, pass `-c core.hooksPath=`, or commit with
   plumbing (`git commit-tree`). `prepare-commit-msg` is **not** skipped by `--no-verify` (git's
   documented behaviour; not run). A commit without the trailer renders *"no session recorded"* —
   the same words whether it was made outside the cockpit or the hook was bypassed, because we
   cannot tell which.
2. **It names the session that ran `git commit`, not the sessions whose edits are in it.** In a
   worktree shared by two agents (§3.2), A's commit can carry B's lines.
3. **It shadows the repository's own hooks.** `GIT_CONFIG_*` outranks repository config, so a repo
   using `core.hooksPath` (husky and similar) silently loses its hooks. The image's hook must chain
   to the repository's configured hook of the same name. Unbuilt, and it must be tested against a
   real husky repo before it ships.
4. **A shell session's commits are stamped with the shell session's id.** That is correct — the
   commit was made there — and the Files lens must say "shell", not imply an agent.
5. **Rewrites.** Amend, cherry-pick and rebase re-run `prepare-commit-msg` for new commits
   (inferred for rebase; not run). With `--if-exists doNothing`, a rebased commit keeps its original
   session, which is the author of the change, not of the rebase. A squash at the remote drops
   trailers from the trunk commit; the lane's commits keep them.
6. **The managed Claude policy does not reach it, which is the point.** This laptop's managed
   policy strips `settings.json` hooks and MCP from Claude Code
   (`strictPluginOnlyCustomization: ["hooks", "mcp"]`, per the user's device notes). **Whether the
   same remote policy applies to Claude Code inside a sandbox after `/login` is not verified** — it
   is fetched per account, so it probably does. A git hook is outside Claude Code entirely and is
   unaffected. Any phase 3 mechanism built on Claude Code hooks or MCP inherits this risk; one built
   on git does not.

**One environment block answers three needs.** The same `Spec.Env` carries the user's identity
(§4.3), `core.hooksPath`, and the session id. Phase 3 writes it once, in session create and
restart — the file phase 4 also wants for Materialise (`dependency-graph-and-order.md:128`,
*"SERIALISING — the Materialise call site"*). That is a scheduled collision, and §9 of the brief
names it.

---

## 6. Identity, rename and deletion

### 6.1 Worktree lifecycle — what phase 3 creates and destroys

Phase 3 is the first code allowed to run `git worktree add`, `git branch`, `git worktree remove`
and `git branch -d`. Every one of them changes an id phase 2 derives (§1.3):

| Verb | git | Id effect | Who else notices |
|---|---|---|---|
| Create a lane | `worktree add` + set upstream to the parent under `3-D1` (a); a local clone under (b) | new `w-…`; under (a) also a new local head, so a new `b-…` in phase 2's branch list; under (b) no new head in the project repo | the rail (every poll) |
| Rename a branch | `branch -m` | **new `b-…`** — the old id vanishes | phase 4's brief (`branchId`), phase 7's per-branch view, `pinned`, `branchDefaults` |
| Remove a lane | `worktree remove` + delete the lane head | `w-…` gone; `b-…` gone | sessions on it; the `ready` flag in the registry (`filestore.go:422`) |
| Delete a product branch | `branch -D` + every lane | everything below it | every store keyed by branch — and the brief head |

**Recommendation — rename is a first-class event, not a delete plus a create.** The control plane
records `old b-… → new b-…` in its own state when it performs the rename and serves the mapping, so
phase 4 can re-point `Brief.branchId` and the desktop can move `pinned`/`branchDefaults`. A rename
done outside the cockpit (`git branch -m` in a terminal) cannot be told from a delete plus a create,
and renders that way. This is an addition to the Go contract (a new route), phase 3's to write.

### 6.2 Branch deletion — obligation 4, accepted and widened

The contract's P13 protects brief **revisions**. Phase 3's deletion also has to protect **commits**,
and that half is in git, not in any of our stores:

- A release names `Deployment.Commit` (contract §2.3). A merge link names a merge commit. An
  evidence record (§8.4) names a commit. **Delete a branch whose commits were never pushed and every
  one of those becomes unreachable, and `git gc` eventually deletes the objects.** The manifest then
  names a commit git cannot show.
- **So deletion archives the head as a ref:** `refs/panacea/archive/<branch-id>/<timestamp>` points
  at the deleted tip, and git keeps everything reachable from it. Nothing in our UI lists
  `refs/panacea/*` by default; `for-each-ref refs/heads` (`project.go:241`) never sees it.
- **And deletion asks phase 4 to archive the brief head** — P13's *"may archive"*. The call is one
  way (phase 3 → phase 4), so phase 3 never writes phase 4's store. Revisions are untouched.
- **Refuse, or ask, when a lane has uncommitted work or a live session.** Deleting a worktree with a
  running agent destroys work the transcript cannot restore. `DESIGN-SYSTEM.md:69` prefers
  *"Optimistic mutation with an Undo window, not confirm dialogs"* for agent actions; that rule's
  premise is that the action can be undone, and discarding uncommitted files cannot. So this is the
  one place the lens asks first, and the question names the sessions and the dirty files.

The archive-ref retention is an erasure question as well: a regulated customer who asks for a branch
to be erased is asking for these refs to go too. It joins contract Q5 (`provenance-contract-draft.md:620-623`);
it is not phase 3's to answer.

### 6.3 What phase 3 should not store

Git is the truth for every git fact: tip, ahead/behind, dirty, conflict. **Phase 3 persists none of
them**, in the registry or in the desktop. The desktop receives them the way it receives sessions:
from the control plane, not persisted (`store.ts` — *"re-fetched, because the control plane is the
only truth"*, quoted at `provenance-contract-draft.md:541-543`). What phase 3 does persist is what
git cannot hold: the rename map, the land log's undo window (§7.4), evidence records (§8.4) and, if
the owner chooses the ledger, merge links.

---

## 7. Convergence — landing, the merge link, undo, push and PRs

### 7.1 Landing a lane without touching a checkout

The parent branch in the lane model (§3.1) is an **integration ref**: nobody needs it checked out.
git can land into it with plumbing alone, which is what makes merge ordering safe under concurrency:

1. `git merge-tree --write-tree <parent> <lane>` → a tree, or a list of conflicted paths.
2. If clean: `git commit-tree <tree> -p <parent> -p <lane> -F <message>` → a merge commit, no
   worktree touched, no index.
3. `git update-ref refs/heads/<parent> <new> <old>` — **compare-and-swap**: git refuses the update
   if the parent moved since step 1. Two lands racing cannot both succeed; the loser re-runs step 1
   against the new parent and may now conflict. That is merge ordering enforced by git, not by us.

**If the parent branch is checked out in a worktree** (commonly `main` in the primary), a ref move
leaves that worktree's index describing the old tip. Phase 3 must refuse to land into a checked-out
branch, or land through that worktree with a real `git merge`. Recommendation: refuse, and say why.

**Conflicts are resolved in the lane, never in the parent.** The prototype already says so —
*"Land `holder` first, then rebase the other — I can queue that"* (`prototype/app.html:1157`). The
resolution is the lane taking the parent's new work (`git merge <parent>` or `git rebase <parent>`
in the lane's worktree) and settling the conflict there: by the lane's agent, told to, or by a
human in a shell. The parent only ever receives clean merges. Whether a conflicting lane may be
queued for "rebase me and try again" automatically is `3-D3`; the land strategy itself (merge commit, squash or rebase) is `3-D4`.

### 7.2 The merge link — obligation 2, and its home

**Written at the land, by the operation that lands.** The contract's P7 discipline for releases —
*"a release link written by a second call … a crash between the two calls loses the link"*
(`provenance-contract-draft.md:417`) — applies with the same force here.

| Home | Written atomically with the land? | Survives loss of our state directory? | Visible to a teammate, or to Panacea? | Cost |
|---|---|---|---|---|
| **A trailer on the merge commit** — `Panacea-Brief: brief:b-42@3`, `Panacea-Brief-Digest: sha256:…` in step 2's message | **yes** — it is inside the commit `update-ref` publishes | **yes** — it is in the repository | **yes**, once pushed | a brief **key** — not text — enters git history; forgeable by anyone who can commit |
| A ledger record in `statepath.Dir("provenance")` | **no** — a second write after `update-ref` | no | no | a crash window; a per-person store (phase 7 D1) |
| Both — trailer is the record, ledger is an index rebuilt from `git log --format=%(trailers)` | yes | yes | yes | the index is derived, so it can never disagree with the record for long |

**Recommendation: the trailer is the record; the ledger, if kept, is a derived index.** The
contract lets phase 3 choose either (`:184`); this choice is the only one that satisfies P7's
reasoning. It does not breach P5 (*"parsing a brief key outside the `provenance` package"*): phase
3 copies the key string it was handed by the resolver into the message and, reading it back, hands
the string to the resolver. It never splits it. The owner decides whether a brief key in git
history is acceptable to a regulated customer — `3-D6`. It reveals an id and a digest, not the
brief's text, which is a smaller question than `CLAUDE.md`'s.

**Which key.** The key the `DefinitionOfDone` above the queue resolved at the moment of landing —
the head `Ref` (contract §4.4). Landing against a superseded view is possible (the head moved
between render and click); the land uses the key the human **saw**, carried in the request, and the
server refuses if it no longer resolves.

### 7.3 Lane → branch is ours; branch → trunk is the remote's

A PR is merged by GitHub's or Azure DevOps's server, which writes its own merge commit, possibly a
squash. **Phase 3 cannot put a trailer there.** It can put the brief key in the PR description, as
text, labelled as text — not as a provenance record. So the merge link exists for lands, and a
release shipped from trunk after a PR carries the merge links of the lands inside it, found by
walking first-parent history. Phase 7's manifest shows *"reviewed against rev 3 at land"* from that
walk. That is contract §3.4's *"two facts, not a choice"* with a second source.

### 7.4 Landed history, with undo

`AGENTIC-UX.md` §3 LANDED is *"bulk-acceptable, low attention"*, and `DESIGN-SYSTEM.md` puts an Undo
window on optimistic actions. For a land there are two undos, and the UI must not pretend they are
one:

| State of the land | Undo | Reversible exactly? |
|---|---|---|
| Not pushed, nothing landed after it | `update-ref <parent> <before> <merge>` — the ref moves back | **yes** — the merge commit is kept under `refs/panacea/undone/…` so its merge link still resolves |
| Not pushed, later lands on top | a revert commit (`merge-tree` of the inverse), landed like any other | no — history shows the revert |
| Pushed | a revert commit, then push | no — and never a force-push |

**Force-push is out of scope for v1** and should stay out: it rewrites what a teammate or Panacea's
pipeline has already fetched.

### 7.5 Push, ahead/behind — obligations 6 and 10

- **Push runs on the machine, from the control plane, with the user's own credentials** — never
  from the sandbox. The sandbox has no credentials and should not: a leaked token there is a push
  to any repo the user can reach. `ExecRunner` already sets `GCM_INTERACTIVE=never`
  (`gitrunner.go:62`), so a push with no cached credential fails with git's own sentence rather than
  a hidden prompt. Surfacing that sentence is the whole first-run story in v1.
- **Ahead/behind** is two numbers per branch from `git rev-list --left-right --count
  <branch>...<branch>@{upstream}`, where the upstream is the real remote for a product branch and the
  parent for a lane (§3.1). Phase 7's *"2 local commits are not pushed and will not ship"* is the
  first number for the product branch, rendered beside Ship. **A branch with no upstream has no
  ahead/behind** — it has *"never pushed"*, a different fact. Stale remote refs are a third: the
  numbers are as of the last fetch, and the lens says when that was.
- **Fetch is not free** and must not run on the 2 s poll. It is on demand and on a slow timer,
  owned by phase 3, and its time is shown.
- **PR creation needs a host API and a token** — GitHub (KPMG's EMU org) or Azure DevOps. Neither
  host is named anywhere in `PROGRAM.md` or the briefs, and the control plane has no credential
  store (`provenance-contract-draft.md:575-577`: no `keyring`, `credential` or `wincred` module).
  Phase 7 needs the same store for its `pnc_` token. **One store, chosen once, for both** — `3-D8`.

---

## 8. Diff review, transcript back-links, and the two lenses

### 8.1 What the owner specified

`AGENTIC-UX.md:206` (§8 item 5): *"The diff is a co-equal second surface with back-links into the
transcript. Each hunk links to the span where it was written and why, constructed mechanically from
timestamped file-write tool calls. Review at the level of 'what changed about the product', with the
diff as drill-down, because the user is deciding acceptance, not authorship. This is the missing
piece in every PR-returning competitor and the strongest differentiator available."*

And the hard rule over everything: *"if you cannot name the transcript span that justifies a UI
element, the element must not exist"* (`AGENTIC-UX.md:41`).

### 8.2 Where the file-write tool calls are — and the ruling it needs

A "timestamped file-write tool call" is an entry in the harness's own transcript. For Claude Code
that is a JSON-lines file under `~/.claude/projects/`, named by the conversation id. Two facts from
the tree make it reachable **without reading a PTY byte**:

- **The control plane is about to mint that id itself.** The in-flight resume fix changes
  `harness.Request` so that *"The control plane mints Conversation and keeps it on the session
  record, which is how it can later supply Resume without ever reading what the CLI printed"*
  (working-tree diff of `harness/harness.go`, `Request` doc comment; task 26 seam SX.10 names
  `--session-id <uuid>`).
- **The conversation store is about to outlive the sandbox.** The working-tree
  `images/sandbox/Dockerfile:26-32`: *"The control plane bind-mounts each worktree's conversation
  store over ~/.claude/projects"*. Uncommitted, and the mount itself is not yet in `api/` (a
  `git grep` for it finds only the Dockerfile).

So the control plane will know **which file** holds a session's transcript, by an id it chose. Reading
the **contents** is another matter. The file is the harness's private, undocumented, versioned
format. Phase 2's rule says *"never … parsed"* and *"No feature … may read PTY bytes"*; it does not
name transcript files. `PROGRAM.md:116` forbids parsing *"agent UI"*; a transcript file is not UI.
**The two readings disagree, and a phase 3 plan cannot be written until one wins** — `3-D7`.

### 8.3 Back-links in two tiers, so the answer to `3-D7` changes depth, not shape

| Tier | Chain | Needs | Harnesses | Granularity |
|---|---|---|---|---|
| **1 — always** | hunk → the commit that introduced it (`git blame` on the lane) → its `Panacea-Session` trailer (§5) → that session, opened at the commit's time | git, and the hook | **every** harness and shells | commit — *"written by `upload handler` in commit `a41c9e2`, 14:02"* |
| **2 — if `3-D7` allows** | hunk → the `Edit`/`Write` tool call whose `file_path` and new text match it → that span | reading the transcript file | Claude Code only, until each other harness's file is read | tool call — the AGENTIC-UX promise |

Tier 2's honest limits: an edit made through a shell command (`sed`, a code generator, a
formatter) has no file-write tool call and matches nothing; a region edited three times matches the
last edit only; uncommitted edits have no commit for tier 1. Each of those renders as the tier it
reached, with the reason — never as a link that lands on the top of a session (`AGENTIC-UX.md:74`
fails that).

### 8.4 The criterion evaluator — obligation 1

Contract §2.3: the evaluator *"takes `(brief key, commit)` and returns states"*, and evidence is
*"keyed by brief key and criterion id together"* (`provenance-contract-draft.md:209-213`). Nothing
says what evidence **is**. Natural-language criteria cannot be judged mechanically; an LLM verdict
is a claim, not a fact, and `AGENTIC-UX.md:202` builds the index *"exclusively from mechanically
derivable facts"*. So the evaluator is a **function over recorded evidence**, and the only question
is who records it:

- **Evidence record:** `{ briefKey, criterionId, criterionDigest, commit, verdict (settled |
  contested), because, recordedBy, recordedAt }`. `because` is required — P12 forbids a non-`open`
  state without one.
- **State at commit C:** the evidence whose `commit` is C or an ancestor of C
  (`git merge-base --is-ancestor`). None → `open`. Where one record's commit descends from another's,
  the descendant supersedes. Two unrelated records that disagree → `contested`. Otherwise the
  verdict they share. That is mechanical, it is defined for any commit, and it is exactly what makes
  the three surfaces of contract §2.3 agree: the Develop chip at a worktree head, the Merge view at
  the parent head, the Deploy manifest at `Deployment.Commit`.
- **A useful consequence:** landing a lane makes its evidence reachable from the parent. The Merge
  queue can say *"landing `upload` settles criterion 2"* before the land, from reachability alone.
- **Criterion identity across revisions.** The plan's criterion id is positional
  (`provenance-contract-draft.md:210-213`). Carrying evidence from revision 3 to revision 6 is safe
  **only if the criterion's text is unchanged**; `criterionDigest` makes that a byte comparison, and
  the surface says *"evidence recorded against rev 3"* when it carries. A contract amendment,
  proposed here.
- **Who records, in v1:** a human, in the Merge lens, per criterion, at the commit in front of them.
  An agent proposing evidence needs a channel (MCP or a harness hook — §5.3's risk). Test-linked
  evidence (a criterion bound to a test, the verdict from a run) needs a runner at an arbitrary
  commit. Both are `3-D5`.

### 8.5 The Merge lens — what it shows

Built from the prototype's composition (`prototype/app.html:1144-1170`: *"Merge into ${branch}"*,
DoD on top, *READY TO MERGE* in order, *LANDED* below), with each element tied to a fact:

| Element | Fact it stands on |
|---|---|
| `DefinitionOfDone`, head `Ref`, states at the parent head (obligation 8) | phase 4's resolver; §8.4's evaluator at `refs/heads/<parent>` |
| The queue: lanes marked ready, in the order the human set | `Worktree.ready` (`types.ts:85`, seam S3.3); order through `moveWithinVisible` (seam S3.4) |
| Per lane: *lands clean* / *conflicts with X in N files* | `git merge-tree --write-tree` against the parent **as it will be** after the lanes above it |
| Per lane: files changed, commits, sessions | `git diff --name-only`, `git log`, trailers |
| Per lane: *not committed: 3 files* | `git status --porcelain` in the lane — **uncommitted work does not land**, and the row says so |
| Per lane: *landing settles criterion 2* | evidence reachability (§8.4) |
| Landed: each land, its merge link's brief key, undo | first-parent history of the parent + trailers; §7.4 |
| Ahead/behind of the parent against its remote | §7.5 |

**Not on it:** a count of ready lanes, a percentage, a "2 of 4 criteria" — `AGENTIC-UX.md:217-218`
deletes exactly that from v3.

### 8.6 The Files lens — obligation 9, and a proposal because no document has one

`AppShell.tsx:49`: *"The worktree's files, with what each session changed."* Read against
`AGENTIC-UX.md` §2 property 2 (*"Change is the unit of interest, not state"*), **Files is not a file
browser** — an editor-grade browser is an IDE, and the agent already has one. Proposal:

- **Scope: the branch, across its lanes.** Merge answers *"what lands, in what order"* per lane;
  Files answers *"what changes, and who is changing it"* per path. Same facts, the other axis.
- **Rows: changed paths only**, grouped by directory, relative to the parent's merge base. A path
  shows each lane that touched it, committed or not, and — this is the observed file claim of §3.5 —
  **a path two lanes touch is marked**, with *"conflicts on landing"* when `merge-tree` says so and
  *"both edit this file"* when it cannot yet (uncommitted).
- **Selecting a path opens its diff**, per lane, each hunk with its tier-1 back-link and, if
  `3-D7` allows, tier 2.
- **Empty state is the target state:** *"Nothing has changed on this branch since it left
  `main`."* A branch with no lanes says that instead.

Whether Files is this, a full tree with changes highlighted, or folded into Merge as a second tab is
`3-D9`. Its phase number stays a claim until then (task 26, S3.5).

---

## 9. The obligations, one at a time

### 9.1 Obligation 3 — is `CLAUDE.md` committed? And obligation 7 — the handoff bundle

**The engineering answer is no, and the lane model makes it sharper than the contract saw.** If the
managed block is committed:

- **Two lanes that inherited different revisions conflict on landing, every time.** Lane `upload`
  carries `<!-- panacea:discover:begin brief=b-42 rev=3 -->…`, lane `test` carries `rev=5`; both
  changed the same lines from the merge base, so the second land conflicts in `CLAUDE.md`. Contract
  §2.4 restamps on every process start, so revisions differing across lanes is the normal case.
- **An agent that stages everything commits it whether anyone decided it should**
  (`scope-and-risk-phase-4-discover.md:53-55`, which also notes that a repo already tracking
  `CLAUDE.md` gets a tracked file modified by every Materialise).
- **The brief's text enters git history** — visible to anyone with the repo, pushed with the branch,
  and outside any erasure the owner decides for contract Q5.

What committing would buy — the contract's conditional cross-check (`:317-321`) and a derived
brief reading for releases shipped outside the cockpit — **the merge-link trailer mostly buys
instead** (§7.2, §7.3): a release built from trunk carries the lands inside it, each naming the
brief key it was reviewed against. That is a key and a digest, not the text.

**Mechanism for "not committed":** an untracked `CLAUDE.md` is excluded in the repository's
`info/exclude` (shared by every git worktree of the repo; one per clone under `3-D1`'s option b). A
repository that **already tracks** its own `CLAUDE.md` cannot hold the managed block without
dirtying a tracked file; phase 4 then needs a location Claude Code reads that git does not track.
The sandbox's user memory, `~/.claude/CLAUDE.md`, is outside the worktree entirely and inside the
home the control plane already provisions — **documented Claude Code behaviour, not run in the
pinned image**, and a phase 4 decision, not this survey's.

**The handoff bundle follows the same answer** (`scope-and-risk-phase-5-design.md:107` asks for
exactly that): excluded, materialised with its marker, never committed. What an agent *builds from*
the bundle — tokens copied into source, components written — is code and is committed like any code.

### 9.2 All twelve

| # | Obligation | Verdict | Why, in one line |
|---|---|---|---|
| 1 | Criterion evaluator over `(brief key, commit)` | **Accepted** — as a function over evidence records and ancestry (§8.4) | the only definition that is mechanical and agrees across the three surfaces; *who records evidence* goes to the owner (`3-D5`) |
| 2 | Write the merge link; choose its home | **Accepted — trailer on the merge commit, ledger only as a derived index** (§7.2) | the only home written atomically with the land; brief key in git history goes to the owner (`3-D6`) |
| 3 | Is `CLAUDE.md` committed? | **Put to the owner, recommending no** (§9.1) | lanes conflict on every land if it is; the trailer replaces most of what committing buys (`3-D6`) |
| 4 | Branch deletion archives, never deletes a revision | **Accepted and widened** (§6.2) | revisions are phase 4's; phase 3 must also keep **commits** reachable with an archive ref, or releases name objects `gc` deletes |
| 5 | Session id on every agent commit | **Accepted, by a git hook enabled by sandbox environment** (§5) | respects phase 2's no-wrap rule; advisory, and the limits are drawn on screen |
| 6 | Ahead/behind for Ship | **Accepted** (§7.5) | two numbers from `rev-list`; *"never pushed"* and *"as of last fetch"* are separate facts |
| 7 | Is the handoff bundle committed? | **Put to the owner with obligation 3, recommending no** (§9.1) | one answer for both, as the phase 5 brief asks |
| 8 | Mount `DefinitionOfDone` above the merge list | **Accepted** (§8.5) | head `Ref`, states at the parent head, no denominator |
| 9 | Own Files and Merge | **Accepted for Merge; Files accepted with a proposed meaning the owner must confirm** (§8.6) | no document says what Files shows (`3-D9`) |
| 10 | Push (P3 → P7b edge) | **Accepted** (§7.5) | control plane pushes with the user's credentials; the sandbox never holds them |
| 11 | Worktree creation | **Accepted** (§6.1) | and every created worktree resolves through `Provider.Path` (seam S3.2) |
| 12 | The real worktree layout | **Accepted, gated on `3-D1`** (§4.2) | the layout is the clone-versus-worktree decision |

---

## 10. Not phase 3 — declined, with the owner named

| Item | Why not here | Who |
|---|---|---|
| **Authority** (`TopBar.tsx:29`, open carry 8) | an `AGENTIC-UX.md` §7 grid over action classes; it needs the agent-runtime signal `AGENTIC-UX.md:60` calls *"the single most important non-UI implication"*. Nothing about git | owner — `3-D10` |
| Carry 11 (`observe` drops `dirty`) | output-tracker throttle, not git; the phase number is probably pre-renumber (§1.6) | orchestrator |
| Getting a lane into a Kata pod | pod plumbing is phase 8's (S8.3). **Phase 3 owns the model** (`3-D1`) so phase 8 swaps an implementation (§4.4) | phase 8, against phase 3's interface |
| Control-plane authentication | phase 3 adds write routes behind the same loopback boundary contract §6.6 already questions — a push route makes another local user able to push as you | owner — contract Q6, now with a phase 3 row |
| An event stream | phase 3 is poll-friendly: every fact is a git read. Nothing here needs push | — |

---

## 11. What I did not verify

- **Nothing was run.** No build, no test, no container, no `git` command that writes. The only
  executions were reads: `git grep`, `git worktree list`, `git --version` (**2.53.0.windows.2**),
  `git diff --stat`, and `ls`/`cat`/`grep` of named files.
- **§4.2's defect — git fails in a linked worktree inside the sandbox — is inferred** from git's
  documented `.git`-file format and the single-directory mount. This repository has no linked
  worktree for me to read a `gitdir:` line from. One `docker run` against a real linked worktree
  settles it in a minute.
- **§4.3 — no commit identity in the sandbox — is inferred** from the Dockerfile. Not run.
- **git behaviours cited from documentation, not run here:** `worktree add` refusing a checked-out
  branch; `merge-tree --write-tree` (git ≥ 2.38); `update-ref` compare-and-swap;
  `prepare-commit-msg` surviving `--no-verify`; `GIT_CONFIG_COUNT` (git ≥ 2.31) outranking
  repository config; `prepare-commit-msg` on rebase.
- **Claude Code behaviours, not run in the pinned 2.1.274 image:** the transcript file's location
  and format; that `~/.claude/CLAUDE.md` is read as user memory; whether the managed remote policy
  that strips hooks and MCP on this laptop also applies inside a sandbox after `/login`.
- **The resume fix is in flight.** The conversation-store mount is described in the working-tree
  `Dockerfile` and not yet in `api/`. §8.2 depends on it landing as described.
- **Task 26 is a plan, not code.** Every seam id in §1.6 is its proposal; none has an anchor yet.
- **Which remote hosts Panacea projects use** (GitHub EMU, Azure DevOps, or Panacea's own) is not
  stated anywhere I read; §7.5's PR row depends on it.
- **I did not open the phase 4 plan** (`2026-09-18-phase-4-discover.md`); its line numbers here
  come through the contract and the phase 4 brief.
- **No rendering.** Nothing in this survey is a claim about how anything looks.






