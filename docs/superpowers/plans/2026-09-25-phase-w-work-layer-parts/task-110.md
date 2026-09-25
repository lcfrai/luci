### Task 110: the read-only *ask* worker: answers that link files and lines

Stream **W3** (agents on the work), about 3 hours (an estimate, not a measurement). Depends on
task 101 (the W3 verbs and their dispatch rows) and **TM-21** (`fleetbe` and the team launch on the
local control plane, Phase T plan `:636-643`, read at `64ab2c4`; **UNVERIFIED until merge**). Uses
TM-09's control wire v1 (`{ "v": 1, "id", "verb", "args" }` and its response, plan `:506-514`),
TM-20's `Launch(owner, project, class, harness, delegation)` and its gateway-key tags (`:624-631`),
TM-13's class invariants (`:546-557`), task 109's `Gate` (`OriginAsk`) and the local control plane's
`project.Runner` (`control-plane/internal/project/project.go:56-58`, built).

L-162 asks for grounded codebase questions (`05-design.md:324`: *"An **ask** worker: read-only
session on the repo; answers link files and lines"*). This part claims one thing: **every link in
an answer resolves**. It does not claim the answer is right.

**Decisions made here, so nobody re-derives them:**

1. **Team projects only.** The ask worker is a Fleet session, so it needs Fleet. In local mode
   the palette command is shown disabled with *"Ask needs a team project"*; L-162 for local
   projects is gap 5.
2. **A fourth class, `ask`.** TM-09's `Class` is `orchestrator`·`worker`·`mirror` (plan `:506-507`)
   and TM-13's defaults are `{worker,orchestrator,mirror}.json` (`:206`); none is read-only. This
   part adds `ask` to both, a delta whose owner phase wins (rule H2), with these invariants on top
   of TM-13's common ones: allowed tools `Read`, `Grep`, `Glob` and `Bash(panacea answer *)`;
   denied `Edit`, `Write`, `MultiEdit`, `NotebookEdit` and every other `Bash`; `defaultMode`
   pinned to `plan`. Fleet's verb table gives the `ask` class **one** verb, `answer`, and nothing
   else: no `lease`, no `report`, no `peer`.
3. **Nothing an ask session does can land.** Its checkout is a throwaway clone at the named
   commit, with no lane and no fetch-back (`OD-2`'s `refs/panacea/lanes/…` is never written for it).
   Even a write that got past the policy would reach nothing a person merges.
4. **The commit is named by the asker.** The desktop sends the commit its own clone has for the
   project's default branch remote-tracking ref, so the worker and the citation check read the
   same tree. If the remote lacks it, Fleet refuses with `commit_not_on_remote`.
5. **The answer comes back through one new verb**, `answer {answer, citations}`, callable once per
   ask session; the session ends after it. The scope names no return channel, and TM-18's
   `report done <text>` caps text at a length and has no typed citations, so this is a delta to
   TM-09's wire (gap 2). Fleet stores the answer for the asker only.
6. **It is the asker's session, billed to the asker.** Fleet launches it with `owner = asker`
   under the asker's delegation (TM-21 creates it as the user), so TM-20's key tags name the
   asker's `oid` (`OD-19`); the project owner is never named. It appears in the asker's own grid as
   `you · agent N`, with the word *ask* beside it.
7. **The control plane checks every citation before render.** On the local control plane, with the
   asker's clone, `git cat-file -e <commit>:<path>` then `git show <commit>:<path>` and a line count.
   A citation is `ok`, `missing_path`, `missing_lines` or `commit_absent`; only `ok` is a link. Any
   other renders as the fact *"not found at 7c1f0a2"* or *"not checked: 7c1f0a2 is not in this
   clone"*. No citation at all reads *"no file cited"*.
8. **An answer never lands on an issue by itself.** *Post as my comment* opens the issue's composer
   (task 50) with the answer quoted under a line `ask (agent) at 7c1f0a2:`; what the person posts
   is their own comment, `text·H`, and they may edit it first.
9. **The switch applies.** Launching an ask calls task 109's `gate.Allow` with `OriginAsk` first;
   with Orchestrators off the ask is refused with `agents_disabled` and the desktop shows
   `agentsOffLine`.

**Files:**
- `control-plane/internal/fleet/work/ask.go` (+ `_test.go`) — launch, the `answer` handler, the
  asker-only read (new; **path UNVERIFIED**, no Fleet row in the file map)
- `control-plane/internal/policy/defaults/ask.json` ⚑ (TM-13's directory) and TM-09's class and
  verb table in `control-plane/internal/fleet/model` ⚑ — the `ask` class (modified; **UNVERIFIED
  until merge**)
- `testdata/panacea-control/v1/answer.*.json` ⚑ (TM-09's fixture set) — the verb's fixtures (new)
- `control-plane/internal/workask/cite.go` (+ `_test.go`) — the citation check on the local clone
  (new; not in the file map)
- `control-plane/internal/api/workask.go` (+ `_test.go`), `api.go` ⚑, `routes_audit_test.go` ⚑ —
  the loopback routes (new, modified; not in the file map)
- `desktop/src/components/work/AskAnswer.tsx` (+ `.test.tsx`) — the question, the answer, the
  checked citations (new)
- `desktop/src/components/palette/commands.ts` ⚑ (task 59's) — *Ask about the code…* (modified)
- `desktop/src/api/work.ts` ⚑ — `ask` and `askResult` (modified)
