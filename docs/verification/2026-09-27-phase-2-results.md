# Phase 2 verification results — 2026-09-27

Phase 2's defining claims — scrollback survives a re-parent, and sessions survive detach and
restart — were **not observed** in this document's run: it stopped at its first check, `setup`,
when the control plane killed a host-git read at its 10 s bound on a host saturated by other work.
The image, its fail-closed start, token rejection and the harness flags were proven, and the day's
runs found three product defects (F1-F3), reported to their owners and not fixed here.

Run on the development machine (Windows 11) against Docker Desktop 29.4.3, image
`panacea-app/sandbox:dev` (`sha256:243189316a3e…`) created `2026-09-23T06:52:05.425529555Z` from
commit `dbaf2ad6aed86885096c29de4827e194086f0a88` (its `panacea-app.source-commit` label), control
plane built from `83660ac2` (its Go is `502f0751`'s: nothing under `control-plane`, `sandboxd` or
`go.work` changed since). Browser automation: Playwright 1.63.0, bundled Chromium build 1243
(Chrome for Testing 153.0.8010.12), headed. Native window: WebView2 `Edg/153.0.4234.48`, reached
over its DevTools port; `w.webview2` did not run (the run stopped at `setup`).

**The host was saturated by other work on the same machine, and it decides how to read every
timing.** Sampled as a session create feels it (five `git config` spawns, then one `docker ps`):
7.6 to 57.5 s for the five spawns between 10:28 and 10:58 local, `docker ps` 11.6 to 18.5 s, and
16-18 other `go.exe` processes running. On such a host a 10 s bound on one git call cannot hold
(F2), and a 10 s client budget on a create cannot either (F1). Those are findings, recorded as
observed; nothing was loosened, and no check was re-run until green (task 25's rule). A red here is
not a verdict on the code on a normal host, and a green on a normal host would not undo F1-F3.

**Ports.** The plan names 8730 (API), 1420 (Vite) and 9222 (CDP). On the orchestrator's rule for this
shared machine this document's run used 8813 (API), 8814 (Vite, `--port 8814 --strictPort`,
`VITE_PANACEA_APP_API=http://127.0.0.1:8813`) and 8815 (WebView2 DevTools), with
`PANACEA_APP_ALLOWED_ORIGINS=http://localhost:8814`; the native window was started with
`tauri dev --config` pointing `devUrl` at 8814. So the harness origin was an allowed origin, not
byte-for-byte the dev window's `http://localhost:1420`.

---

## What was tested

| Target | How | Result |
|---|---|---|
| The sandbox image alone | `docker run`, `curl` (steps 2–4) | 6/6 PASS (the table below) |
| The cockpit at a dev-window origin | headed Chromium, `desktop/e2e/phase2.e2e.mjs` | 0/19 PASS: 1 FAIL (`setup`), 18 NOT RUN (*The checks*) |
| The native window | WebView2 over the DevTools protocol (step 15) | Port allowed (`Edg/153.0.4234.48`); `w.webview2` NOT RUN |
| The owner's eyes | 19 numbered items (steps 16–18), batched to the owner's end-of-build session (OD-12) | 0 answered, 19 PENDING OWNER, unverified |

## Prerequisites — the two this phase recorded, and the three fixes

| Prerequisite | Evidence | Result |
|---|---|---|
| Image newer than `382f056` (2026-09-21T22:43:47Z), and tied to a commit | before `5d9a580e035b` labelled `a38c268`; after `2026-09-23T06:52:05.425529555Z` (every layer a cache hit) labelled `dbaf2ad6aed8…`; last image-tree commit `b4be5703`, an ancestor of the label | PASS |
| The Resume fix and the git + CA fix landed | step 1 (h): 1 / 1 / 2 / 2 / 1 / 4 / 3, every count as expected | PASS |
| TLS interception, and the CA bundle | step 1 (i): `ERROR SELF_SIGNED_CERT_IN_CHAIN`, exit 1; with the proxy's root (`CN=caadmin.netskope.com`, SHA-1 `05:20:B9:…:17:27`, exported by `openssl s_client -showcerts`) `HTTP 404`, exit 0 | PASS |
| Fail-closed startup | tokenless container exit 1: `sandboxd: no attach token configured; set AttachToken (PANACEA_APP_ATTACH_TOKEN / -attach-token) or pass -allow-unauthenticated to run without one` | PASS |
| Token rejection | `/healthz` 200, `/attach` 401 / 401 / 101; published `127.0.0.1:61233` only | PASS |
| Harness flags vs 2.1.274 | `--permission-mode` choices exactly acceptEdits, auto, bypassPermissions, manual, dontAsk, plan; both evidence comments re-cited to the image (`502f0751`) | PASS |

## Phase 1, re-proven

| Phase 1 row (`2026-09-17-phase-1-results.md:25-36`) | Re-proven where | Result |
|---|---|---|
| Container starts, `/healthz` | image alone, step 3: `200` | PASS |
| `claude --version` as uid 1001 | image alone, step 4: `1001`, `2.1.274 (Claude Code)` | PASS |
| WebSocket upgrade on `/attach` | image alone, step 3: `101` with the bearer | PASS |
| Resize observed by the process | through the relay, `c2.cols` (`stty size`) | NOT RUN |
| ANSI colour passes through | through the relay, `p1.fidelity` | NOT RUN |
| Box drawing (UTF-8) intact | through the relay, `p1.fidelity` | NOT RUN |
| Cursor control sequences intact | through the relay, `p1.fidelity` | NOT RUN |
| Exit status propagates | through the relay, `p1.fidelity` | NOT RUN |
| 500-line burst not truncated | through the relay, `p1.fidelity` | NOT RUN |
| Claude Code runs through the PTY | through the relay, `p1.claude` | NOT RUN |
| Claude Code help renders | through the relay, `p1.claude` | NOT RUN |
| Ctrl-C interrupts, shell survives | through the relay, `p1.fidelity` | NOT RUN |

Phase 1's results headline reads 9/9 over a table of twelve checks; the three container-level rows
appear to have been counted outside its harness. Recorded here rather than corrected there.

Task 5's Q3 and Q4 (step 4): `docker port` printed `8080/tcp -> 127.0.0.1:61418`, and no
`0.0.0.0` line; `State` read `running`, then `exited` after `docker stop`.

## The checks

This document's run: the committed harness at `83660ac2`, run once in full with
`PANACEA_E2E_CDP` set, from a fresh fixture and state directory. `e2e exit=1`, ended 10:30:45
local. It printed one line; `setup` is `always`, and a failed `setup` stops the run by design
(`desktop/e2e/phase2.e2e.mjs:163`), so the other eighteen did not run.

| Check | Closes | Result | Observed |
|---|---|---|---|
| `setup` | task 24 step 11, re-run: the catalogue loads and a session can be created from the UI | FAIL | 22,024 ms: `GET /v1/projects/p-b9ffb3be/branches -> 500 {"error":"internal","message":"Something went wrong inside the control plane. The detail is in its log."}`; the log: `git for-each-ref --format=%(refname:short) refs/heads in …\e2e-demo: exit status 1: ` (F2) |
| `p1.fidelity` | phase-1 rows: ANSI, box drawing, cursor control, exit status, 500-line burst, Ctrl-C — through the relay | NOT RUN | — |
| `p1.claude` | phase-1 rows: Claude Code runs through the PTY, and its help renders | NOT RUN | — |
| `c2.cols` | real-browser carry 2: mount after append yields a non-zero column count | NOT RUN | — |
| `c3.theme` | real-browser carry 3: themeFromTokens() produces real colours | NOT RUN | — |
| `c7.popover` | real-browser carry 7: Popover's viewport clamp, including the flip-above branch | NOT RUN | — |
| `c9.exit-row` | real-browser carry 9: the exit row is zero-height until exit, and the pane does not move | NOT RUN | — |
| `c1.flip` | real-browser carry 1: scrollback survives Columns <-> Canvas for a session both render | NOT RUN | — |
| `c1.cap` | real-browser carry 1, the boundary: a session dropped by the Canvas cap is re-attached, not preserved | NOT RUN | — |
| `c5.resize` | real-browser carry 5: a tile resize reflows the terminal, and the PTY is told | NOT RUN | — |
| `c4.pinned` | real-browser carry 4: the pinned-scroll rule holds against real xterm's asynchronous parse | NOT RUN | — |
| `c6.load` | real-browser carry 6: 4,000 lines/s for 10 s — usability asserted, render rate measured | NOT RUN | — |
| `d.detach` | durability: a pane unmounted past the grace re-attaches, and the replay carries the transcript | NOT RUN | — |
| `d.reload` | durability: the cockpit restarts and every live session re-attaches; known defect 6 in a real window | NOT RUN | — |
| `d.cp-restart` | durability: the control plane restarts; sessions and containers survive; the cockpit re-attaches unaided | NOT RUN | — |
| `c10.retry` | real-browser carry 10 + carry 9's overlay: Retry attach after a genuine failure; the strip lets the pointer through | NOT RUN | — |
| `o2.resume` | open carry 2: Resume forks the same Claude conversation, from its persisted transcript, in a new sandbox | NOT RUN | — |
| `g.primary` | sandbox git + CA fix: git works in a session on a primary worktree, as the configured identity | NOT RUN | — |
| `w.webview2` | phase-1 unproven item 5, agent half: the native WebView2 window mounts a live terminal | NOT RUN | — |

**Why no second full run.** Declared in advance: at most one more, started only after two load
samples in a row under 4 s for five git spawns (the level measured at 09:28). No sample between
10:32 and 11:09 came under 7.5 s, so none was started.

## Owner-observed

1. **The window is the cockpit**: PENDING OWNER
2. **Every pane shows a live terminal, not a blank box**: PENDING OWNER
3. **The terminal is legible**: PENDING OWNER
4. **Colour and box drawing survived the pipe**: PENDING OWNER
5. **A burst is acceptable**: PENDING OWNER
6. **The window resizes cleanly**: PENDING OWNER
7. **Flipping layout keeps what is on screen**: PENDING OWNER
8. **The focus ring says where the keyboard is**: PENDING OWNER
9. **The pane footer is readable**: PENDING OWNER
10. **The pane width uses the window well**: PENDING OWNER
11. **The reconnect strip does not hide what you need**: PENDING OWNER
12. **An exited session is calm and costs the layout nothing**: PENDING OWNER
13. **One heartbeat, no spinners**: PENDING OWNER
14. **A keyboard focus is visible on Canvas**: PENDING OWNER
15. **Start a Claude Code session**: PENDING OWNER
16. **Give it something to remember**: PENDING OWNER
17. **Kill it**: PENDING OWNER
18. **Resume it**: PENDING OWNER
19. **Ask**: PENDING OWNER

Each item's *do* and *you should see* are in `docs/superpowers/plans/phase-2-parts/task-25.md`
steps 16-18. The session's preparation — bringing the stack up, the state each step needs, and
which findings below bear on which item — is in the task's owner checklist in the SDD workspace
(`task-25-owner-checklist.md`, git-ignored).

## Findings

Each was measured in the day's control-plane logs and containers — F2 in this document's run,
F1 and F3 in the earlier runs listed below. None is fixed here (task 25 reports; the task that
owns the file fixes).

| # | What failed | Evidence | Owner |
|---|---|---|---|
| F1 | **A create the cockpit gives up on is cancelled half-made, and leaks a container.** The cockpit aborts `POST /v1/sessions` at `DEFAULT_TIMEOUT_MS` = 10 s (`desktop/src/api/client.ts:33`); under this load a create took about 15 s. The abort cancels the request context, which the create's Docker calls run on. | `dockerbe: start panacea-app-s-12993e6c: … context canceled` (08:09:58); `dockerbe: create panacea-app-s-e19633bd: … context canceled` (08:58:34); the cockpit's own text *"The control plane did not answer POST …/v1/sessions within 10000 ms."*; the record `exited` / *"the sandbox could not be created"* yet still naming `sandboxId s-e19633bd`, and that container `Created` more than an hour later, through a restart's startup sweep: the reaper counts an exited record as tracked (`reaper.go:372-377`) and rules 2 and 3 skip exited records (`:435`, `:459`). | `control-plane/internal/api/sessions.go` (a create must not run on the caller's context) with the reaper (task 13); the 10 s budget is `client.ts`'s (task 20) |
| F2 | **A timed-out host-git call reads as `exit status 1:` with nothing after it.** `project.GitTimeout` = 10 s (`control-plane/internal/project/gitrunner.go:17`); on Windows the kill is `TerminateProcess(…, 1)`, so the timeout reports as a plain exit 1 with empty output and the cause is never named. | 10 such lines in the day's two control-plane logs (9 in the first stack's, 1 in this run's), plus one `exec: canceling Cmd: TerminateProcess: Access is denied.`; e.g. `git config --get --system --includes --default native core.eol …: exit status 1: ` and `git for-each-ref --format=%(refname:short) refs/heads …: exit status 1: `; the run's `setup` failed on the second (`GET …/branches -> 500`); load sampled beside it: five git spawns in 40.9 s. | `gitrunner.go` (the error must say it timed out, and after how long); the 10 s bound itself is worth re-deciding with it |
| F3 | **sandboxd outlives the process it runs, so an exited session never becomes `exited`.** | A shell that ran `exit`: 26 min later `ps -ef` in its container shows only `sandboxd` (PID 1), the container is `Up`, `GET /v1/sessions` says `running`. Every attach gets an Exit frame and close *"session ended"* (`sandboxd/internal/server/server.go:151-154`) and the cockpit reconnects, because its ladder stops only on `status === "exited"` (`desktop/src/api/attachRegistry.ts:503`): 312 `attach relay ended … session ended` lines in 7 min for that one session. `sandboxd/cmd/sandboxd/main.go:66` serves until killed; the reaper's rule 3 (Go contract §5.3, with §2.2's `StateStopped` = "exists, process ended") assumes the container stops when the process does. | sandboxd (task 5), against the reaper's rule 3 (task 13) |

**The run's one FAIL is F2.** `setup`'s error is F2's second log line verbatim; its owner is
`gitrunner.go`'s. **Carried as an open observation, not a finding:** in run 1 and the diagnostic
variant, typed input drew no output in five and four checks respectively, after `c1.cap` had put
ten or more sessions on the branch; typing into a shell worked in isolation (ten tiles, both
layouts). Unexplained; a quiet host is the first thing to rule out.

## Earlier runs the same day — on record, not this document's result

Listed so no red is hidden and no green is borrowed. None of these lines is claimed above.

- **Run 1**, harness `953d14db` (before its four premise fixes), a state directory polluted by
  probes. Exit 1. PASS: `setup` (a false pass, false result 1), `p1.fidelity`, `c2.cols` (PTY
  53×71, one resize frame), `c3.theme`, `c1.cap`, `d.reload` (11 re-attached), `d.cp-restart`
  (11 sessions, containers unchanged), `g.primary` (status 0, commit 0, identity `Panacea E2E
  <e2e@invalid>`, CA mounted); `w.webview2` skipped. FAIL: `p1.claude`, `c7.popover`,
  `c9.exit-row`, `c1.flip`, `c5.resize`, `c4.pinned`, `c6.load`, `d.detach`, `c10.retry`,
  `o2.resume` — three on harness premises since fixed (`c7`, `c9`, `c1.flip`), two downstream of
  F1 (`p1.claude`, `o2.resume`), and five on typed input that drew no output (`c5`, `c4`, `c6`,
  `d.detach`, `c10`), not reproduced in isolation.
- **Run 2**, harness `83660ac2`, fresh state. Exit 1: `setup` FAIL, *"timed out after 10000 ms
  waiting for the UI-created session to reach the control plane"* — F1.
- **A diagnostic variant** (a scratch copy of the harness that substitutes a REST create when the
  UI create fails; not the committed harness). Exit 1. The UI create itself failed (F1), so its
  `setup` is no proof. PASS: `p1.fidelity`, `p1.claude` (`Welcome to Claude Code v2.1.274`),
  `c2.cols`, `c3.theme`, `d.reload`, `d.cp-restart`. FAIL, eleven: `c9.exit-row` and `c1.flip`
  (F3), `c1.cap` and `g.primary` (F2), `c4`, `c6`, `d.detach`, `c10` (typed input that drew no
  output), `c5.resize` (the tile did not grow), `c7.popover` (no context menu within 10 s), and
  `o2.resume` (no `Resume` menu item within 30 s).

## Open questions for the owner

1. **Should the product offer the `max` effort level?** `claude --help` in the pinned image
   (2.1.274) lists five levels — low, medium, high, xhigh, max. The product offers four, by a
   written decision: docs/FUNCTIONAL-SPEC.md:27, which control-plane/internal/harness/harness.go:37-40
   cites, and two tests pin. Saying yes amends FUNCTIONAL-SPEC.md, and the change is not Go-only:
   SessionPicker.tsx's EFFORT_LABEL is a Record<Effort, string>, so a fifth level added to the
   catalogue alone renders a picker row labelled `undefined`.

The run raised no further question for the owner: findings F1 to F3 above are engineering defects
with a root-cause fix each, not product choices.

## False results caught

1. **A setup that passed on a dead session** (run 1). The UI-created record appeared, so `setup`
   passed, and its sandbox then failed to start (F1). `p1.claude` and `o2.resume` failed for a
   reason `setup` had already seen. The harness now requires the session to leave `starting`
   alive (`83660ac2`).
2. **"2985 FLIP lines not rendered"** (run 1, `c1.flip`). One wheel event moves xterm five rows
   whatever its delta (measured: rows W349→W344→W339 for −1,000,000 and −300 alike), so the one
   "wheel to the top" read only the visible rows. The DOM-node, socket and relay-end observers
   were all clean. Harness premise, fixed in `83660ac2`.
3. **"third track … while running"** (run 1, `c9.exit-row`). The plan read the pane's third grid
   track as the exit bar's, `0px` while running. `.pane-exit` is rendered only on exit, and on a linked worktree
   the third child is the git note (tracks `38px 809.76px 48px 25.3958px`). Harness premise,
   fixed in `83660ac2` without changing what carry 9 asks.
4. **"the flip branch did not run"** (run 1, `c7.popover`). `.rail-machine` is the rail's first
   row (top 0), so a menu anchored to it can never flip. Harness premise, fixed in `83660ac2`.
5. **"dropped 7 of 11"** (run 1, `c1.cap`). The state directory held earlier probes' exited
   records on the same branch, and Canvas pinned them first. Every later run used a fresh one.
6. **"opened 10 new attach socket(s)"** (the first holder's diagnostic run, `c1.flip`). This was
   F3's reconnect storm on an exited shell, not the flip.
7. **A PID that did not hold the port** (step 6). `/proc/$!/winpid` named `nohup.exe`, not the
   listener; every PID in this document is read from `netstat` on the port instead.

## Carry disposition

The table is task 25's (`task-25.md:1691-1718`), copied; only *Result* is filled. NOT RUN means
the check exists and this document's run stopped before it (see *The checks*).

| Carry | Source | Disposition in task 25 | Result |
|---|---|---|---|
| RB 1 — scrollback survives Columns ↔ Canvas | `group-d-real-browser-carries.md:29` | Step 10, `c1.flip` + `c1.cap`; step 16 item 7 | NOT RUN (`c1.flip`, `c1.cap`); item 7 PENDING OWNER |
| RB 2 — non-zero columns after mount | `:51` | Step 8, `c2.cols`; step 15, `w.webview2` | NOT RUN (`c2.cols`, `w.webview2`) |
| RB 3 — `themeFromTokens()` real colours | `:56` | Step 8, `c3.theme`; step 16 item 3 (legibility) | NOT RUN (`c3.theme`); item 3 PENDING OWNER |
| RB 4 — pinned scroll vs async parse | `:60` | Step 12, `c4.pinned` | NOT RUN (`c4.pinned`) |
| RB 5 — fit reflows; resize reaches the daemon | `:65` | Step 11, `c5.resize`; step 16 item 6 | NOT RUN (`c5.resize`); item 6 PENDING OWNER |
| RB 6 — 10 Hz under real load | `:82` | Step 12, `c6.load` (measured); step 16 item 5 (judged) | NOT RUN (`c6.load`); item 5 PENDING OWNER |
| RB 7 — `Popover` viewport clamp | `:86` | Step 9, `c7.popover` | NOT RUN (`c7.popover`) |
| RB 8 — no terminal until task 24 mounts | `:89` | **Closed by task 24** step 6; `setup` depends on it | Closed by task 24; not re-observed here (`setup` failed before any pane mounted) |
| RB 9 — Develop lens geometry, contrast, 11px footer, overlay | `:93` | Step 9 `c9.exit-row`, step 14 overlay hit-test; step 17 items 8–12 | NOT RUN (`c9.exit-row`, `c10.retry`'s overlay); items 8–12 PENDING OWNER; F3 bears on item 12 |
| RB 10 — Retry attach on a real socket | `:119` | Step 14, `c10.retry` | NOT RUN (`c10.retry`) |
| OC 1 — failed attach had no way forward | `group-d-open-carries.md:16` | Closed by `07119d1`; its observation is RB 10 | Closed by `07119d1`; its observation (RB 10) NOT RUN |
| OC 2 — `resume` has no end-to-end proof | `:43` | Step 14 `o2.resume` (identity, the fork, and the persisted transcript); step 18 (the agent remembers, after a fresh `/login`; PENDING OWNER per OD-12). Built on the Resume fix, `b4be570` and `2e3db79`: `--session-id <uuid>` at create, `--resume <U> --fork-session --session-id <V>`, `/home/dev/.claude/projects` persisted | `o2.resume` NOT RUN; step 18 PENDING OWNER |
| Sandbox git + CA fix — git and Claude Code in a sandbox on this network | `sandbox-git-fix-report.md` | Step 1 (h) and (i); step 14 `g.primary` (status, command-scope `safe.directory`, a commit as the configured identity, the env names in `docker inspect`) | Step 1 (h) and (i) PASS; `g.primary` NOT RUN |
| OC 3 — Canvas `⌘1-9` does not raise the tile | `:47` | **Owner: task 23.** Step 17 item 14 observes the result | PENDING OWNER (item 14) |
| OC 4 — palette exposes no session verbs | `:52` | Correction already made (`80342ee`); nothing to observe | Closed by `80342ee`; nothing observed here |
| OC 5 — `max` effort (corrected at source: a product decision, not an omission) | `:60` | **Not changed.** Recorded as open question 1 for the owner; step 5a's `--help` reading is its evidence | Not changed; open question 1; `--effort` lists `low, medium, high, xhigh, max` in 2.1.274 |
| OC 6 — harness comments cite 2.1.267 | `:73` | Step 5a, 5b | PASS (`502f0751`) |
| OC 7 — rebuild the sandbox image | `:73` | Step 2, proven by step 3 | PASS |
| OC 8 — `Files` and `Merge` have no owning phase | `:77` | **Named by task 26** (which ran before this task) as seam S3.5 in `docs/SEAMS.md`, with the gap pinned; the ownership decision is the product owner's | Named by task 26 (S3.5); nothing observed here |
| OC 9 — `pinned` unbounded across branches | `:85` | **Deferred** to whichever task next opens `prefsSlice`; the cheapest prune is where the poller replaces sessions | Deferred |
| OC 10 — an unpin does not survive a flip or restart | `:91` | **Deferred** to the next persisted-schema bump (`PERSIST_VERSION`), which can tell *never pinned* from *unpinned* | Deferred |
| OC 11 — `observe` drops `dirty` before the write | `:99` | **Deferred to Phase 3**, as recorded | Deferred to Phase 3 |
| OC 12 — `proxy.Relay` has no production caller | `:102` | **Ruled by task 26** (which ran before this task): deleted, after an anchored grep re-proved no production caller; `proxy.RelayHooked` is the one relay (`docs/SEAMS.md` S8.7) | Ruled by task 26; nothing observed here |
| Contract §12 open question 1 — `@xterm/xterm@6` under jsdom never observed | `2026-09-18-phase-2-ui-contract.md:2355-2358` | Answered as it prescribes: real xterm is observed in steps 8–15 by Playwright, not by `canvas` in jsdom | Not answered by this run: no real xterm mounted before `setup` failed |
| Phase 1 unproven item 5 — the native window | `2026-09-17-phase-1-results.md:68` | Step 15 (agent half), step 16 (owner half) | Agent half: the DevTools port was allowed (`Edg/153.0.4234.48`), `w.webview2` NOT RUN; owner half (step 16) PENDING OWNER |
| Task 5's Q3 and Q4 | `task-05.md:1457` | Step 4 | PASS |

## What is still unproven

1. Everything in 2026-09-17-phase-1-results.md:62-67 that needs an AKS cluster — Kata
   isolation, the RuntimeClass name, cold start, the read-only root in practice. Phase 8.
2. Contract §4.7's refresh half: declared as not delivered by task-24.md:28.
3. The CORS grant is task 24 step 11's evidence, relied on here and not re-run (step 6 proved
   only that the allowlisted origin gets 200 and another gets 403).
4. All nineteen owner items: batched to the owner's end-of-build session (OD-12), unverified
   until then. That includes whether a resumed agent remembers (step 18).
5. Org inference (OD-19) is the cloud path and is not built in this phase. Local mode (OD-20),
   which this run exercised, still needs `/login` in every sandbox.
6. **Everything the eighteen NOT RUN checks were to prove**, because this document's run stopped
   at `setup`: scrollback across a re-parent and the Canvas cap (RB 1), columns after mount (RB 2),
   real theme colours (RB 3), pinned scroll (RB 4), resize reaching the PTY (RB 5), the 10 Hz
   flush under load (RB 6), the popover clamp (RB 7), the exit row (RB 9), Retry attach (RB 10),
   detach, reload and control-plane restart, Resume end to end (OC 2), git on a primary worktree,
   and the native window mounting a live terminal. Earlier runs the same day observed some of
   these; they are listed above and not claimed.

## Reproducing this

In Git Bash, every command through the audited wrapper, after taking the Docker lock and running
its guard. `$E2E` is an empty directory under `$TEMP`; `$CA` is the proxy root as a PEM file
(step 1 (i)); the ports are this run's (the plan's are 8730, 1420, 9222).

1. The image, as task 25 step 2 writes it, then steps 3 and 4's probes.
2. The fixture and the control plane:
   `bash desktop/e2e/fixture.sh "$E2E"` and `(cd control-plane && go build -o "$E2E/bin/panacea-api.exe" ./cmd/api)`.
3. One environment for everything after this, exported in the shell that runs the harness:
   `PANACEA_APP_ADDR=127.0.0.1:8813`, `PANACEA_APP_ALLOWED_ORIGINS=http://localhost:8814`,
   `PANACEA_APP_PROJECTS_DIR=$E2E/projects`, `PANACEA_APP_STATE_DIR=$E2E/state`,
   `PANACEA_APP_CA_BUNDLE=$CA`, `PANACEA_APP_REAP_INTERVAL=24h`,
   `PANACEA_APP_GIT_NAME="Panacea E2E"`, `PANACEA_APP_GIT_EMAIL=e2e@invalid`,
   `PANACEA_APP_SESSION_CPU_MILLIS=250`, `PANACEA_APP_SESSION_MEMORY_MIB=512`,
   `PANACEA_APP_LOG_LEVEL=debug`, `VITE_PANACEA_APP_API=http://127.0.0.1:8813`,
   `PANACEA_E2E_APP=http://localhost:8814`, `PANACEA_E2E_API=http://127.0.0.1:8813`,
   `PANACEA_E2E_CP_LOG=$E2E/control-plane.log`, `PANACEA_E2E_CP_BIN=$E2E/bin/panacea-api.exe`,
   and `PANACEA_E2E_CP_PID` = the PID `netstat` shows on 8813 once the control plane is up
   (Windows paths via `cygpath -w`).
4. Start the control plane (`"$E2E/bin/panacea-api.exe" >> "$E2E/control-plane.log" 2>&1 &`) and,
   from `desktop/`, Vite (`node node_modules/vite/bin/vite.js --port 8814 --strictPort`).
5. The native window, from `desktop/`: `WEBVIEW2_ADDITIONAL_BROWSER_ARGUMENTS=--remote-debugging-port=8815
   npx tauri dev --config '{"build":{"devUrl":"http://localhost:8814","beforeDevCommand":null}}'`.
6. The run, from `desktop/`: `PANACEA_E2E_CDP=http://127.0.0.1:8815 npm run e2e > "$E2E/results.jsonl"; echo "exit=$?"`
   — redirected, not piped, so the exit code is the harness's own.
