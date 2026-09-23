### Task 23: End-to-end verification, and the owner's batched steps written down

No new product code. This task proves, against running processes, the claims the phase lives on:
a brief written in Discover reaches the Hub, a session start writes exactly its latest complete
revision into the worktree **before** the agent starts, the session carries that key, and the chip
resolves it. It then writes down — without running — the steps only the owner can do (`OD-12`),
and every result is recorded honestly, failures included. **A screenshot is not evidence**
(`CLAUDE.md`), and a green suite is not either.

**Files:**
- `docs/verification/2026-09-23-phase-4-results.md` (new)
- `docs/verification/phase-4-e2e.sh` — the scripted part, one `curl` per step (new)

**Preconditions, each checked and recorded** (a missing one is a recorded **not run**, never a
fabricated pass):

| # | Needs | Check |
|---|---|---|
| P1 | the Hub's local stack (H0–H3) | `docker compose -f control-plane/cmd/hub/compose.yaml ps` shows `hub` healthy (Hub contract §12.1) |
| P2 | a sign-in against `devissuer` (C1) | `GET /v1/auth/status` → `signed_in` |
| P3 | phase 3's `githost` (task 13) | `go doc ./control-plane/internal/githost ParseRemote` |
| P4 | the phase 2 sandbox image | `docker image inspect panacea-app/sandbox:dev` |

**The scripted part** (`phase-4-e2e.sh`, run through claudezsh; the control plane started with
`PANACEA_APP_PROJECTS_DIR`, `PANACEA_APP_STATE_DIR` and the Hub's variables — `config.go:23-45`
lists the control plane's own):

- [ ] **Step 1: A real repository with a shareable remote.** `git init` a directory under the
  projects root, add `README.md`, and `git remote add origin https://github.com/lcfrai/phase4-e2e.git`
  (the URL is parsed, never fetched). Expected: `GET /v1/discover/projects/{id}/binding` →
  `unbound`.
- [ ] **Step 2: Share it.** `POST …/binding {"teamId": …}` → `bound`.
- [ ] **Step 3: Answer five questions** through `POST …/branches/{b}/brief/ops`, the purpose
  containing a codeword only the file can supply: *"The codeword for this branch is ORTHOCLASE."*
  Expected: `written` keys `@1`…`@5`, the fifth `complete`, `preview.block` containing `ORTHOCLASE`.
- [ ] **Step 4: Start a session** with `POST /v1/sessions` on the branch's worktree, sending a
  bogus `"contextRef": "brief:bf-0000000000000000@1"`. Expected: the response's `contextRef` is
  the `@5` key — **not** the bogus one (P6); `CLAUDE.md` on disk equals `preview.block` inside its
  markers; `.git/info/exclude` holds `/CLAUDE.md` once; `git status --porcelain` shows no
  `CLAUDE.md` (3-D6).
- [ ] **Step 5: Read the file as the sandbox reads it.** `docker exec <sandbox> cat
  /home/dev/work/CLAUDE.md` as the container's user. Expected: the same bytes (task 09's
  permission question, answered by observation).
- [ ] **Step 6: Record the pinned CLI version.** `docker exec <sandbox> claude --version`.
  Recorded; the canaries below are about **this** version.
- [ ] **Step 7: A hand-written section survives.** Append `## House rules\n\nNever touch infra.`
  outside the block; edit one answer; restart the session. Expected: the house rules survive; one
  begin marker; the session's `contextRef` is `@6` (restamped).
- [ ] **Step 8: The thread.** `GET /v1/discover/refs?key=<@5>&key=<@6>&key=none:declined&key=`.
  Expected: `@5` resolved with `latestCompleteRevision: 6` (so the desktop shows it superseded),
  `@6` resolved, `none` with `declined`, `none` with `""`; no field anywhere is a count, a percentage
  or a ratio.
- [ ] **Step 9: 4-D1.** Stop the Hub (`docker compose … stop hub`); start a session. Expected:
  `409 brief_not_written`, `reason: hub_unavailable`, no record created. Repeat with
  `"withoutBrief": true`. Expected: 201, `contextRef: "none:declined"`, and no managed block in
  `CLAUDE.md`. **Then the restart refusal (review C1):** with the Hub still stopped, record the
  step-7 session's `sandboxId` and `contextRef` from `GET /v1/sessions/{id}`, then
  `POST /v1/sessions/{id}/restart` with an empty body. Expected: `409 brief_not_written`,
  `reason: hub_unavailable`; `docker ps --filter label=panacea-app.sandbox-id=<sandboxId>` still
  lists the old container, running; a second `GET` shows the same `sandboxId`, status and
  `contextRef`; the pane still attaches and echoes a keystroke. A restart that kills the agent and
  then refuses fails this step. **Then the restart override (re-review I1):** with the Hub still
  stopped, `POST /v1/sessions/{id}/restart` with `{"withoutBrief": true}`. Expected: 200,
  `contextRef: "none:declined"`, a `sandboxId` different from the one recorded above, the old
  container gone from `docker ps --filter label=panacea-app.sandbox-id=<old sandboxId>`, and no
  managed block left in that worktree's `CLAUDE.md`. A second 409 here means the restart dropped
  `withoutBrief` on its way to `Materialise`. Start the Hub again.
- [ ] **Step 10: 4-D3.** Give the project brief five answers; confirm the branch brief moved to a
  new revision whose `inherits.key` is the project's new key, in the **same** `written` list.
- [ ] **Step 11: The suites and the guards.** `go test ./control-plane/... -race -count=1`;
  `cd desktop && npx vitest run && npx tsc --noEmit`; task 06's and task 16's no-key-splitting
  scans; the `types.ts` byte gate (phase 2 task 15).

**Written down for the owner's batched session (`OD-12`) — not run by this task, and reported as
unverified until the owner runs them:**

| # | Step | What it settles |
|---|---|---|
| O1 | `/login` inside the sandbox as yourself; ask *"What is this branch for, and is there a codeword?"* without pasting the file | **the phase's acceptance criterion**: the agent answers ORTHOCLASE from `CLAUDE.md` |
| O2 | In a repository that tracks `CLAUDE.md`, repeat O1 | whether the pinned CLI reads `.claude/CLAUDE.md` (P4-D, OQ-2) |
| O3 | Edit the brief, then `--resume` the session (the cockpit's Resume) and ask again | whether a resumed conversation acts on the restamped file (brief R7) |
| O4 | Look at every Discover state, the eight chip states on a real pane header, the refusal, the import | the visual acceptance: the owner's eyes, and nothing else |
| O5 | Sign in as two teammates through `devissuer`; one edits, the other watches the chip go superseded within one feed poll | `OD-3`'s "a teammate sees the same brief" |

**Acceptance:** `docs/verification/2026-09-23-phase-4-results.md` exists and holds, for every
step: the command, its output (trimmed), and **pass**, **fail** or **not run** with the reason;
the `CLAUDE.md` bytes from step 4; the CLI version from step 6; the O1–O5 table marked
**unverified — owner, OD-12**. A failure is written as a failure.

**Does NOT claim:** anything in O1–O5.
