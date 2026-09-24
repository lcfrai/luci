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
- `images/sandbox/Dockerfile` — a comment only, directly above the `RUN npm install -g
  @anthropic-ai/claude-code@…` line (`:17` at `3303fd6`), so a pin bump cannot miss OQ-10.
  A comment starts no build step, so no build changes:
  ```dockerfile
  # Phase 4 OQ-10: at 2.1.277 or later Claude Code reads AGENTS.md, and a materialised
  # CLAUDE.md displaces it. A bump to that range re-runs canaries O2 and O6
  # (docs/superpowers/plans/2026-09-23-phase-4-discover-parts/task-23.md) and lands
  # the OQ-10 import batch in the same change.
  ```

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
- [ ] **Step 10: A black-holed Hub answers inside every desktop budget (review I4).** Step 9's
  stopped Hub refuses connections at once, so it passes with or without any budget. This step
  makes the Hub **silent** instead:
  1. Pre-check that the address black-holes rather than refuses:
     `curl -sS --connect-timeout 3 -o /dev/null http://10.255.255.1:8750/ > $OUT 2>&1; echo $? > $RC`.
     Exit 28 (timed out) is the black hole this step needs. Exit 7 (refused) or any answer means
     this network routes `10/8`. Then use `192.0.2.1` (RFC 5737 TEST-NET-1, never routed) and
     re-check. If neither times out, record **not run** with both exit codes.
  2. Kill the control plane by PID. Restart the same binary with
     `PANACEA_APP_HUB_URL=http://<that address>:8750`, keeping every other variable (the
     Docker-daemon lock and `PANACEA_APP_REAP_INTERVAL=24h` still apply: a restarted control
     plane's reaper sweeps at start-up).
  3. Each call below runs with `curl --max-time` set to the desktop's own budget, and records
     `%{http_code}` and `%{time_total}`. A curl exit of 28 is a **fail**: the desktop would have
     shown `timeout`.

     | Call | `--max-time` (desktop) | Budget | Expected |
     |---|---|---|---|
     | `POST /v1/sessions` on the step-4 worktree | 10 (`DEFAULT_TIMEOUT_MS`) | 5 s (task 09's `HubBudget`) | `409 brief_not_written`, `reason: hub_unavailable`, in 5 to 6.25 s; no record created |
     | `GET …/branches/{b}/brief` | 10 | 8 s (task 11's `Budgets.Default`) | `503 upstream_unavailable`, `upstream: "hub"`, in 8 to 9.25 s |
     | `POST …/branches/{b}/brief/ops` with `{"baseRevision": <head>, "op": {"kind": "propose_follow_up"}}` | 30 (`DISCOVER_FOLLOW_UP_TIMEOUT_MS`) | 28 s (`Budgets.FollowUp`) | `503 upstream_unavailable`, in 29.25 s at most |
     | `POST …/analyse` | 60 (`DISCOVER_ANALYSE_TIMEOUT_MS`) | 55 s (`Budgets.Analyse`) | `503 upstream_unavailable`, in 56.25 s at most (earlier if the model answers fast, because hubclient's three attempts then run out before the budget) |

  4. Read each row against its budget. Each failure time has its own cause (Hub ask **HA4-10**,
     plan §9). Record the cause, never "flake".
     - **At the budget:** HA4-10's (a), (b) and (c) all hold.
     - **Between the budget and 1.25 s past it:** (a) holds, and (b) or (c) is missing. The Hub's
       task 33 retries the caller's own expiry and sleeps with `time.Sleep`, which costs up to
       250 ms + 1 s. This is a **pass**: every budget is sized to absorb it (task 11, "Back-off
       that cannot be interrupted"). Record which rows showed it.
     - **curl exit 28, or more than 1.25 s past the budget:** a **fail**. Run that one call again
       with `--max-time 70` and record `%{time_total}`. About 31 s after the call's first Hub read
       means (a) is missing: hubclient's attempts do not derive from the caller's `ctx`, so no
       budget can cut them short. Then `discoverhub` needs the fallback HA4-10 declares.
  5. Restore the real `PANACEA_APP_HUB_URL` and restart the control plane by PID.
  That `10.255.255.1` black-holes on the corporate network is **unverified**, and sub-step 1
  exists to find out.
- [ ] **Step 11: 4-D3.** Give the project brief five answers; confirm the branch brief moved to a
  new revision whose `inherits.key` is the project's new key, in the **same** `written` list.
- [ ] **Step 12: The suites and the guards.** `go test ./control-plane/... -race -count=1`;
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
| O6 | A repository that tracks a root `AGENTS.md` holding a second codeword (*"The AGENTS codeword is TOURMALINE."*) and has no `CLAUDE.md` or `.claude/AGENTS.md`. First open and close one throwaway session in the same sandbox, because the memory page says the first session after a CLI upgrade reads only `CLAUDE.md` files; below v2.1.281, none of the sessions may be on Bedrock or have telemetry disabled, for the same reason. **Baseline arm:** a session on the same clone and the same CLI with **no** brief, so nothing is materialised; run `/memory` and look for `AGENTS.md`'s path, then ask for the second codeword. **Brief arm**, only when the CLI from step 6 is v2.1.277 or later: repeat O1, run `/memory` again, and ask for the second codeword | **Below v2.1.277** (the pin is 2.1.274, `images/sandbox/Dockerfile:17`) only the baseline arm runs, and it checks the premise OQ-10's recommendation rests on: the docs say that CLI reads no `AGENTS.md` at all. The brief arm is recorded **not applicable**, with the version. A baseline that lists `AGENTS.md` or answers TOURMALINE disproves that premise, and OQ-10 goes back to the owner. **At v2.1.277 or later**, displacement is shown only when the baseline arm lists `AGENTS.md` and answers TOURMALINE and the brief arm does neither. A baseline that does neither makes O6 **inconclusive**, never "displaces" (P4-D, OQ-10) |

**Acceptance:** `docs/verification/2026-09-23-phase-4-results.md` exists and holds, for every
step: the command, its output (trimmed), and **pass**, **fail** or **not run** with the reason;
the `CLAUDE.md` bytes from step 4; the CLI version from step 6; the O1–O6 table marked
**unverified — owner, OD-12**. A failure is written as a failure.

**Does NOT claim:** anything in O1–O6.
